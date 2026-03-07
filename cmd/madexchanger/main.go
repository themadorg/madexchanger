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

// Madexchanger is an HTTP/HTTPS email relay proxy designed for the
// Madmail ecosystem. It receives email messages via HTTP POST (using
// Madmail's /mxdeliv wire format) and forwards them to a downstream
// server over HTTP or HTTPS.
//
// This is useful for:
//   - Relaying email between servers that communicate via HTTP
//   - Proxying email delivery through an intermediary node
//   - Bridging networks where direct server-to-server delivery is
//     not possible (NAT, firewalls, split DNS, etc.)
//
// Usage:
//
//	madexchanger [flags]
//
// Flags:
//
//	-config string    Path to config.yml (default "config.yml")
//	-version          Print version and exit
//
// See config.yml.example for full configuration reference.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/themadorg/madexchanger/internal/config"
	"github.com/themadorg/madexchanger/internal/db"
	"github.com/themadorg/madexchanger/internal/forwarder"
	"github.com/themadorg/madexchanger/internal/logger"
	"github.com/themadorg/madexchanger/internal/server"
)

// Version is set at build time via -ldflags.
//
//	go build -ldflags "-X main.Version=1.0.0" ./cmd/madexchanger
var Version = "dev"

func main() {
	configPath := flag.String("config", "config.yml", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("madexchanger %s\n", Version)
		os.Exit(0)
	}

	// Check for subcommands (proxy, proxy-route).
	args := flag.Args()
	if len(args) > 0 {
		runSubcommand(*configPath, args)
		return
	}

	// Load and validate configuration.
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger.
	log := logger.New(cfg.LogLevel)

	log.Info("configuration loaded",
		"config", *configPath,
		"listen", cfg.Listen,
		"routing", routingMode(cfg),
		"tls", cfg.HasTLS(),
		"skip_tls_verify", cfg.SkipTLSVerify,
		"relay_mode", cfg.RelayMode,
	)

	// Initialize the SQLite database.
	store, err := db.Open(cfg.DatabasePath)
	if err != nil {
		log.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer func() { _ = store.Close() }()

	log.Info("database opened", "path", cfg.DatabasePath)

	// Initialize the forwarder (dynamic routing or static downstream).
	fwd := forwarder.New(
		cfg.DownstreamURL,
		cfg.ForwardTimeout,
		cfg.SkipTLSVerify,
		cfg.Proxy.URL,
		log,
	)

	// Wire up DB-based proxy resolution for per-destination routing.
	fwd.SetProxyResolver(&dbProxyResolver{store: store, log: log})

	// Initialize and start the inbound server.
	srv := server.New(cfg, fwd, store, log)

	// Graceful shutdown on SIGINT/SIGTERM.
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.Run(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// Block until a signal is received.
	sig := <-done
	log.Info("received signal, shutting down", "signal", sig)

	// Allow 10 seconds for in-flight requests to complete.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown error", "err", err)
		os.Exit(1)
	}

	log.Info("madexchanger stopped")
}

// routingMode returns a human-readable description of the routing configuration.
func routingMode(cfg *config.Config) string {
	if cfg.DownstreamURL != "" {
		return "static → " + cfg.DownstreamURL + config.DeliveryPath
	}
	return "dynamic → https://<recipient-domain>" + config.DeliveryPath
}

// dbProxyResolver adapts db.DB to forwarder.ProxyResolver.
type dbProxyResolver struct {
	store *db.DB
	log   *logger.Logger
}

func (r *dbProxyResolver) ResolveProxy(destination string) (*forwarder.ProxyInfo, error) {
	p, err := r.store.ResolveProxy(destination)
	if err != nil || p == nil {
		return nil, err
	}
	return &forwarder.ProxyInfo{
		Type:     p.Type,
		Host:     p.Host,
		Username: p.Username,
		Password: p.Password,
		Name:     p.Name,
	}, nil
}
