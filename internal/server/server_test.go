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

package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/themadorg/madexchanger/internal/config"
	"github.com/themadorg/madexchanger/internal/db"
	"github.com/themadorg/madexchanger/internal/forwarder"
	"github.com/themadorg/madexchanger/internal/logger"
)

// newTestServer creates a Server with a mock downstream and returns
// the Server plus the downstream test server (for cleanup).
func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()

	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	cfg := &config.Config{
		Listen:         ":0",
		ReceivePath:    "/mxdeliv",
		DownstreamURL:  downstream.URL,
		ForwardTimeout: 5,
		SkipTLSVerify:  true,
		MaxBodySize:    1024 * 1024, // 1 MiB for tests.
		LogLevel:       "error",
		IncomingMode:   "all",
	}

	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	log := logger.New(cfg.LogLevel)
	fwd := forwarder.New(cfg.DownstreamURL, cfg.ForwardTimeout, cfg.SkipTLSVerify, "", log)
	srv := New(cfg, fwd, store, log)

	return srv, downstream
}

func TestHandleReceiveSuccess(t *testing.T) {
	srv, downstream := newTestServer(t)
	defer downstream.Close()

	body := "From: alice@example.org\r\nTo: bob@example.org\r\n\r\nHello!"
	req := httptest.NewRequest(http.MethodPost, "/mxdeliv", strings.NewReader(body))
	req.Header.Set("X-Mail-From", "alice@example.org")
	req.Header.Add("X-Mail-To", "bob@example.org")

	w := httptest.NewRecorder()
	srv.handleReceive(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body)
	}

	if srv.received.Load() != 1 {
		t.Errorf("received counter = %d, want 1", srv.received.Load())
	}
	if srv.forwarded.Load() != 1 {
		t.Errorf("forwarded counter = %d, want 1", srv.forwarded.Load())
	}

	// Verify stats.
	stats, err := srv.store.GetStats()
	if err != nil {
		t.Fatalf("GetStats error: %v", err)
	}
	if stats.TotalRelayed != 1 {
		t.Errorf("TotalRelayed = %d, want 1", stats.TotalRelayed)
	}

	routes, err := srv.store.ListRoutes()
	if err != nil {
		t.Fatalf("ListRoutes error: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
}

func TestHandleReceiveMethodNotAllowed(t *testing.T) {
	srv, downstream := newTestServer(t)
	defer downstream.Close()

	req := httptest.NewRequest(http.MethodGet, "/mxdeliv", nil)
	w := httptest.NewRecorder()
	srv.handleReceive(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleReceiveMissingRecipient(t *testing.T) {
	srv, downstream := newTestServer(t)
	defer downstream.Close()

	req := httptest.NewRequest(http.MethodPost, "/mxdeliv", strings.NewReader("body"))
	req.Header.Set("X-Mail-From", "alice@example.org")
	// No X-Mail-To header.

	w := httptest.NewRecorder()
	srv.handleReceive(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleReceiveEmptyBody(t *testing.T) {
	srv, downstream := newTestServer(t)
	defer downstream.Close()

	req := httptest.NewRequest(http.MethodPost, "/mxdeliv", strings.NewReader(""))
	req.Header.Set("X-Mail-From", "alice@example.org")
	req.Header.Add("X-Mail-To", "bob@example.org")

	w := httptest.NewRecorder()
	srv.handleReceive(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleReceiveBodyTooLarge(t *testing.T) {
	srv, downstream := newTestServer(t)
	defer downstream.Close()

	// Create a body larger than the 1 MiB test limit.
	bigBody := strings.Repeat("X", 1024*1024+100)
	req := httptest.NewRequest(http.MethodPost, "/mxdeliv", strings.NewReader(bigBody))
	req.Header.Set("X-Mail-From", "alice@example.org")
	req.Header.Add("X-Mail-To", "bob@example.org")

	w := httptest.NewRecorder()
	srv.handleReceive(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", w.Code)
	}
}

func TestHandleHealth(t *testing.T) {
	srv, downstream := newTestServer(t)
	defer downstream.Close()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	srv.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, `"status":"ok"`) {
		t.Errorf("health body = %q, want JSON with status:ok", body)
	}
	if !strings.Contains(body, `"incoming_mode":"all"`) {
		t.Errorf("health body should contain incoming_mode")
	}
}

func TestHandleReceiveDownstreamFailure(t *testing.T) {
	// Use a downstream that returns 500.
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer downstream.Close()

	cfg := &config.Config{
		Listen:         ":0",
		ReceivePath:    "/mxdeliv",
		DownstreamURL:  downstream.URL,
		ForwardTimeout: 5,
		SkipTLSVerify:  true,
		MaxBodySize:    1024 * 1024,
		LogLevel:       "error",
		IncomingMode:   "all",
	}

	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	log := logger.New(cfg.LogLevel)
	fwd := forwarder.New(cfg.DownstreamURL, cfg.ForwardTimeout, cfg.SkipTLSVerify, "", log)
	srv := New(cfg, fwd, store, log)

	body := "From: alice@example.org\r\nTo: bob@example.org\r\n\r\nHello!"
	req := httptest.NewRequest(http.MethodPost, "/mxdeliv", strings.NewReader(body))
	req.Header.Set("X-Mail-From", "alice@example.org")
	req.Header.Add("X-Mail-To", "bob@example.org")

	w := httptest.NewRecorder()
	srv.handleReceive(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}

	if srv.errors.Load() != 1 {
		t.Errorf("errors counter = %d, want 1", srv.errors.Load())
	}

	// Verify error was recorded.
	stats, _ := store.GetStats()
	if stats.TotalErrors != 1 {
		t.Errorf("TotalErrors = %d, want 1", stats.TotalErrors)
	}
}

func TestRelaySelectedModeReject(t *testing.T) {
	srv, downstream := newTestServer(t)
	defer downstream.Close()

	// Switch to "reject" mode with no filters = reject all.
	srv.cfg.IncomingMode = "reject"

	body := "From: alice@example.org\r\nTo: bob@example.org\r\n\r\nHello!"
	req := httptest.NewRequest(http.MethodPost, "/mxdeliv", strings.NewReader(body))
	req.Header.Set("X-Mail-From", "alice@example.org")
	req.Header.Add("X-Mail-To", "bob@example.org")

	w := httptest.NewRecorder()
	srv.handleReceive(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (rejected by filter)", w.Code)
	}
}

func TestRelaySelectedModeAllow(t *testing.T) {
	srv, downstream := newTestServer(t)
	defer downstream.Close()

	// Switch to "reject" mode and add a matching filter.
	srv.cfg.IncomingMode = "reject"
	_ = srv.store.AddIncomingRule(&db.AllowRule{
		Enabled: true,
		Field:   "domain",
		Pattern: "example.org",
	})

	body := "From: alice@example.org\r\nTo: bob@example.org\r\n\r\nHello!"
	req := httptest.NewRequest(http.MethodPost, "/mxdeliv", strings.NewReader(body))
	req.Header.Set("X-Mail-From", "alice@example.org")
	req.Header.Add("X-Mail-To", "bob@example.org")

	w := httptest.NewRecorder()
	srv.handleReceive(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (should pass filter)", w.Code)
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern, value string
		want           bool
	}{
		{"example.org", "example.org", true},
		{"example.org", "other.org", false},
		{"*example.org", "sub.example.org", true},
		{"*@example.org", "alice@example.org", true},
		{"alice*", "alice@example.org", true},
		{"alice*", "bob@example.org", false},
	}
	for _, tt := range tests {
		got := matchPattern(tt.pattern, tt.value)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
		}
	}
}
