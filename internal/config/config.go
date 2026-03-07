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

// Package config provides configuration loading and validation
// for the madexchanger relay proxy. Configuration is read from a YAML
// file and validated for required fields and sane defaults.
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// DeliveryPath is the fixed path used for all email delivery.
// This matches Madmail's /mxdeliv endpoint and is not configurable.
const DeliveryPath = "/mxdeliv"

// TLSConfig holds TLS certificate paths for the inbound listener.
// When both CertFile and KeyFile are non-empty, the server starts
// in HTTPS mode using the provided PEM-encoded certificate and key.
// Self-signed certificates are fully supported.
type TLSConfig struct {
	// CertFile is the path to the PEM-encoded TLS certificate.
	CertFile string `yaml:"cert_file"`

	// KeyFile is the path to the PEM-encoded TLS private key.
	KeyFile string `yaml:"key_file"`
}

// ProxyConfig holds optional proxy configuration for outbound connections.
// When set, the forwarder routes outbound HTTP(S) requests through this proxy.
type ProxyConfig struct {
	// URL is the proxy URL:
	//   socks5://host:port  — SOCKS5 proxy
	//   http://host:port    — HTTP CONNECT proxy
	//   https://host:port   — HTTPS CONNECT proxy
	// Empty means no proxy (direct connection).
	URL string `yaml:"url"`
}

// AdminWebConfig holds configuration for the embedded admin dashboard.
type AdminWebConfig struct {
	// Enabled controls whether the admin web UI is served.
	Enabled bool `yaml:"enabled"`

	// Path is the URL path prefix for the admin UI (e.g., "/admin").
	Path string `yaml:"path"`

	// Token is the authentication token for the admin API.
	// Requests must include this as a Bearer token.
	Token string `yaml:"token"`
}

// Config holds all configuration values for the madexchanger relay.
//
// # Dynamic Routing
//
// The madexchanger routes dynamically based on the recipient's email
// domain. When a message arrives for b@2.2.2.2, the exchanger:
//  1. Tries HTTPS first: https://2.2.2.2/mxdeliv
//  2. Falls back to HTTP: http://2.2.2.2/mxdeliv
//
// This matches exactly how Madmail's target.remote delivers emails.
//
// If DownstreamURL is set, it overrides dynamic routing and ALL
// messages go to that fixed URL (for chaining exchangers).
type Config struct {
	// Listen is the address for the inbound HTTP/HTTPS listener.
	// Format: <host>:<port> (e.g., "0.0.0.0:8443", ":8080").
	Listen string `yaml:"listen"`

	// ReceivePath is the HTTP path where inbound email POST requests
	// are accepted. Must match the path sent by the upstream server.
	// Default: "/mxdeliv".
	ReceivePath string `yaml:"receive_path"`

	// TLS holds optional TLS certificate configuration for the inbound
	// listener. When both cert_file and key_file are set, HTTPS is enabled.
	TLS TLSConfig `yaml:"tls"`

	// DownstreamURL is an optional fixed destination override.
	// When empty (default), the exchanger routes dynamically by
	// extracting the domain from the recipient's email address
	// and trying HTTPS then HTTP.
	// When set, ALL messages are forwarded to this URL regardless
	// of the recipient domain. Useful for chaining exchangers.
	DownstreamURL string `yaml:"downstream_url"`

	// ForwardTimeout is the maximum duration (in seconds) for a single
	// forwarding HTTP request. Covers connect, TLS handshake, send, and
	// response. Default: 30.
	ForwardTimeout int `yaml:"forward_timeout"`

	// SkipTLSVerify disables TLS certificate verification for the
	// destination connections. Required when destinations use
	// self-signed certificates. Default: true (matching Madmail).
	SkipTLSVerify bool `yaml:"skip_tls_verify"`

	// Proxy holds optional proxy settings for outbound connections.
	Proxy ProxyConfig `yaml:"proxy"`

	// MaxBodySize is the maximum allowed size (in bytes) for incoming
	// email POST bodies. Requests exceeding this are rejected with
	// HTTP 413. Default: 33554432 (32 MiB).
	MaxBodySize int64 `yaml:"max_body_size"`

	// LogLevel controls log verbosity.
	// Accepted values: "debug", "info", "warn", "error".
	// Default: "info".
	LogLevel string `yaml:"log_level"`

	// RelayMode controls which messages are forwarded.
	//   "all"      — relay all incoming messages (default).
	//   "selected" — only relay messages matching relay filter rules.
	RelayMode string `yaml:"relay_mode"`

	// DatabasePath is the path to the SQLite database file for stats,
	// rewrite rules, and relay filters.
	// Default: "madexchanger.db".
	DatabasePath string `yaml:"database_path"`

	// AdminWeb configures the embedded admin dashboard.
	AdminWeb AdminWebConfig `yaml:"admin_web"`
}

// Load reads and parses a YAML configuration file, applies defaults
// for unset fields, and validates the resulting configuration.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: failed to read %s: %w", path, err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: failed to parse %s: %w", path, err)
	}

	cfg.applyDefaults()

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config: validation failed: %w", err)
	}

	return cfg, nil
}

// applyDefaults fills in default values for any unset configuration fields.
func (c *Config) applyDefaults() {
	if c.Listen == "" {
		c.Listen = "0.0.0.0:8443"
	}
	if c.ReceivePath == "" {
		c.ReceivePath = DeliveryPath
	}
	if c.ForwardTimeout <= 0 {
		c.ForwardTimeout = 30
	}
	if c.MaxBodySize <= 0 {
		c.MaxBodySize = 33554432 // 32 MiB
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.RelayMode == "" {
		c.RelayMode = "all"
	}
	if c.DatabasePath == "" {
		c.DatabasePath = "madexchanger.db"
	}
	if c.AdminWeb.Path == "" {
		c.AdminWeb.Path = "/admin"
	}
}

// validate checks that all required configuration fields are present
// and that values are within acceptable ranges.
func (c *Config) validate() error {
	// downstream_url is optional: when empty, dynamic routing is used.
	if c.DownstreamURL != "" {
		if !strings.HasPrefix(c.DownstreamURL, "http://") && !strings.HasPrefix(c.DownstreamURL, "https://") {
			return fmt.Errorf("downstream_url must start with http:// or https://")
		}
	}

	// Ensure the receive path starts with /.
	if !strings.HasPrefix(c.ReceivePath, "/") {
		return fmt.Errorf("receive_path must start with /")
	}

	// Validate log level.
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
		// Valid.
	default:
		return fmt.Errorf("log_level must be one of: debug, info, warn, error (got %q)", c.LogLevel)
	}

	// Validate relay mode.
	switch c.RelayMode {
	case "all", "selected":
		// Valid.
	default:
		return fmt.Errorf("relay_mode must be one of: all, selected (got %q)", c.RelayMode)
	}

	// Validate TLS config: both cert and key must be set, or neither.
	if (c.TLS.CertFile != "") != (c.TLS.KeyFile != "") {
		return fmt.Errorf("tls: both cert_file and key_file must be set, or neither")
	}

	// Validate proxy URL if set.
	if c.Proxy.URL != "" {
		if !strings.HasPrefix(c.Proxy.URL, "socks5://") &&
			!strings.HasPrefix(c.Proxy.URL, "http://") &&
			!strings.HasPrefix(c.Proxy.URL, "https://") {
			return fmt.Errorf("proxy.url must start with socks5://, http://, or https://")
		}
	}

	return nil
}

// HasTLS returns true if TLS certificate paths are configured for the
// inbound listener.
func (c *Config) HasTLS() bool {
	return c.TLS.CertFile != "" && c.TLS.KeyFile != ""
}
