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

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	content := `
listen: "0.0.0.0:9090"
receive_path: "/mxdeliv"
downstream_url: "https://10.0.0.5"
forward_path: "/mxdeliv"
forward_timeout: 60
skip_tls_verify: true
max_body_size: 10485760
log_level: "debug"
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Listen != "0.0.0.0:9090" {
		t.Errorf("listen = %q, want %q", cfg.Listen, "0.0.0.0:9090")
	}
	if cfg.DownstreamURL != "https://10.0.0.5" {
		t.Errorf("downstream_url = %q, want %q", cfg.DownstreamURL, "https://10.0.0.5")
	}
	if cfg.ForwardTimeout != 60 {
		t.Errorf("forward_timeout = %d, want 60", cfg.ForwardTimeout)
	}
	if cfg.MaxBodySize != 10485760 {
		t.Errorf("max_body_size = %d, want 10485760", cfg.MaxBodySize)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("log_level = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestLoadDefaults(t *testing.T) {
	content := `
log_level: "info"
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Listen != "0.0.0.0:8443" {
		t.Errorf("listen default = %q, want %q", cfg.Listen, "0.0.0.0:8443")
	}
	if cfg.ReceivePath != "/mxdeliv" {
		t.Errorf("receive_path default = %q, want %q", cfg.ReceivePath, "/mxdeliv")
	}
	if cfg.ForwardTimeout != 30 {
		t.Errorf("forward_timeout default = %d, want 30", cfg.ForwardTimeout)
	}
	if cfg.MaxBodySize != 33554432 {
		t.Errorf("max_body_size default = %d, want 33554432", cfg.MaxBodySize)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("log_level default = %q, want %q", cfg.LogLevel, "info")
	}
	// downstream_url empty = dynamic routing mode.
	if cfg.DownstreamURL != "" {
		t.Errorf("downstream_url should be empty for dynamic routing, got %q", cfg.DownstreamURL)
	}
}

func TestLoadDynamicRouting(t *testing.T) {
	// No downstream_url = dynamic routing mode (should succeed).
	content := `
listen: ":8080"
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error for dynamic routing, got: %v", err)
	}
	if cfg.DownstreamURL != "" {
		t.Errorf("downstream_url should be empty, got %q", cfg.DownstreamURL)
	}
}

func TestLoadInvalidScheme(t *testing.T) {
	content := `
downstream_url: "ftp://10.0.0.5"
`
	path := writeTemp(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid scheme")
	}
}

func TestLoadInvalidLogLevel(t *testing.T) {
	content := `
downstream_url: "https://10.0.0.5"
log_level: "verbose"
`
	path := writeTemp(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid log_level")
	}
}

func TestLoadPartialTLS(t *testing.T) {
	content := `
downstream_url: "https://10.0.0.5"
tls:
  cert_file: "/tmp/cert.pem"
`
	path := writeTemp(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for partial TLS config (cert without key)")
	}
}

func TestHasTLS(t *testing.T) {
	cfg := &Config{
		DownstreamURL: "https://10.0.0.5",
		TLS: TLSConfig{
			CertFile: "/tmp/cert.pem",
			KeyFile:  "/tmp/key.pem",
		},
	}
	if !cfg.HasTLS() {
		t.Error("HasTLS() = false, want true")
	}

	cfg.TLS = TLSConfig{}
	if cfg.HasTLS() {
		t.Error("HasTLS() = true, want false")
	}
}

func TestLoadInvalidReceivePath(t *testing.T) {
	content := `
downstream_url: "https://10.0.0.5"
receive_path: "no-slash"
`
	path := writeTemp(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for receive_path without leading slash")
	}
}

// writeTemp creates a temporary YAML file for testing and returns its path.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}
