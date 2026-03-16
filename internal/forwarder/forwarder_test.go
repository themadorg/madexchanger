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

package forwarder

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/themadorg/madexchanger/internal/logger"
)

func TestStaticForwardSuccess(t *testing.T) {
	var gotFrom, gotTo string
	var gotBody []byte
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFrom = r.Header.Get("X-Mail-From")
		gotTo = strings.Join(r.Header.Values("X-Mail-To"), ",")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer downstream.Close()

	log := logger.New("debug")
	fwd := New(downstream.URL, 5, true, "", log)

	body := []byte("From: a@1.1.1.1\r\nTo: b@2.2.2.2\r\n\r\nHello!")
	results := fwd.Forward("a@1.1.1.1", []string{"b@2.2.2.2", "c@3.3.3.3"}, body)

	// Static mode: single result for all recipients.
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("unexpected error: %v", results[0].Err)
	}
	if gotFrom != "a@1.1.1.1" {
		t.Errorf("X-Mail-From = %q, want a@1.1.1.1", gotFrom)
	}
	if gotTo != "b@2.2.2.2,c@3.3.3.3" {
		t.Errorf("X-Mail-To = %q, want b@2.2.2.2,c@3.3.3.3", gotTo)
	}
	if string(gotBody) != string(body) {
		t.Errorf("body mismatch")
	}
}

func TestDynamicForwardGroupsByDomain(t *testing.T) {
	groups := groupByDomain([]string{
		"alice@1.1.1.1",
		"bob@2.2.2.2",
		"charlie@1.1.1.1",
		"dave@3.3.3.3",
		"eve@[1.1.1.1]", // bracketed IPv4 should group with bare
	})

	if len(groups) != 3 {
		t.Fatalf("expected 3 domain groups, got %d: %v", len(groups), groups)
	}
	if len(groups["1.1.1.1"]) != 3 {
		t.Errorf("1.1.1.1 group has %d members, want 3 (bare + bracketed)", len(groups["1.1.1.1"]))
	}
	if len(groups["2.2.2.2"]) != 1 {
		t.Errorf("2.2.2.2 group has %d members, want 1", len(groups["2.2.2.2"]))
	}
}

func TestStaticForwardDownstreamError(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer downstream.Close()

	log := logger.New("error")
	fwd := New(downstream.URL, 5, true, "", log)

	results := fwd.Forward("a@x.org", []string{"b@y.org"}, []byte("body"))

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestForwardConnectionRefused(t *testing.T) {
	log := logger.New("error")
	// Use a static downstream that's definitely not running.
	fwd := New("http://127.0.0.1:1", 2, true, "", log)

	results := fwd.Forward("a@x.org", []string{"b@y.org"}, []byte("body"))
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Fatal("expected connection refused error")
	}
}

func TestResolveTarget(t *testing.T) {
	log := logger.New("error")

	// Static mode.
	fwd := New("https://gateway.example.com", 5, true, "", log)
	if got := fwd.ResolveTarget("2.2.2.2"); got != "https://gateway.example.com/mxdeliv" {
		t.Errorf("static resolve = %q", got)
	}

	// Dynamic mode.
	fwd2 := New("", 5, true, "", log)
	if got := fwd2.ResolveTarget("2.2.2.2"); got != "https://2.2.2.2/mxdeliv" {
		t.Errorf("dynamic resolve = %q, want https://2.2.2.2/mxdeliv", got)
	}

	// Dynamic mode — bracketed IPv4 (the root cause bug).
	if got := fwd2.ResolveTarget("[10.0.0.1]"); got != "https://10.0.0.1/mxdeliv" {
		t.Errorf("bracketed ipv4 resolve = %q, want https://10.0.0.1/mxdeliv", got)
	}

	// IPv6.
	if got := fwd2.ResolveTarget("::1"); got != "https://[::1]/mxdeliv" {
		t.Errorf("ipv6 resolve = %q", got)
	}
}

func TestDomainOf(t *testing.T) {
	tests := []struct{ input, want string }{
		{"alice@example.org", "example.org"},
		{"bob@1.2.3.4", "1.2.3.4"},
		{"carol@[10.0.0.1]", "10.0.0.1"}, // bracketed IPv4
		{"noatsign", "noatsign"},
		{"@only-domain", "only-domain"},
	}
	for _, tt := range tests {
		if got := domainOf(tt.input); got != tt.want {
			t.Errorf("domainOf(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHostForURL(t *testing.T) {
	tests := []struct{ input, want string }{
		{"1.2.3.4", "1.2.3.4"},         // bare IPv4
		{"[10.0.0.1]", "10.0.0.1"},     // bracketed IPv4 → strip
		{"example.org", "example.org"}, // domain name
		{"::1", "[::1]"},               // IPv6 → wrap
		{"[::1]", "[::1]"},             // already bracketed IPv6
	}
	for _, tt := range tests {
		if got := hostForURL(tt.input); got != tt.want {
			t.Errorf("hostForURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
