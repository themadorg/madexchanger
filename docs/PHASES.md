# Development phases

Full operator documentation: [GUIDE.md](GUIDE.md).

| Phase | Branch | Goal | Lab |
|-------|--------|------|-----|
| **A** | `phase-a/harden-push-push` | Push–push relay, tunnels, endpoint-cache | PASS |
| **B** | `phase-b/routing-proxy-chain` | Allowlist filters | PASS |
| **C** | `phase-c/pull-model` | Pull queue and `/pull` API | PASS |
| **D** | `phase-d/peer-discovery` | `GET /peers` | PASS |

Checkout for the complete stack: **`phase-c/pull-model`**.

## Models

```
Push–push (A):  Madmail ──POST /mxdeliv──► Exchanger ──POST /mxdeliv──► Madmail
Allowlist (B):  same path; only matching filters when relay_mode=selected
Pull (C):       store on exchanger; client GET /pull then POST /pull/ack
Discovery (D):  GET /peers
```

## Verification

Use the checklist in [GUIDE.md](GUIDE.md) section 10. Integrated three-host lab: A–D all PASS; push–push still green after filter and pull tests.
