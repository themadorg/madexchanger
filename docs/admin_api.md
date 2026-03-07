# Admin API

Madexchanger includes a built-in Admin API accessible through a single RPC-style endpoint.
All requests are `POST /api/admin` with a JSON body. This follows the same design as
[Madmail's Admin API](../../docs/chatmail/admin_api.md).

## Design Principles

1. **Single endpoint** — One path, one POST handler, easier to protect behind firewalls
2. **Bearer token auth** — Token stored in config, passed in inner request headers
3. **No sensitive data in responses** — Passwords and keys are never exposed
4. **JSON-RPC style** — Method, resource, headers, and body in a single request envelope
5. **CORS headers** — Cross-origin access enabled for dashboard deployments

## Authentication

The Admin API uses a Bearer token configured in `config.yml`:

```yaml
admin_web:
  enabled: true
  path: /admin
  token: your-secret-token-here
```

## Request Format

```json
{
    "method": "GET|POST|PUT|DELETE",
    "resource": "/admin/stats",
    "headers": {
        "Authorization": "Bearer your-secret-token-here"
    },
    "body": {}
}
```

## Response Format

```json
{
    "status": 200,
    "resource": "/admin/stats",
    "body": { ... },
    "error": null
}
```

## Available Resources

### `/admin/stats` — Relay Statistics
- **GET**: Returns aggregate relay statistics

Example:
```bash
curl -X POST https://your-relay:8443/api/admin \
  -H 'Content-Type: application/json' \
  -d '{
    "method": "GET",
    "resource": "/admin/stats",
    "headers": {"Authorization": "Bearer YOUR_TOKEN"}
  }'
```

Response:
```json
{
    "status": 200,
    "resource": "/admin/stats",
    "body": {
        "total_relayed": 1547,
        "total_rejected": 23,
        "total_errors": 5,
        "total_bytes": 52428800
    }
}
```

### `/admin/messages` — Message Log
- **GET**: Returns recent message records (body: `{"limit": N}`, max 500, default 50)

Response:
```json
{
    "status": 200,
    "resource": "/admin/messages",
    "body": [
        {
            "id": 42,
            "timestamp": "2026-03-07T15:00:00Z",
            "mail_from": "alice@example.org",
            "mail_to": "bob@other.org",
            "size_bytes": 4096,
            "status": "ok",
            "error_message": "",
            "remote_addr": "192.168.1.100:44321",
            "downstream": "https://10.0.0.5/mxdeliv"
        }
    ]
}
```

### `/admin/config` — Relay Configuration
- **GET**: Returns current relay configuration
- **POST**: Update relay mode — `{"relay_mode": "all"}` or `{"relay_mode": "selected"}`

Example — switch to selected mode:
```json
{"method": "POST", "resource": "/admin/config",
 "headers": {"Authorization": "Bearer TOKEN"},
 "body": {"relay_mode": "selected"}}
```

Response:
```json
{"status": 200, "body": {"status": "ok", "relay_mode": "selected"}}
```

### `/admin/rewrites` — Endpoint Rewrite Rules
Rewrite sender, recipient, or downstream URL before forwarding.

- **GET**: List all rewrite rules
- **POST**: Add a rewrite rule
- **PUT**: Update a rewrite rule (body must include `"id"`)
- **DELETE**: Delete a rewrite rule — `{"id": N}`

Fields: `mail_from`, `mail_to`, `downstream`

Example — add a rewrite rule:
```json
{"method": "POST", "resource": "/admin/rewrites",
 "headers": {"Authorization": "Bearer TOKEN"},
 "body": {"enabled": true, "field": "mail_from", "pattern": "old@x.org", "replacement": "new@x.org", "comment": "migrate"}}
```

Example — delete a rewrite rule:
```json
{"method": "DELETE", "resource": "/admin/rewrites",
 "headers": {"Authorization": "Bearer TOKEN"},
 "body": {"id": 3}}
```

### `/admin/filters` — Relay Filters
Used in "Relay Selected" mode to control which messages are relayed.

- **GET**: List all relay filters
- **POST**: Add a relay filter
- **PUT**: Update a relay filter (body must include `"id"`)
- **DELETE**: Delete a relay filter — `{"id": N}`

Fields: `mail_from`, `mail_to`, `domain`

Example — add a domain filter:
```json
{"method": "POST", "resource": "/admin/filters",
 "headers": {"Authorization": "Bearer TOKEN"},
 "body": {"enabled": true, "field": "domain", "pattern": "example.org", "comment": "allow example.org"}}
```

## Web Admin Panel

All Admin API resources are also accessible through a built-in web interface at the configured admin path (default: `/admin/`). The panel uses the same authentication token and provides:

| Tab | Features |
|-----|----------|
| **Overview** | Relay stats (relayed, rejected, errors, bytes), relay mode toggle, config display, recent message log |
| **Rewrites** | View, add, toggle, and delete endpoint rewrite rules |
| **Filters** | View, add, toggle, and delete relay filters (with mode awareness) |

The panel supports both **light and dark mode** via a toggle in the header.

## Security Considerations

- The admin token is a shared secret — treat it like a password
- Never expose the admin token in logs or version control
- Use HTTPS in production to protect the token in transit
- Consider firewalling the `/api/admin` endpoint to trusted networks only
