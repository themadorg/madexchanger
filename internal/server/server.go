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

// Package server implements the inbound HTTP/HTTPS listener for
// madexchanger. It accepts email POST requests on a configurable path,
// validates the envelope headers (X-Mail-From, X-Mail-To), reads the
// RFC 822 message body, applies rewrite rules and relay filters, and
// hands it off to the forwarder for downstream delivery.
//
// The server also hosts the admin API and admin web dashboard on
// configurable paths for monitoring and management.
package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/themadorg/madexchanger/internal/adminapi"
	"github.com/themadorg/madexchanger/internal/adminweb"
	"github.com/themadorg/madexchanger/internal/config"
	"github.com/themadorg/madexchanger/internal/db"
	"github.com/themadorg/madexchanger/internal/forwarder"
	"github.com/themadorg/madexchanger/internal/logger"
)

// Server is the inbound HTTP/HTTPS listener that receives email
// POST requests and relays them to the downstream server.
type Server struct {
	cfg      *config.Config
	fwd      *forwarder.Forwarder
	store    *db.DB
	log      *logger.Logger
	httpServ *http.Server
	admin    *adminapi.Handler

	// Metrics (atomic counters).
	received  atomic.Int64
	forwarded atomic.Int64
	errors    atomic.Int64
	pulled    atomic.Int64
	queued    atomic.Int64
}

// New creates a new Server with the given configuration, forwarder,
// database, and logger. The server is not started until [Server.Run]
// is called.
func New(cfg *config.Config, fwd *forwarder.Forwarder, store *db.DB, log *logger.Logger) *Server {
	s := &Server{
		cfg:   cfg,
		fwd:   fwd,
		store: store,
		log:   log,
	}

	// Initialize admin API handler.
	if store != nil {
		s.admin = adminapi.New(cfg, store, log)
	}

	return s
}

// Run starts the HTTP/HTTPS listener and blocks until the server
// is shut down. On error, it returns the underlying listen/serve error.
// Use [Server.Shutdown] to stop the server gracefully.
func (s *Server) Run() error {
	mux := http.NewServeMux()

	// Email relay endpoint.
	mux.HandleFunc(s.cfg.ReceivePath, s.handleReceive)

	// Health check.
	mux.HandleFunc("/health", s.handleHealth)

	// Pull-based delivery (Phase C): destination polls for queued mail.
	if s.cfg.Pull.Enabled {
		path := s.cfg.Pull.Path
		if path == "" {
			path = "/pull"
		}
		mux.HandleFunc(path, s.handlePull)
		mux.HandleFunc(path+"/ack", s.handlePullAck)
		s.log.Info("pull handlers registered", "path", path, "on_failure", s.cfg.Pull.OnFailure)
	}

	// Peer discovery (Phase D).
	mux.HandleFunc("/peers", s.handlePeers)

	// Admin API — single RPC endpoint (matching Madmail's pattern).
	if s.admin != nil {
		mux.Handle("/api/admin", s.admin)
	}

	// Admin web dashboard (embedded SPA, matching Madmail's pattern).
	if s.cfg.AdminWeb.Enabled {
		prefix := s.cfg.AdminWeb.Path
		handler := s.serveAdminWeb(prefix)
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		mux.HandleFunc(prefix, handler)
		// Also handle exact path without trailing slash.
		trimmed := strings.TrimSuffix(prefix, "/")
		if trimmed != "" {
			mux.HandleFunc(trimmed, func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, prefix, http.StatusMovedPermanently)
			})
		}
		s.log.Info("admin web UI registered", "path", prefix)
	}

	s.httpServ = &http.Server{
		Addr:         s.cfg.Listen,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	scheme := "http"
	if s.cfg.HasTLS() {
		scheme = "https"
		s.httpServ.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	s.log.Info("madexchanger starting",
		"listen", s.cfg.Listen,
		"scheme", scheme,
		"receive_path", s.cfg.ReceivePath,
		"relay_mode", s.cfg.RelayMode,
	)

	if s.cfg.HasTLS() {
		return s.httpServ.ListenAndServeTLS(s.cfg.TLS.CertFile, s.cfg.TLS.KeyFile)
	}
	return s.httpServ.ListenAndServe()
}

// Shutdown gracefully stops the server with the given context deadline.
// In-flight requests are allowed to complete until the context expires.
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("shutting down",
		"received", s.received.Load(),
		"forwarded", s.forwarded.Load(),
		"errors", s.errors.Load(),
	)
	return s.httpServ.Shutdown(ctx)
}

// handleReceive processes inbound email POST requests. It validates
// the HTTP method and required headers, reads the message body (up to
// the configured max size), applies rewrite rules and relay filters,
// and forwards it downstream.
func (s *Server) handleReceive(w http.ResponseWriter, r *http.Request) {
	s.received.Add(1)

	s.log.Debug("inbound request",
		"remote", r.RemoteAddr,
		"method", r.Method,
		"path", r.URL.Path,
	)

	// Only POST is accepted — matches Madmail's handleReceiveEmail.
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		s.errors.Add(1)
		return
	}

	// Read envelope metadata from headers.
	mailFrom := r.Header.Get("X-Mail-From")
	mailTo := r.Header.Values("X-Mail-To")

	if len(mailTo) == 0 {
		s.log.Warn("rejected: missing X-Mail-To header", "remote", r.RemoteAddr)
		http.Error(w, "Missing X-Mail-To header", http.StatusBadRequest)
		if s.store != nil {
			_ = s.store.RecordRejected()
		}
		s.errors.Add(1)
		return
	}

	// Read the message body with a size limit to prevent abuse.
	limitedReader := io.LimitReader(r.Body, s.cfg.MaxBodySize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		s.log.Error("failed to read request body", "err", err, "remote", r.RemoteAddr)
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		s.errors.Add(1)
		return
	}

	// Check if the body exceeded the max size.
	if int64(len(body)) > s.cfg.MaxBodySize {
		s.log.Warn("rejected: body too large",
			"size", len(body), "max", s.cfg.MaxBodySize, "remote", r.RemoteAddr)
		http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		if s.store != nil {
			_ = s.store.RecordRejected()
		}
		s.errors.Add(1)
		return
	}

	if len(body) == 0 {
		s.log.Warn("rejected: empty body", "remote", r.RemoteAddr)
		http.Error(w, "Empty message body", http.StatusBadRequest)
		if s.store != nil {
			_ = s.store.RecordRejected()
		}
		s.errors.Add(1)
		return
	}

	// Apply relay mode filtering.
	if s.cfg.RelayMode == "selected" && !s.matchesFilter(mailFrom, mailTo) {
		s.log.Info("rejected by relay filter",
			"from", mailFrom, "to", mailTo, "remote", r.RemoteAddr)
		http.Error(w, "Not allowed by relay filter", http.StatusForbidden)
		if s.store != nil {
			_ = s.store.RecordRejected()
		}
		s.errors.Add(1)
		return
	}

	// Apply rewrite rules.
	mailFrom, mailTo = s.applyRewrites(mailFrom, mailTo)

	// Extract the source server from the remote address.
	fromServer := remoteHost(r.RemoteAddr)

	// Split recipients: always-pull domains vs push domains.
	var pushTo, pullTo []string
	for _, rcpt := range mailTo {
		dom := domainOf(rcpt)
		if s.pullAlways(dom) {
			pullTo = append(pullTo, rcpt)
		} else {
			pushTo = append(pushTo, rcpt)
		}
	}

	var hasError, hasSuccess bool

	// Queue always-pull domains without attempting push.
	if len(pullTo) > 0 && s.cfg.Pull.Enabled && s.store != nil {
		for _, rcpt := range pullTo {
			dom := domainOf(rcpt)
			if id, err := s.store.EnqueuePull(dom, mailFrom, rcpt, body); err != nil {
				hasError = true
				s.log.Error("pull enqueue failed", "err", err, "domain", dom)
				s.errors.Add(1)
			} else {
				hasSuccess = true
				s.queued.Add(1)
				s.log.Info("email queued for pull", "id", id, "domain", dom, "to", rcpt, "size", len(body))
			}
		}
	}

	// Push remaining recipients.
	if len(pushTo) > 0 {
		results := s.fwd.Forward(mailFrom, pushTo, body)
		for _, res := range results {
			if res.Err != nil {
				// On failure, optionally store for pull.
				if s.cfg.Pull.Enabled && s.cfg.Pull.OnFailure && s.store != nil {
					for _, rcpt := range res.Recipients {
						if id, err := s.store.EnqueuePull(res.Domain, mailFrom, rcpt, body); err != nil {
							hasError = true
							s.log.Error("pull enqueue after push fail", "err", err, "domain", res.Domain)
							s.errors.Add(1)
						} else {
							hasSuccess = true
							s.queued.Add(1)
							s.log.Info("email queued for pull after push fail",
								"id", id, "domain", res.Domain, "err", res.Err)
						}
					}
				} else {
					hasError = true
					s.log.Error("forwarding failed",
						"err", res.Err, "domain", res.Domain, "target", res.TargetURL,
						"from", mailFrom, "to", res.Recipients, "remote", r.RemoteAddr)
					if s.store != nil {
						_ = s.store.RecordError()
					}
					s.errors.Add(1)
				}
			} else {
				hasSuccess = true
				s.forwarded.Add(1)
				if s.store != nil {
					_ = s.store.RecordRelay(fromServer, res.Domain, int64(len(body)))
				}
				scheme := "http"
				if r.TLS != nil {
					scheme = "https"
				}
				s.log.Info("email relayed",
					"scheme", scheme, "from", mailFrom, "to", res.Recipients,
					"domain", res.Domain, "target", res.TargetURL,
					"size", len(body), "remote", r.RemoteAddr)
			}
		}
	}

	if hasError && !hasSuccess {
		http.Error(w, "Forwarding failed for all recipients", http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// pullAlways reports whether domain is configured for always-pull (no push).
func (s *Server) pullAlways(domain string) bool {
	if !s.cfg.Pull.Enabled {
		return false
	}
	dom := strings.Trim(strings.ToLower(domain), "[]")
	for _, d := range s.cfg.Pull.Domains {
		if strings.Trim(strings.ToLower(d), "[]") == dom {
			return true
		}
	}
	return false
}

// matchesFilter checks if the message matches any enabled relay filter.
// Used when relay_mode is "selected".
func (s *Server) matchesFilter(mailFrom string, mailTo []string) bool {
	if s.store == nil {
		return false
	}

	filters, err := s.store.ListRelayFilters()
	if err != nil {
		s.log.Error("failed to load relay filters", "err", err)
		return false
	}

	if len(filters) == 0 {
		// No filters defined in "selected" mode = deny all.
		return false
	}

	toJoined := strings.Join(mailTo, ",")
	for _, f := range filters {
		if !f.Enabled {
			continue
		}
		switch f.Field {
		case "mail_from":
			if matchPattern(f.Pattern, mailFrom) {
				return true
			}
		case "mail_to":
			for _, to := range mailTo {
				if matchPattern(f.Pattern, to) {
					return true
				}
			}
		case "domain":
			// Check sender domain.
			if matchPattern(f.Pattern, domainOf(mailFrom)) {
				return true
			}
			// Check all recipient domains.
			if matchPattern(f.Pattern, domainOf(toJoined)) {
				return true
			}
			for _, to := range mailTo {
				if matchPattern(f.Pattern, domainOf(to)) {
					return true
				}
			}
		}
	}
	return false
}

// applyRewrites applies all enabled rewrite rules to the envelope.
func (s *Server) applyRewrites(mailFrom string, mailTo []string) (string, []string) {
	if s.store == nil {
		return mailFrom, mailTo
	}

	rules, err := s.store.ListRewriteRules()
	if err != nil {
		s.log.Error("failed to load rewrite rules", "err", err)
		return mailFrom, mailTo
	}

	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		switch r.Field {
		case "mail_from":
			if matchPattern(r.Pattern, mailFrom) {
				s.log.Debug("rewrite mail_from", "from", mailFrom, "to", r.Replacement)
				mailFrom = r.Replacement
			}
		case "mail_to":
			for i, to := range mailTo {
				if matchPattern(r.Pattern, to) {
					s.log.Debug("rewrite mail_to", "from", to, "to", r.Replacement)
					mailTo[i] = r.Replacement
				}
			}
		case "downstream":
			// Downstream rewrites are logged but handled in the forwarder.
			s.log.Debug("downstream rewrite rule active", "pattern", r.Pattern, "replacement", r.Replacement)
		}
	}

	return mailFrom, mailTo
}

// remoteHost extracts the host (IP) from a remote address like "1.2.3.4:5678".
func remoteHost(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// handleHealth returns HTTP 200 with a simple status message.
// Useful for load balancer health checks and monitoring.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var pending int64
	if s.store != nil && s.cfg.Pull.Enabled {
		pending, _ = s.store.CountPull("")
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w,
		`{"status":"ok","received":%d,"forwarded":%d,"errors":%d,"queued_pull":%d,"pulled":%d,"relay_mode":"%s","pull_enabled":%v}`,
		s.received.Load(), s.forwarded.Load(), s.errors.Load(),
		pending, s.pulled.Load(), s.cfg.RelayMode, s.cfg.Pull.Enabled)
}

// handlePull lists queued messages for a domain (Bearer token required).
// GET /pull?domain=delta.sudoshz.ir
func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorizePull(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	if domain == "" {
		http.Error(w, "domain query parameter required", http.StatusBadRequest)
		return
	}
	if s.store == nil {
		http.Error(w, "store unavailable", http.StatusServiceUnavailable)
		return
	}
	msgs, err := s.store.ListPullByDomain(domain, 50)
	if err != nil {
		s.log.Error("pull list failed", "err", err, "domain", domain)
		http.Error(w, "pull list failed", http.StatusInternalServerError)
		return
	}
	if msgs == nil {
		msgs = []db.PullMessage{}
	}
	// Encode body as base64-friendly: return raw body as base64 in JSON via string
	type pullItem struct {
		ID       int64  `json:"id"`
		Domain   string `json:"domain"`
		MailFrom string `json:"mail_from"`
		MailTo   string `json:"mail_to"`
		Body     string `json:"body"` // raw RFC822 string
		Created  string `json:"created_at"`
	}
	out := make([]pullItem, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, pullItem{
			ID: m.ID, Domain: m.Domain, MailFrom: m.MailFrom, MailTo: m.MailTo,
			Body: string(m.Body), Created: m.CreatedAt,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"domain":   domain,
		"count":    len(out),
		"messages": out,
	})
	s.log.Info("pull listed", "domain", domain, "count", len(out))
}

// handlePullAck deletes pulled message IDs after the client delivered them.
// POST /pull/ack  {"ids":[1,2,3]}
func (s *Server) handlePullAck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorizePull(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if s.store == nil {
		http.Error(w, "store unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := s.store.DeletePull(body.IDs); err != nil {
		http.Error(w, "ack failed", http.StatusInternalServerError)
		return
	}
	s.pulled.Add(int64(len(body.IDs)))
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"status":"ok","acked":%d}`, len(body.IDs))
	s.log.Info("pull acked", "count", len(body.IDs))
}

func (s *Server) authorizePull(r *http.Request) bool {
	tok := s.cfg.Pull.Token
	if tok == "" {
		tok = s.cfg.AdminWeb.Token
	}
	if tok == "" {
		return false
	}
	h := r.Header.Get("Authorization")
	return h == "Bearer "+tok
}

// handlePeers returns a static peer directory for discovery (Phase D).
// GET /peers — optional Bearer (admin token) when peers_require_auth is set later.
func (s *Server) handlePeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Built-in lab peers; can be replaced by config file in future.
	peers := []map[string]string{
		{"id": "delta", "domain": "delta.sudoshz.ir", "mxdeliv": "https://delta.sudoshz.ir/mxdeliv", "note": "internal madmail"},
		{"id": "alireza", "domain": "172.104.234.13", "mxdeliv": "https://172.104.234.13/mxdeliv", "note": "external IP madmail"},
		{"id": "exchanger", "domain": "madexchanger", "mxdeliv": "http://127.0.0.1:19080/mxdeliv", "note": "this relay"},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"version": 1,
		"peers":   peers,
	})
}

// --- Admin Web SPA (embedded, matching Madmail's pattern) ---

// serveAdminWeb creates an HTTP handler that serves the embedded admin-web SPA
// under the given prefix path. It handles:
//   - Static assets (JS, CSS, images, fonts) with correct MIME types
//   - SPA fallback: any path that doesn't match a real file returns index.html
//   - Dynamic path rewriting in index.html so the SPA works under any prefix
func (s *Server) serveAdminWeb(prefix string) http.HandlerFunc {
	// Build a sub-filesystem from the embedded admin-web build directory.
	adminFS, err := fs.Sub(adminweb.Files, "build")
	if err != nil {
		s.log.Error("failed to open embedded admin-web filesystem", "err", err)
		return func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Admin web UI not available", http.StatusInternalServerError)
		}
	}

	// Check if the admin-web build is actually available (not just the placeholder).
	if _, err := fs.ReadFile(adminFS, "index.html"); err != nil {
		s.log.Info("admin web UI not available (admin-web not built)")
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`<!doctype html><html><body><h1>Admin Web UI Not Available</h1><p>The admin web dashboard was not included in this build. Build admin-web first, then rebuild the server.</p></body></html>`))
		}
	}

	// Pre-read and patch index.html once at startup.
	indexBytes, _ := fs.ReadFile(adminFS, "index.html")

	// Ensure prefix has trailing slash for path rewriting.
	prefixWithSlash := prefix
	if !strings.HasSuffix(prefixWithSlash, "/") {
		prefixWithSlash += "/"
	}

	// Rewrite paths in index.html so the SPA works under any prefix:
	//   /_app/ → /admin/_app/
	//   base: "" → base: "/admin"
	patchedIndex := string(indexBytes)
	patchedIndex = strings.ReplaceAll(patchedIndex, `href="/`, `href="`+prefixWithSlash)
	patchedIndex = strings.ReplaceAll(patchedIndex, `src="/`, `src="`+prefixWithSlash)
	patchedIndex = strings.ReplaceAll(patchedIndex, `import("/`, `import("`+prefixWithSlash)
	cleanPrefix := strings.TrimSuffix(prefix, "/")
	patchedIndex = strings.ReplaceAll(patchedIndex, `base: ""`, `base: "`+cleanPrefix+`"`)
	patchedIndexBytes := []byte(patchedIndex)

	return func(w http.ResponseWriter, r *http.Request) {
		// CORS headers.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Strip the prefix to get the file path within the SPA.
		path := strings.TrimPrefix(r.URL.Path, prefix)
		path = strings.TrimPrefix(path, "/")

		// Empty path → serve patched index.html.
		if path == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write(patchedIndexBytes)
			return
		}

		// Try to serve the file from the embedded FS.
		f, err := adminFS.Open(path)
		if err != nil {
			// File not found → SPA fallback: serve patched index.html.
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write(patchedIndexBytes)
			return
		}
		_ = f.Close()

		// Check if it's a directory.
		stat, err := fs.Stat(adminFS, path)
		if err != nil || stat.IsDir() {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write(patchedIndexBytes)
			return
		}

		// Read the file content.
		data, err := fs.ReadFile(adminFS, path)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Determine content type.
		contentType := mime.TypeByExtension(filepath.Ext(path))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", contentType)

		// Immutable assets get aggressive caching (fingerprinted filenames).
		if strings.Contains(path, "/immutable/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}

		_, _ = w.Write(data)
	}
}

// --- Pattern matching helpers ---

// matchPattern checks if value matches a simple pattern.
// Supports exact match and wildcard (*) prefix/suffix.
func matchPattern(pattern, value string) bool {
	if pattern == value {
		return true
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(value, pattern[1:])
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(value, pattern[:len(pattern)-1])
	}
	return false
}

// domainOf extracts the domain part from an email address.
func domainOf(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return email
}
