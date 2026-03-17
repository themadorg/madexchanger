/*
Madexchanger — HTTP/HTTPS email relay proxy for Madmail.
Copyright © 2024 The Mad Org contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package adminapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/themadorg/madexchanger/internal/config"
	"github.com/themadorg/madexchanger/internal/db"
	"github.com/themadorg/madexchanger/internal/logger"
)

const testToken = "test-secret-token"

func setupHandler(t *testing.T) *Handler {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cfg := &config.Config{
		RelayMode:  "all",
		DownstreamURL: "https://10.0.0.5",
		ReceivePath:   "/mxdeliv",
		SkipTLSVerify: true,
		MaxBodySize:   33554432,
		AdminWeb: config.AdminWebConfig{
			Token: testToken,
		},
	}

	log := logger.New("error")
	return New(cfg, store, log)
}

// rpc builds a JSON RPC request body and POSTs it to the handler.
func rpc(h *Handler, method, resource string, body interface{}) *httptest.ResponseRecorder {
	rpcBody := map[string]interface{}{
		"method":   method,
		"resource": resource,
		"headers":  map[string]string{"Authorization": "Bearer " + testToken},
		"body":     body,
	}
	data, _ := json.Marshal(rpcBody)
	req := httptest.NewRequest(http.MethodPost, "/api/admin", strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// rpcNoAuth builds an RPC request without auth.
func rpcNoAuth(h *Handler, method, resource string) *httptest.ResponseRecorder {
	rpcBody := map[string]interface{}{
		"method":   method,
		"resource": resource,
		"headers":  map[string]string{},
		"body":     map[string]interface{}{},
	}
	data, _ := json.Marshal(rpcBody)
	req := httptest.NewRequest(http.MethodPost, "/api/admin", strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func parseResponse(t *testing.T, w *httptest.ResponseRecorder) rpcResponse {
	t.Helper()
	var resp rpcResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse RPC response: %v, body: %s", err, w.Body.String())
	}
	return resp
}

func TestUnauthorized(t *testing.T) {
	h := setupHandler(t)
	w := rpcNoAuth(h, "GET", "/admin/stats")
	resp := parseResponse(t, w)

	if resp.Status != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.Status)
	}
	if resp.Error == nil || *resp.Error != "unauthorized" {
		t.Errorf("error = %v, want 'unauthorized'", resp.Error)
	}
}

func TestCORSPreflight(t *testing.T) {
	h := setupHandler(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/admin", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("OPTIONS status = %d, want 200", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS header")
	}
}

func TestGetStats(t *testing.T) {
	h := setupHandler(t)
	w := rpc(h, "GET", "/admin/stats", nil)
	resp := parseResponse(t, w)

	if resp.Status != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Status)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", *resp.Error)
	}
}

func TestGetRoutes(t *testing.T) {
	h := setupHandler(t)
	w := rpc(h, "GET", "/admin/routes", nil)
	resp := parseResponse(t, w)

	if resp.Status != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Status)
	}
}

func TestGetConfig(t *testing.T) {
	h := setupHandler(t)
	w := rpc(h, "GET", "/admin/config", nil)
	resp := parseResponse(t, w)

	if resp.Status != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Status)
	}
	body := resp.Body.(map[string]interface{})
	if body["relay_mode"] != "all" {
		t.Errorf("incoming_mode = %v, want all", body["relay_mode"])
	}
}

func TestUpdateRelayMode(t *testing.T) {
	h := setupHandler(t)

	w := rpc(h, "POST", "/admin/config", map[string]string{"relay_mode": "selected"})
	resp := parseResponse(t, w)
	if resp.Status != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Status)
	}

	// Verify change.
	w = rpc(h, "GET", "/admin/config", nil)
	resp = parseResponse(t, w)
	body := resp.Body.(map[string]interface{})
	if body["relay_mode"] != "selected" {
		t.Errorf("incoming_mode = %v, want selected", body["relay_mode"])
	}
}

func TestUpdateRelayModeInvalid(t *testing.T) {
	h := setupHandler(t)
	w := rpc(h, "POST", "/admin/config", map[string]string{"relay_mode": "invalid"})
	resp := parseResponse(t, w)
	if resp.Status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.Status)
	}
}

func TestRewriteRulesEndToEnd(t *testing.T) {
	h := setupHandler(t)

	// Create.
	w := rpc(h, "POST", "/admin/rewrites", map[string]interface{}{
		"enabled": true, "field": "mail_from", "pattern": "old@x.org", "replacement": "new@x.org", "comment": "test",
	})
	resp := parseResponse(t, w)
	if resp.Status != http.StatusOK {
		t.Fatalf("create status = %d, body: %s", resp.Status, w.Body.String())
	}
	body := resp.Body.(map[string]interface{})
	ruleID := body["id"].(float64)
	if ruleID == 0 {
		t.Fatal("created rule should have an ID")
	}

	// List.
	w = rpc(h, "GET", "/admin/rewrites", nil)
	resp = parseResponse(t, w)
	if resp.Status != http.StatusOK {
		t.Fatalf("list status = %d", resp.Status)
	}
	rules := resp.Body.([]interface{})
	if len(rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(rules))
	}

	// Update.
	w = rpc(h, "PUT", "/admin/rewrites", map[string]interface{}{
		"id": ruleID, "enabled": false, "field": "mail_from", "pattern": "updated@x.org", "replacement": "n2@x.org", "comment": "upd",
	})
	resp = parseResponse(t, w)
	if resp.Status != http.StatusOK {
		t.Fatalf("update status = %d", resp.Status)
	}

	// Delete.
	w = rpc(h, "DELETE", "/admin/rewrites", map[string]interface{}{"id": ruleID})
	resp = parseResponse(t, w)
	if resp.Status != http.StatusOK {
		t.Fatalf("delete status = %d", resp.Status)
	}

	// Verify empty.
	w = rpc(h, "GET", "/admin/rewrites", nil)
	resp = parseResponse(t, w)
	rules = resp.Body.([]interface{})
	if len(rules) != 0 {
		t.Errorf("len(rules) = %d after delete, want 0", len(rules))
	}
}

func TestRelayFiltersEndToEnd(t *testing.T) {
	h := setupHandler(t)

	// Create.
	w := rpc(h, "POST", "/admin/filters", map[string]interface{}{
		"enabled": true, "field": "domain", "pattern": "example.org", "comment": "allow",
	})
	resp := parseResponse(t, w)
	if resp.Status != http.StatusOK {
		t.Fatalf("create status = %d", resp.Status)
	}
	body := resp.Body.(map[string]interface{})
	filterID := body["id"].(float64)

	// List.
	w = rpc(h, "GET", "/admin/filters", nil)
	resp = parseResponse(t, w)
	filters := resp.Body.([]interface{})
	if len(filters) != 1 {
		t.Fatalf("len(filters) = %d, want 1", len(filters))
	}

	// Update.
	w = rpc(h, "PUT", "/admin/filters", map[string]interface{}{
		"id": filterID, "enabled": false, "field": "domain", "pattern": "*.example.org", "comment": "updated",
	})
	resp = parseResponse(t, w)
	if resp.Status != http.StatusOK {
		t.Fatalf("update status = %d", resp.Status)
	}

	// Delete.
	w = rpc(h, "DELETE", "/admin/filters", map[string]interface{}{"id": filterID})
	resp = parseResponse(t, w)
	if resp.Status != http.StatusOK {
		t.Fatalf("delete status = %d", resp.Status)
	}
}

func TestUnknownResource(t *testing.T) {
	h := setupHandler(t)
	w := rpc(h, "GET", "/admin/nonexistent", nil)
	resp := parseResponse(t, w)
	if resp.Status != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.Status)
	}
}

func TestGetMethod(t *testing.T) {
	h := setupHandler(t)
	// Direct GET (not POST) should be rejected.
	req := httptest.NewRequest(http.MethodGet, "/api/admin", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	resp := parseResponse(t, w)
	if resp.Status != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.Status)
	}
}
