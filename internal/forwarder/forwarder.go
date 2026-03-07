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

// Package forwarder implements the dynamic HTTP/HTTPS relay for
// madexchanger. It routes emails to their destination based on the
// recipient's domain, matching how Madmail's target.remote delivers.
//
// # Delivery Flow (matches Madmail's tryHTTP)
//
//  1. Extract recipient domain from X-Mail-To header
//  2. Try HTTPS first: POST https://<domain>/mxdeliv
//  3. If HTTPS fails, try HTTP: POST http://<domain>/mxdeliv
//  4. If both fail, the delivery fails
//
// # Static Override
//
// If a fixed DownstreamURL is configured, ALL messages go there
// regardless of the recipient domain. Useful for chaining exchangers.
//
// # Wire format (compatible with Madmail):
//
//	POST /mxdeliv
//	X-Mail-From: sender@example.org
//	X-Mail-To: recipient1@example.org
//	X-Mail-To: recipient2@example.org
//	Content-Type: application/octet-stream
//
//	<RFC 822 message body: headers + body>
package forwarder

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/themadorg/madexchanger/internal/config"
	"github.com/themadorg/madexchanger/internal/logger"
	"golang.org/x/net/proxy"
)

// Forwarder relays email data to destination servers via HTTP POST.
// It routes dynamically by recipient domain or to a fixed downstream.
type Forwarder struct {
	// downstreamURL is the optional fixed destination override.
	// When non-empty, ALL messages go here instead of dynamic routing.
	downstreamURL string

	// client is the HTTP client used for outbound requests.
	client *http.Client

	log *logger.Logger
}

// New creates a Forwarder with the given settings.
//
// Parameters:
//   - downstreamURL: optional fixed destination (empty = dynamic routing)
//   - timeoutSec: request timeout in seconds
//   - skipTLSVerify: skip TLS certificate verification
//   - proxyURL: optional proxy URL (socks5://, http://, https://) or empty
//   - log: logger instance
func New(downstreamURL string, timeoutSec int, skipTLSVerify bool, proxyURL string, log *logger.Logger) *Forwarder {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipTLSVerify, //nolint:gosec // Intentional: self-signed cert support.
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	// Configure proxy if set.
	if proxyURL != "" {
		if strings.HasPrefix(proxyURL, "socks5://") {
			dialer, err := proxy.SOCKS5("tcp", strings.TrimPrefix(proxyURL, "socks5://"), nil, proxy.Direct)
			if err != nil {
				log.Error("failed to create SOCKS5 dialer", "err", err, "proxy", proxyURL)
			} else {
				transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
					return dialer.Dial(network, addr)
				}
				log.Info("outbound proxy configured", "type", "socks5", "proxy", proxyURL)
			}
		} else {
			parsed, err := url.Parse(proxyURL)
			if err != nil {
				log.Error("failed to parse proxy URL", "err", err, "proxy", proxyURL)
			} else {
				transport.Proxy = http.ProxyURL(parsed)
				log.Info("outbound proxy configured", "type", parsed.Scheme, "proxy", proxyURL)
			}
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(timeoutSec) * time.Second,
	}

	return &Forwarder{
		downstreamURL: downstreamURL,
		client:        client,
		log:           log,
	}
}

// ForwardResult holds the result of forwarding to one domain.
type ForwardResult struct {
	// Domain is the recipient domain this result is for.
	Domain string

	// TargetURL is the actual URL that was POSTed to.
	TargetURL string

	// Recipients is the list of recipient addresses in this domain group.
	Recipients []string

	// Err is non-nil if forwarding to this domain failed.
	Err error
}

// Forward sends an email to one or more destinations. Recipients are
// grouped by domain, and each unique domain gets its own delivery attempt.
//
// Dynamic mode (no DownstreamURL):
//   - Try HTTPS first: https://<domain>/mxdeliv
//   - If HTTPS fails, try HTTP: http://<domain>/mxdeliv
//   - Exactly matching Madmail's tryHTTP behavior
//
// Static mode (DownstreamURL set):
//   - Send everything to the fixed downstream URL
func (f *Forwarder) Forward(mailFrom string, mailTo []string, body []byte) []ForwardResult {
	if f.downstreamURL != "" {
		// Static mode: send everything to the fixed downstream.
		targetURL := strings.TrimRight(f.downstreamURL, "/") + config.DeliveryPath
		err := f.post(targetURL, mailFrom, mailTo, body)
		domain := "static"
		if len(mailTo) > 0 {
			domain = domainOf(mailTo[0])
		}
		return []ForwardResult{{
			Domain:     domain,
			TargetURL:  targetURL,
			Recipients: mailTo,
			Err:        err,
		}}
	}

	// Dynamic mode: group recipients by domain, try each domain.
	groups := groupByDomain(mailTo)
	results := make([]ForwardResult, 0, len(groups))

	for domain, rcpts := range groups {
		host := domain
		// For IPv6, wrap in brackets.
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}

		// Try HTTPS first, fall back to HTTP — matching Madmail's tryHTTP.
		httpsURL := "https://" + host + config.DeliveryPath
		err := f.post(httpsURL, mailFrom, rcpts, body)
		targetURL := httpsURL

		if err != nil {
			f.log.Info("HTTPS delivery failed, trying HTTP fallback",
				"err", err, "domain", domain)

			httpURL := "http://" + host + config.DeliveryPath
			err = f.post(httpURL, mailFrom, rcpts, body)
			targetURL = httpURL
		}

		results = append(results, ForwardResult{
			Domain:     domain,
			TargetURL:  targetURL,
			Recipients: rcpts,
			Err:        err,
		})
	}

	return results
}

// ResolveTarget returns the URL that would be used for a given domain.
// In dynamic mode returns the HTTPS URL (the first attempt).
func (f *Forwarder) ResolveTarget(domain string) string {
	if f.downstreamURL != "" {
		return strings.TrimRight(f.downstreamURL, "/") + config.DeliveryPath
	}
	host := domain
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return "https://" + host + config.DeliveryPath
}

// post performs a single HTTP POST to the given target URL.
func (f *Forwarder) post(targetURL, mailFrom string, mailTo []string, body []byte) error {
	f.log.Debug("forwarding email",
		"url", targetURL,
		"from", mailFrom,
		"to", strings.Join(mailTo, ","),
		"size", len(body),
	)

	req, err := http.NewRequest("POST", targetURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("forwarder: failed to create request: %w", err)
	}

	// Set envelope metadata headers — same format as Madmail's
	// target.remote (see remote.go:doHTTPRequest).
	req.Header.Set("X-Mail-From", mailFrom)
	for _, rcpt := range mailTo {
		req.Header.Add("X-Mail-To", rcpt)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("forwarder: request to %s failed: %w", targetURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Drain the response body to allow connection reuse.
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("forwarder: %s returned HTTP %d", targetURL, resp.StatusCode)
	}

	f.log.Info("email forwarded successfully",
		"url", targetURL,
		"from", mailFrom,
		"to", strings.Join(mailTo, ","),
		"size", len(body),
	)

	return nil
}

// groupByDomain groups recipient addresses by their email domain.
func groupByDomain(addrs []string) map[string][]string {
	groups := make(map[string][]string)
	for _, addr := range addrs {
		d := domainOf(addr)
		groups[d] = append(groups[d], addr)
	}
	return groups
}

// domainOf extracts the domain part from an email address.
func domainOf(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return email
}
