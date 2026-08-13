# Madexchanger development phases

| Phase | Branch | Goal | Status |
|-------|--------|------|--------|
| **A** | `phase-a/harden-push-push` | Harden **push–push** HTTP relay + deploy/ops | green (lab) |
| **B** | `phase-b/routing-proxy-chain` | Routing rules, rewrite, outbound proxy, multi-hop | in progress |
| **C** | `phase-c/pull-model` | Pull / “do you have mail?” | planned |
| **D** | `phase-d/peer-discovery` | Peer directory / discovery feeds | planned |

## Model reminder

```
Push–Push (Phase A):
  Madmail A ──POST /mxdeliv──► Exchanger ──POST /mxdeliv──► Madmail B

Pull (Phase C — future):
  Exchanger ──poll──► Madmail A
```

See [docs/general.md](general.md) and per-phase docs under `docs/phase-*.md`.
