# Madexchanger development phases

Roadmap for hardening and extending the exchanger used between
restricted / multi-network Madmail deployments.

| Phase | Branch | Goal | Status |
|-------|--------|------|--------|
| **A** | `phase-a/harden-push-push` | Production-harden current **push–push** HTTP relay | in progress |
| **B** | `phase-b/routing-proxy-chain` | Routing rules, rewrite, outbound proxy, multi-hop docs + ops | planned |
| **C** | `phase-c/pull-model` | Pull / “do you have mail?” (when upstream cannot push) | planned |
| **D** | `phase-d/peer-discovery` | Peer directory / discovery feeds (not message relay) | planned |

## Model reminder

```
Push–Push (Phase A — current product):
  Madmail A ──POST /mxdeliv──► Exchanger ──POST /mxdeliv──► Madmail B

Pull (Phase C — future):
  Exchanger ──poll /exchanger/pull──► Madmail A  (fetch queued outbound)
```

See [docs/general.md](general.md) for architecture detail.
