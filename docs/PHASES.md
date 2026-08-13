# Madexchanger development phases

Canonical overview. Full operator guide (Persian, Server1/Server2 naming): **[GUIDE.md](GUIDE.md)**.

| Phase | Branch | Goal | Lab status |
|-------|--------|------|------------|
| **A** | `phase-a/harden-push-push` | Push–push relay + tunnels + endpoint-cache | **PASS** |
| **B** | `phase-b/routing-proxy-chain` | Allowlist filters (`selected` / domain rules) | **PASS** |
| **C** | `phase-c/pull-model` | Pull queue + `/pull` + `/pull/ack` | **PASS** |
| **D** | `phase-d/peer-discovery` | `GET /peers` directory | **PASS** |

**Recommended checkout for full stack:** `phase-c/pull-model` (includes A–D code).

## Models

```
Push–Push (A):  Madmail ──POST /mxdeliv──► Exchanger ──POST /mxdeliv──► Madmail
Allowlist (B):  same path, only matching filters when relay_mode=selected
Pull (C):       store on exchanger; destination GET /pull then POST /pull/ack
Discovery (D):  GET /peers
```

## Verification

See [GUIDE.md §10](GUIDE.md) checklist. Last full three-host lab run: all phases PASS; push–push still green after pull tests.
