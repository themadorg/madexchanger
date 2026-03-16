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

// Package adminapi provides the RPC-style single-endpoint Admin API for
// the madexchanger relay proxy. Following the same pattern as Madmail's
// Admin API, all requests are `POST /api/admin` with a JSON body.
//
// # Request Format
//
//	{
//	    "method": "GET|POST|PUT|DELETE",
//	    "resource": "/admin/stats",
//	    "headers": { "Authorization": "Bearer <token>" },
//	    "body": {}
//	}
//
// # Response Format
//
//	{
//	    "status": 200,
//	    "resource": "/admin/stats",
//	    "body": { ... },
//	    "error": null
//	}
//
// # Available Resources
//
//	GET    /admin/stats      — Aggregate relay statistics
//	GET    /admin/routes     — Route counters (from_server → to_server)
//	GET    /admin/config     — Current relay configuration
//	POST   /admin/config     — Update relay mode
//	GET    /admin/rewrites   — List rewrite rules
//	POST   /admin/rewrites   — Add rewrite rule
//	PUT    /admin/rewrites   — Update rewrite rule (body must include "id")
//	DELETE /admin/rewrites   — Delete rewrite rule (body: {"id": N})
//	GET    /admin/filters    — List relay filters
//	POST   /admin/filters    — Add relay filter
//	PUT    /admin/filters    — Update relay filter (body must include "id")
//	DELETE /admin/filters    — Delete relay filter (body: {"id": N})
package adminapi

import (
	"encoding/json"
	"net/http"

	"github.com/themadorg/madexchanger/internal/config"
	"github.com/themadorg/madexchanger/internal/db"
	"github.com/themadorg/madexchanger/internal/logger"
)

// rpcRequest is the envelope for all Admin API requests.
type rpcRequest struct {
	Method   string            `json:"method"`
	Resource string            `json:"resource"`
	Headers  map[string]string `json:"headers"`
	Body     json.RawMessage   `json:"body"`
}

// rpcResponse is the envelope for all Admin API responses.
type rpcResponse struct {
	Status   int         `json:"status"`
	Resource string      `json:"resource"`
	Body     interface{} `json:"body"`
	Error    *string     `json:"error"`
}

// Handler manages admin API request routing and authentication.
type Handler struct {
	cfg   *config.Config
	store *db.DB
	log   *logger.Logger
}

// New creates an admin API handler.
func New(cfg *config.Config, store *db.DB, log *logger.Logger) *Handler {
	return &Handler{
		cfg:   cfg,
		store: store,
		log:   log,
	}
}

// ServeHTTP handles the single RPC endpoint. All requests are POST
// with a JSON body containing method, resource, headers, and body.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers for cross-origin dashboard access.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		h.writeRPC(w, "", http.StatusMethodNotAllowed, nil, "only POST is allowed")
		return
	}

	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeRPC(w, "", http.StatusBadRequest, nil, "invalid JSON request")
		return
	}

	// Authenticate via inner headers.
	if h.cfg.AdminWeb.Token != "" {
		auth := req.Headers["Authorization"]
		expected := "Bearer " + h.cfg.AdminWeb.Token
		if auth != expected {
			h.writeRPC(w, req.Resource, http.StatusUnauthorized, nil, "unauthorized")
			return
		}
	}

	// Route to the correct handler based on resource.
	h.dispatch(w, &req)
}

// dispatch routes the RPC request to the appropriate handler.
func (h *Handler) dispatch(w http.ResponseWriter, req *rpcRequest) {
	switch req.Resource {
	case "/admin/stats":
		h.handleStats(w, req)
	case "/admin/routes":
		h.handleRoutes(w, req)
	case "/admin/config":
		h.handleConfig(w, req)
	case "/admin/rewrites":
		h.handleRewrites(w, req)
	case "/admin/filters":
		h.handleFilters(w, req)
	case "/admin/proxies":
		h.handleProxies(w, req)
	case "/admin/proxy-routes":
		h.handleProxyRoutes(w, req)
	default:
		h.writeRPC(w, req.Resource, http.StatusNotFound, nil, "unknown resource: "+req.Resource)
	}
}

// --- Stats ---

func (h *Handler) handleStats(w http.ResponseWriter, req *rpcRequest) {
	if req.Method != "GET" {
		h.writeRPC(w, req.Resource, http.StatusMethodNotAllowed, nil, "use GET")
		return
	}

	stats, err := h.store.GetStats()
	if err != nil {
		h.log.Error("failed to get stats", "err", err)
		h.writeRPC(w, req.Resource, http.StatusInternalServerError, nil, "failed to get stats")
		return
	}
	h.writeRPC(w, req.Resource, http.StatusOK, stats, "")
}

// --- Routes ---

func (h *Handler) handleRoutes(w http.ResponseWriter, req *rpcRequest) {
	if req.Method != "GET" {
		h.writeRPC(w, req.Resource, http.StatusMethodNotAllowed, nil, "use GET")
		return
	}

	routes, err := h.store.ListRoutes()
	if err != nil {
		h.log.Error("failed to list routes", "err", err)
		h.writeRPC(w, req.Resource, http.StatusInternalServerError, nil, "failed to list routes")
		return
	}
	if routes == nil {
		routes = []db.Route{}
	}
	h.writeRPC(w, req.Resource, http.StatusOK, routes, "")
}

// --- Config ---

type configBody struct {
	RelayMode     string `json:"relay_mode"`
	RoutingMode   string `json:"routing_mode"` // "dynamic" or "static"
	DownstreamURL string `json:"downstream_url"`
	DeliveryPath  string `json:"delivery_path"` // always /mxdeliv
	ReceivePath   string `json:"receive_path"`
	SkipTLSVerify bool   `json:"skip_tls_verify"`
	MaxBodySize   int64  `json:"max_body_size"`
	ProxyURL      string `json:"proxy_url"`
}

type configUpdateBody struct {
	RelayMode string `json:"relay_mode"`
}

func (h *Handler) handleConfig(w http.ResponseWriter, req *rpcRequest) {
	switch req.Method {
	case "GET":
		routingMode := "dynamic"
		if h.cfg.DownstreamURL != "" {
			routingMode = "static"
		}
		resp := configBody{
			RelayMode:     h.cfg.RelayMode,
			RoutingMode:   routingMode,
			DownstreamURL: h.cfg.DownstreamURL,
			DeliveryPath:  config.DeliveryPath,
			ReceivePath:   h.cfg.ReceivePath,
			SkipTLSVerify: h.cfg.SkipTLSVerify,
			MaxBodySize:   h.cfg.MaxBodySize,
			ProxyURL:      h.cfg.Proxy.URL,
		}
		h.writeRPC(w, req.Resource, http.StatusOK, resp, "")

	case "POST":
		var body configUpdateBody
		if err := json.Unmarshal(req.Body, &body); err != nil {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "invalid JSON body")
			return
		}
		if body.RelayMode != "all" && body.RelayMode != "selected" {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, `relay_mode must be "all" or "selected"`)
			return
		}
		h.cfg.RelayMode = body.RelayMode
		h.log.Info("relay mode updated", "mode", body.RelayMode)
		h.writeRPC(w, req.Resource, http.StatusOK, map[string]string{"status": "ok", "relay_mode": body.RelayMode}, "")

	default:
		h.writeRPC(w, req.Resource, http.StatusMethodNotAllowed, nil, "use GET or POST")
	}
}

// --- Rewrite Rules ---

type rewriteDeleteBody struct {
	ID int64 `json:"id"`
}

func (h *Handler) handleRewrites(w http.ResponseWriter, req *rpcRequest) {
	switch req.Method {
	case "GET":
		rules, err := h.store.ListRewriteRules()
		if err != nil {
			h.log.Error("failed to list rewrite rules", "err", err)
			h.writeRPC(w, req.Resource, http.StatusInternalServerError, nil, "failed to list rewrite rules")
			return
		}
		if rules == nil {
			rules = []db.RewriteRule{}
		}
		h.writeRPC(w, req.Resource, http.StatusOK, rules, "")

	case "POST":
		var rule db.RewriteRule
		if err := json.Unmarshal(req.Body, &rule); err != nil {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "invalid JSON body")
			return
		}
		if err := h.store.AddRewriteRule(&rule); err != nil {
			h.log.Error("failed to add rewrite rule", "err", err)
			h.writeRPC(w, req.Resource, http.StatusInternalServerError, nil, "failed to add rewrite rule")
			return
		}
		h.log.Info("rewrite rule added", "id", rule.ID, "field", rule.Field, "pattern", rule.Pattern)
		h.writeRPC(w, req.Resource, http.StatusOK, rule, "")

	case "PUT":
		var rule db.RewriteRule
		if err := json.Unmarshal(req.Body, &rule); err != nil {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "invalid JSON body")
			return
		}
		if rule.ID == 0 {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "id is required for update")
			return
		}
		if err := h.store.UpdateRewriteRule(&rule); err != nil {
			h.log.Error("failed to update rewrite rule", "err", err)
			h.writeRPC(w, req.Resource, http.StatusInternalServerError, nil, "failed to update rewrite rule")
			return
		}
		h.writeRPC(w, req.Resource, http.StatusOK, rule, "")

	case "DELETE":
		var body rewriteDeleteBody
		if err := json.Unmarshal(req.Body, &body); err != nil {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "invalid JSON body")
			return
		}
		if body.ID == 0 {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "id is required")
			return
		}
		if err := h.store.DeleteRewriteRule(body.ID); err != nil {
			h.log.Error("failed to delete rewrite rule", "err", err)
			h.writeRPC(w, req.Resource, http.StatusInternalServerError, nil, "failed to delete rewrite rule")
			return
		}
		h.writeRPC(w, req.Resource, http.StatusOK, map[string]string{"status": "deleted"}, "")

	default:
		h.writeRPC(w, req.Resource, http.StatusMethodNotAllowed, nil, "use GET, POST, PUT, or DELETE")
	}
}

// --- Relay Filters ---

type filterDeleteBody struct {
	ID int64 `json:"id"`
}

func (h *Handler) handleFilters(w http.ResponseWriter, req *rpcRequest) {
	switch req.Method {
	case "GET":
		filters, err := h.store.ListRelayFilters()
		if err != nil {
			h.log.Error("failed to list relay filters", "err", err)
			h.writeRPC(w, req.Resource, http.StatusInternalServerError, nil, "failed to list relay filters")
			return
		}
		if filters == nil {
			filters = []db.RelayFilter{}
		}
		h.writeRPC(w, req.Resource, http.StatusOK, filters, "")

	case "POST":
		var filter db.RelayFilter
		if err := json.Unmarshal(req.Body, &filter); err != nil {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "invalid JSON body")
			return
		}
		if err := h.store.AddRelayFilter(&filter); err != nil {
			h.log.Error("failed to add relay filter", "err", err)
			h.writeRPC(w, req.Resource, http.StatusInternalServerError, nil, "failed to add relay filter")
			return
		}
		h.log.Info("relay filter added", "id", filter.ID, "field", filter.Field, "pattern", filter.Pattern)
		h.writeRPC(w, req.Resource, http.StatusOK, filter, "")

	case "PUT":
		var filter db.RelayFilter
		if err := json.Unmarshal(req.Body, &filter); err != nil {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "invalid JSON body")
			return
		}
		if filter.ID == 0 {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "id is required for update")
			return
		}
		if err := h.store.UpdateRelayFilter(&filter); err != nil {
			h.log.Error("failed to update relay filter", "err", err)
			h.writeRPC(w, req.Resource, http.StatusInternalServerError, nil, "failed to update relay filter")
			return
		}
		h.writeRPC(w, req.Resource, http.StatusOK, filter, "")

	case "DELETE":
		var body filterDeleteBody
		if err := json.Unmarshal(req.Body, &body); err != nil {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "invalid JSON body")
			return
		}
		if body.ID == 0 {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "id is required")
			return
		}
		if err := h.store.DeleteRelayFilter(body.ID); err != nil {
			h.log.Error("failed to delete relay filter", "err", err)
			h.writeRPC(w, req.Resource, http.StatusInternalServerError, nil, "failed to delete relay filter")
			return
		}
		h.writeRPC(w, req.Resource, http.StatusOK, map[string]string{"status": "deleted"}, "")

	default:
		h.writeRPC(w, req.Resource, http.StatusMethodNotAllowed, nil, "use GET, POST, PUT, or DELETE")
	}
}

// writeRPC marshals an RPC response envelope and writes it to w.
func (h *Handler) writeRPC(w http.ResponseWriter, resource string, status int, body interface{}, errMsg string) {
	resp := rpcResponse{
		Status:   status,
		Resource: resource,
		Body:     body,
	}
	if errMsg != "" {
		resp.Error = &errMsg
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.log.Error("failed to encode RPC response", "err", err)
	}
}

// --- Proxies ---

type proxyDeleteBody struct {
	ID int64 `json:"id"`
}

func (h *Handler) handleProxies(w http.ResponseWriter, req *rpcRequest) {
	switch req.Method {
	case "GET":
		proxies, err := h.store.ListProxies()
		if err != nil {
			h.writeRPC(w, req.Resource, http.StatusInternalServerError, nil, err.Error())
			return
		}
		if proxies == nil {
			proxies = []db.Proxy{}
		}
		h.writeRPC(w, req.Resource, http.StatusOK, proxies, "")

	case "POST":
		var p db.Proxy
		if err := json.Unmarshal(req.Body, &p); err != nil {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "invalid body: "+err.Error())
			return
		}
		if p.Host == "" {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "host is required")
			return
		}
		if p.Type == "" {
			p.Type = "socks5"
		}
		if err := h.store.AddProxy(&p); err != nil {
			h.writeRPC(w, req.Resource, http.StatusInternalServerError, nil, err.Error())
			return
		}
		h.log.Info("proxy added", "id", p.ID, "name", p.Name, "type", p.Type, "host", p.Host)
		h.writeRPC(w, req.Resource, http.StatusCreated, p, "")

	case "PUT":
		var p db.Proxy
		if err := json.Unmarshal(req.Body, &p); err != nil {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "invalid body: "+err.Error())
			return
		}
		if p.ID == 0 {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "id is required")
			return
		}
		if err := h.store.UpdateProxy(&p); err != nil {
			h.writeRPC(w, req.Resource, http.StatusInternalServerError, nil, err.Error())
			return
		}
		h.log.Info("proxy updated", "id", p.ID, "name", p.Name)
		h.writeRPC(w, req.Resource, http.StatusOK, p, "")

	case "DELETE":
		var body proxyDeleteBody
		if err := json.Unmarshal(req.Body, &body); err != nil {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "invalid body: "+err.Error())
			return
		}
		if err := h.store.DeleteProxy(body.ID); err != nil {
			h.writeRPC(w, req.Resource, http.StatusInternalServerError, nil, err.Error())
			return
		}
		h.log.Info("proxy deleted", "id", body.ID)
		h.writeRPC(w, req.Resource, http.StatusOK, nil, "")

	default:
		h.writeRPC(w, req.Resource, http.StatusMethodNotAllowed, nil, "method not allowed")
	}
}

// --- Proxy Routes ---

type proxyRouteDeleteBody struct {
	ID int64 `json:"id"`
}

func (h *Handler) handleProxyRoutes(w http.ResponseWriter, req *rpcRequest) {
	switch req.Method {
	case "GET":
		routes, err := h.store.ListProxyRoutes()
		if err != nil {
			h.writeRPC(w, req.Resource, http.StatusInternalServerError, nil, err.Error())
			return
		}
		if routes == nil {
			routes = []db.ProxyRoute{}
		}
		h.writeRPC(w, req.Resource, http.StatusOK, routes, "")

	case "POST":
		var r db.ProxyRoute
		if err := json.Unmarshal(req.Body, &r); err != nil {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "invalid body: "+err.Error())
			return
		}
		if r.Destination == "" {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "destination is required")
			return
		}
		if r.ProxyID == 0 {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "proxy_id is required")
			return
		}
		if err := h.store.AddProxyRoute(&r); err != nil {
			h.writeRPC(w, req.Resource, http.StatusInternalServerError, nil, err.Error())
			return
		}
		h.log.Info("proxy route added", "id", r.ID, "destination", r.Destination, "proxy_id", r.ProxyID)
		h.writeRPC(w, req.Resource, http.StatusCreated, r, "")

	case "DELETE":
		var body proxyRouteDeleteBody
		if err := json.Unmarshal(req.Body, &body); err != nil {
			h.writeRPC(w, req.Resource, http.StatusBadRequest, nil, "invalid body: "+err.Error())
			return
		}
		if err := h.store.DeleteProxyRoute(body.ID); err != nil {
			h.writeRPC(w, req.Resource, http.StatusInternalServerError, nil, err.Error())
			return
		}
		h.log.Info("proxy route deleted", "id", body.ID)
		h.writeRPC(w, req.Resource, http.StatusOK, nil, "")

	default:
		h.writeRPC(w, req.Resource, http.StatusMethodNotAllowed, nil, "method not allowed")
	}
}
		h.writeRPC(w, req.Resource, http.StatusOK, nil, "")

	default:
		h.writeRPC(w, req.Resource, http.StatusMethodNotAllowed, nil, "method not allowed")
	}
}
