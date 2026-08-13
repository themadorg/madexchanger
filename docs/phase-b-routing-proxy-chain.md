# Phase B — Routing, rewrite, proxy, multi-hop

## Goals
1. Exercise **dynamic vs static** downstream.
2. Document **relay_mode selected** filters (allowlist).
3. Document **outbound proxy** + multi-hop chain patterns.
4. Provide admin RPC helpers for lab configuration.

## Features already in madexchanger product
| Feature | Config / Admin |
|---------|----------------|
| Dynamic routing | `downstream_url: ""` |
| Static next hop | `downstream_url: "https://…"` |
| Relay filters | `relay_mode: selected` + admin filters |
| Envelope rewrite | admin rewrite rules |
| Routing override | admin routing rules → fixed destination URL |
| Outbound proxy | `proxy.url` or admin proxy routes |

## Lab tests
| # | Test | Pass |
|---|------|------|
| B1 | Dynamic still works | Phase A smoke green |
| B3 | selected + no filters | HTTP 403 |
| B4 | selected + allow filter | HTTP 200 (when filter API seeded) |

## Scripts
- `deploy/scripts/admin-set-relay-mode.sh` (needs `MADEX_ADMIN_TOKEN`)
