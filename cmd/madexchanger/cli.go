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

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/themadorg/madexchanger/internal/config"
	"github.com/themadorg/madexchanger/internal/db"
)

// runSubcommand handles CLI subcommands (proxy, proxy-route).
//
// Usage:
//
//	madexchanger proxy list
//	madexchanger proxy add <name> <type> <host:port> [--default] [--username=USER] [--password=PASS] [--comment=TEXT]
//	madexchanger proxy remove <id>
//	madexchanger proxy-route list
//	madexchanger proxy-route add <destination> <proxy_id> [--comment=TEXT]
//	madexchanger proxy-route remove <id>
func runSubcommand(configPath string, args []string) {
	if len(args) < 2 {
		printCLIUsage()
		os.Exit(1)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}

	store, err := db.Open(cfg.DatabasePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = store.Close() }()

	switch args[0] {
	case "proxy":
		runProxyCmd(store, args[1:])
	case "proxy-route":
		runProxyRouteCmd(store, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", args[0])
		printCLIUsage()
		os.Exit(1)
	}
}

// --- proxy subcommand ---

func runProxyCmd(store *db.DB, args []string) {
	switch args[0] {
	case "list":
		proxies, err := store.ListProxies()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "ID\tNAME\tTYPE\tHOST\tAUTH\tENABLED\tDEFAULT\tCOMMENT")
		for _, p := range proxies {
			auth := "—"
			if p.Username != "" {
				auth = p.Username
			}
			enabled := "✗"
			if p.Enabled {
				enabled = "✓"
			}
			def := ""
			if p.IsDefault {
				def = "★"
			}
			_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				p.ID, p.Name, p.Type, p.Host, auth, enabled, def, p.Comment)
		}
		_ = tw.Flush()

	case "add":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "usage: madexchanger proxy add <name> <type> <host:port> [options]")
			fmt.Fprintln(os.Stderr, "  type: socks5, http, https")
			fmt.Fprintln(os.Stderr, "  options: --default --username=USER --password=PASS --comment=TEXT")
			os.Exit(1)
		}

		p := &db.Proxy{
			Name:    args[1],
			Type:    args[2],
			Host:    args[3],
			Enabled: true,
		}

		for _, arg := range args[4:] {
			switch {
			case arg == "--default":
				p.IsDefault = true
			case strings.HasPrefix(arg, "--username="):
				p.Username = strings.TrimPrefix(arg, "--username=")
			case strings.HasPrefix(arg, "--password="):
				p.Password = strings.TrimPrefix(arg, "--password=")
			case strings.HasPrefix(arg, "--comment="):
				p.Comment = strings.TrimPrefix(arg, "--comment=")
			default:
				fmt.Fprintf(os.Stderr, "unknown option: %s\n", arg)
				os.Exit(1)
			}
		}

		if err := store.AddProxy(p); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("proxy added: id=%d name=%s type=%s host=%s\n", p.ID, p.Name, p.Type, p.Host)

	case "remove":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: madexchanger proxy remove <id>")
			os.Exit(1)
		}
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid id: %s\n", args[1])
			os.Exit(1)
		}
		if err := store.DeleteProxy(id); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("proxy removed: id=%d\n", id)

	default:
		fmt.Fprintf(os.Stderr, "unknown proxy subcommand: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "usage: madexchanger proxy [list|add|remove]")
		os.Exit(1)
	}
}

// --- proxy-route subcommand ---

func runProxyRouteCmd(store *db.DB, args []string) {
	switch args[0] {
	case "list":
		routes, err := store.ListProxyRoutes()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "ID\tDESTINATION\tPROXY\tCOMMENT")
		for _, r := range routes {
			proxyLabel := r.ProxyName
			if proxyLabel == "" {
				proxyLabel = fmt.Sprintf("#%d", r.ProxyID)
			}
			_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", r.ID, r.Destination, proxyLabel, r.Comment)
		}
		_ = tw.Flush()

	case "add":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: madexchanger proxy-route add <destination> <proxy_id> [--comment=TEXT]")
			os.Exit(1)
		}
		proxyID, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid proxy_id: %s\n", args[2])
			os.Exit(1)
		}
		r := &db.ProxyRoute{
			Destination: args[1],
			ProxyID:     proxyID,
		}
		for _, arg := range args[3:] {
			if strings.HasPrefix(arg, "--comment=") {
				r.Comment = strings.TrimPrefix(arg, "--comment=")
			}
		}
		if err := store.AddProxyRoute(r); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("proxy route added: id=%d destination=%s proxy_id=%d\n", r.ID, r.Destination, r.ProxyID)

	case "remove":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: madexchanger proxy-route remove <id>")
			os.Exit(1)
		}
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid id: %s\n", args[1])
			os.Exit(1)
		}
		if err := store.DeleteProxyRoute(id); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("proxy route removed: id=%d\n", id)

	default:
		fmt.Fprintf(os.Stderr, "unknown proxy-route subcommand: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "usage: madexchanger proxy-route [list|add|remove]")
		os.Exit(1)
	}
}

func printCLIUsage() {
	fmt.Fprintln(os.Stderr, `Madexchanger CLI

Proxy Management:
  madexchanger proxy list                              List all proxies
  madexchanger proxy add <name> <type> <host:port>     Add a proxy (type: socks5, http, https)
    [--default] [--username=USER] [--password=PASS] [--comment=TEXT]
  madexchanger proxy remove <id>                       Delete a proxy

Proxy Route Management:
  madexchanger proxy-route list                        List destination→proxy mappings
  madexchanger proxy-route add <dest> <proxy_id>       Route a destination through a proxy
    [--comment=TEXT]
  madexchanger proxy-route remove <id>                 Delete a proxy route

Without subcommands, starts the relay server.`)
}
