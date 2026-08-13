# Madexchanger

HTTP/HTTPS email relay proxy for the [Madmail](https://github.com/themadorg/madmail) ecosystem.

## Overview

Madexchanger sits between two Madmail-compatible servers and relays email messages over HTTP/HTTPS. It receives emails via POST requests on a configurable path (default `/mxdeliv`) and forwards them to a downstream server using the same wire format.

```
┌──────────────┐    POST /mxdeliv     ┌────────────────┐    POST /mxdeliv     ┌──────────────┐
│  Upstream     │ ──────────────────► │  Madexchanger   │ ──────────────────► │  Downstream   │
│  (Madmail)    │   X-Mail-From       │  (this server)  │   X-Mail-From       │  (Madmail)    │
│              │   X-Mail-To          │                │   X-Mail-To          │              │
│              │   <RFC 822 body>     │                │   <RFC 822 body>     │              │
└──────────────┘                      └────────────────┘                      └──────────────┘
```

### Use Cases

- **Network bridging**: Relay email between servers separated by NAT/firewalls
- **Split DNS**: Route email through an intermediary when direct delivery is not possible
- **Testing**: Intercept and inspect email flow between Madmail instances
- **Load distribution**: Front multiple downstream servers (future)

## Wire Format

Madexchanger uses the same HTTP wire format as Madmail's `target.remote` and chatmail endpoint:

```
POST /mxdeliv HTTP/1.1
X-Mail-From: sender@example.org
X-Mail-To: recipient1@example.org
X-Mail-To: recipient2@example.org
Content-Type: application/octet-stream

<RFC 822 message headers + body>
```

- `X-Mail-From`: envelope sender (MAIL FROM)
- `X-Mail-To`: envelope recipients (RCPT TO), one header per recipient
- Body: complete RFC 822 message (headers + body)

## Quick Start

```bash
# Build
make build

# Copy and edit configuration (dynamic routing is the default)
cp config.yml.example config.yml
# Or: cp deploy/config.push-push.example.yml config.yml

# Run
./madexchanger -config config.yml
```

Push–push lab (reverse tunnels + dual Madmail): `docs/phase-a-harden-push-push.md`.


## Configuration

See [`config.yml.example`](config.yml.example) for a fully documented configuration reference.

| Setting | Default | Description |
|---------|---------|-------------|
| `listen` | `0.0.0.0:8443` | Listen address and port |
| `receive_path` | `/mxdeliv` | Inbound POST path |
| `downstream_url` | *(empty = dynamic)* | Fixed next hop; empty routes by recipient domain |
| `forward_timeout` | `30` | Request timeout (seconds) |
| `skip_tls_verify` | `true` | Skip downstream TLS verification |
| `max_body_size` | `33554432` | Max request body (32 MiB) |
| `relay_mode` | `all` | `all` or `selected` (filter rules) |
| `log_level` | `info` | Log level: debug/info/warn/error |
| `tls.cert_file` | *(empty)* | TLS certificate for inbound HTTPS |
| `tls.key_file` | *(empty)* | TLS private key for inbound HTTPS |

**Routing:** by default Madexchanger is **push–push dynamic**: it POSTs to `https://<recipient-domain>/mxdeliv` (HTTP fallback). Use `downstream_url` only for a fixed next hop (chaining).

**Deploy / multi-server lab:** see [`docs/PHASES.md`](docs/PHASES.md) and [`deploy/`](deploy/).

## Building

```bash
# Development build
make build

# Production build with version info
make build VERSION=1.0.0

# Run tests
make test

# Lint (requires golangci-lint)
make lint

# Clean build artifacts
make clean
```

## Health Check

The server exposes a health endpoint at `/health`:

```bash
curl http://localhost:8443/health
# {"status":"ok","received":42,"forwarded":42,"errors":0}
```

## License

GNU General Public License v3.0 — see [COPYING](COPYING) for details.
