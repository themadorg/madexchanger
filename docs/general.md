# Madexchanger — General Architecture

## What Is an Exchanger?

An exchanger is an **email relay** — think of it as a router for messages.
Its job is simple: receive a message, figure out where it needs to go, and send it there.

In the Madmail ecosystem, exchangers sit between mail servers and relay email
over HTTP/HTTPS, the same way network routers sit between networks and
forward packets.

```
   Madmail A ──► Exchanger ──► Madmail B
                    │
                    ├──► Madmail C
                    └──► Madmail D
```

Just like a network router, one exchanger can fan out to many destinations,
and multiple exchangers can be chained together to form relay paths through
complex network topologies.


## Push-Based Exchanger (Current)

The current Madexchanger implementation is **push-based**. This means:

1. An upstream server **pushes** a message to the exchanger via HTTP POST.
2. The exchanger processes it and **pushes** it onward to the destination.

All communication uses the same wire format and endpoint: `POST /mxdeliv`.

### How It Works

```
┌──────────────┐   POST /mxdeliv    ┌────────────────┐   POST /mxdeliv    ┌──────────────┐
│  Upstream     │ ────────────────► │  Madexchanger   │ ────────────────► │  Destination   │
│  (Madmail)    │   X-Mail-From     │                 │   X-Mail-From     │  (Madmail)     │
│               │   X-Mail-To       │                 │   X-Mail-To       │                │
│               │   <RFC 822 body>  │                 │   <RFC 822 body>  │                │
└──────────────┘                    └────────────────┘                    └──────────────┘
```

The wire format is identical on both sides — the exchanger speaks the exact
same protocol that Madmail servers use to deliver email to each other. This
means any Madmail-compatible server can talk to an exchanger directly.

### Wire Format

```
POST /mxdeliv HTTP/1.1
X-Mail-From: sender@example.org
X-Mail-To: recipient1@example.org
X-Mail-To: recipient2@example.org
Content-Type: application/octet-stream

<RFC 822 message: headers + body>
```

- **`X-Mail-From`** — envelope sender (MAIL FROM)
- **`X-Mail-To`** — envelope recipients (RCPT TO), one header per recipient
- **Body** — the complete RFC 822 email message (headers + body)

### Setting Up the Relay

To route email through a Madexchanger, you configure the upstream Madmail
server to rewrite its delivery endpoint to point at the exchanger instead of
delivering directly:

1. On the **Madmail server** (upstream), add an endpoint rewrite that
   redirects outbound delivery for certain domains to the exchanger's
   `/mxdeliv` path.
2. The **exchanger** receives those messages and forwards them onward.
3. The **destination** receives the message as if it came from a normal
   Madmail server.

That's it. The exchanger is transparent — from the destination's perspective,
the message looks like it came directly from the original sender.


## Routing Modes

The exchanger supports two routing modes for determining where to send messages:

### Dynamic Routing (Default)

When no fixed downstream is configured, the exchanger routes **dynamically**
based on the recipient's email domain:

1. Extract the domain from `X-Mail-To` (e.g., `user@2.2.2.2` → `2.2.2.2`)
2. Try HTTPS first: `POST https://2.2.2.2/mxdeliv`
3. If HTTPS fails, fall back to HTTP: `POST http://2.2.2.2/mxdeliv`

This matches exactly how Madmail's own delivery logic works.

### Static Routing

When `downstream_url` is set in the configuration, **all** messages are
forwarded to that fixed URL regardless of the recipient domain. This is useful
for chaining exchangers or funneling all traffic through a single next-hop.


## Incoming Allow List

The incoming allow list controls **which messages are accepted** by the
exchanger. It operates in two modes:

- **All** (`incoming_mode: all`) — Accept every incoming message. This is
  the default.
- **Selected** (`incoming_mode: selected`) — Only accept messages that
  match at least one enabled incoming rule. Everything else is rejected
  with HTTP 403.

Incoming rules can match on:

| Field | Description |
|-------|------------|
| `mail_from` | Envelope sender address |
| `mail_to` | Envelope recipient address |
| `domain` | Sender or recipient domain |
| `remote_addr` | IP address of the sending server |

Patterns support exact match and wildcard (`*`) prefix/suffix matching.


## Outgoing Allow List

The outgoing allow list controls **which destinations the exchanger is
allowed to deliver to**. It operates in two modes:

- **All** (`outgoing_mode: all`) — Deliver to any destination. This is
  the default.
- **Selected** (`outgoing_mode: selected`) — Only deliver to destinations
  matching at least one enabled outgoing rule. Delivery to unmatched
  destinations is blocked.

Outgoing rules can match on:

| Field | Description |
|-------|------------|
| `domain` | Destination domain or IP |
| `mail_to` | Recipient address pattern |

This is useful for restricting the exchanger to only forward email to a
known set of servers.


## Routing

Routing rules let you **redirect incoming messages to a different
destination**. When an incoming message matches a routing rule's pattern,
the exchanger delivers it to the rule's destination URL instead of the
original target.

Each routing rule has:

- **Pattern** — matches the recipient address or domain
- **Destination** — the override URL (e.g., `https://10.0.0.5:8443`)

Example: a routing rule with pattern `*@example.org` and destination
`https://backup.example.org` would redirect all email addressed to
`example.org` to the backup server.


## Outbound Proxies

When the exchanger needs to reach a destination through a proxy (for example,
to bypass network restrictions or route through a VPN), you can configure
**outbound proxies**.

Proxies are defined individually and then mapped to specific destinations
using **proxy routes**:

1. **Define a proxy** — Create a proxy record specifying the type
   (`socks5`, `http`, or `https`), the host and port, and optional
   credentials.

2. **Create proxy routes** — Map destination patterns to proxies. For
   example, route `1.1.1.1` through a specific SOCKS5 proxy, or route
   `*.example.org` through an HTTP proxy.

When the exchanger sends a message to a destination, it checks if any proxy
route matches that destination. If a match is found, the message is delivered
through the configured proxy. If no route matches, the message is sent
directly.

```
                                    ┌───────────┐
                               ┌──► │ Proxy A   │ ──► 1.1.1.1
┌──────────────┐               │    └───────────┘
│ Exchanger    │ ──── route ───┤
│              │               │    (no route)
└──────────────┘               └────────────────────► 2.2.2.2 (direct)
```

There is no "default proxy" — every proxy must be explicitly mapped to the
destinations that should use it. Unmapped destinations always use direct
connections.


## Pull-Based Exchanger (Future)

A second exchanger variant is planned: the **pull-based exchanger**. Instead
of receiving pushed messages, this exchanger would actively **pull** messages
from upstream servers.

The pull model is useful for environments where the upstream server cannot
initiate outbound connections (e.g., behind strict firewalls) or where
periodic batch retrieval is more appropriate than real-time push.

The pull-based exchanger specification is **not yet defined** and is reserved
for future development.


## Admin Dashboard

The exchanger includes an embedded web-based admin dashboard for monitoring
and management. It provides:

- **Overview** — Real-time relay flow diagram, incoming/outgoing mode
  toggles, and aggregate statistics (relayed, rejected, errors, total data).
- **Incoming** — Manage the incoming allow list rules that control which
  messages are accepted.
- **Outgoing** — Manage the outgoing allow list rules that control which
  destinations are allowed.
- **Routing** — Manage routing rules that redirect messages to different
  destinations.
- **Proxies** — Configure outbound proxies and destination-to-proxy route
  mappings.

The dashboard is served as an embedded SPA at a configurable path
(default `/admin`) and is protected by a Bearer token.


## Use Cases

- **Network Bridging** — Relay email between servers separated by
  NAT, firewalls, or different network segments.
- **Split DNS** — Route email through an intermediary when direct
  delivery is not possible.
- **Proxy Routing** — Deliver email through SOCKS5 or HTTP(S) proxies
  to reach destinations behind restricted networks.
- **Access Control** — Restrict which incoming messages are accepted
  and which outbound destinations are allowed.
- **Destination Override** — Redirect messages to alternative servers
  using routing rules.
- **Testing** — Intercept and inspect email flow between Madmail instances.
- **Chaining** — Connect multiple exchangers in series for complex
  routing topologies.
