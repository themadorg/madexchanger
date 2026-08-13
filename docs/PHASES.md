# Madexchanger development phases

| Phase | Branch | Goal | Status |
|-------|--------|------|--------|
| **A** | `phase-a/harden-push-push` | Harden push–push + deploy/ops | green |
| **B** | `phase-b/routing-proxy-chain` | Filters / routing / proxy ops | green |
| **C** | `phase-c/pull-model` | Pull queue when push cannot deliver | in progress |
| **D** | `phase-d/peer-discovery` | Peer directory / discovery | planned |

## Models
- **Push–Push:** Madmail → Exchanger → Madmail (`POST /mxdeliv`)
- **Pull:** Destination (or agent) polls `GET /pull?domain=…` for stored mail after push failed or forced pull domain
