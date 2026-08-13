# Phase A — Harden push–push

## Goals

1. Document the real default (**dynamic** routing; `downstream_url` optional).
2. Ship deploy assets for dual-madmail + reverse-SSH topology.
3. Health-check + tunnel keeper scripts.
4. Verify end-to-end with isolation (direct path blocked).

## Topology under test

```
alireza (external madmail, IP chatmail)
    │ endpoint-cache → http://127.0.0.1:19080/mxdeliv
    │ reverse SSH -R 19080
    ▼
Acer laptop madexchanger :19080  (dynamic route)
    ▲
    │ reverse SSH -R 19080
delta.sudoshz.ir (internal madmail)
    endpoint-cache → http://127.0.0.1:19080/mxdeliv
```

Optional: iptables OUTPUT reject peer public IP to force exchanger path.

## Acceptance tests

| # | Check | Pass criteria |
|---|--------|----------------|
| A1 | `/health` on exchanger | `status=ok` |
| A2 | Tunnel from both madmail hosts | `curl http://127.0.0.1:19080/health` OK |
| A3 | Direct peer HTTPS blocked (if isolation on) | connect fail |
| A4 | Outbound via endpoint-cache | madexchanger log: `email forwarded successfully` |
| A5 | Bidirectional markers | CAS/maildir or federation stats HTTP success |

## Deploy assets

- `deploy/config.push-push.example.yml`
- `deploy/systemd/madexchanger.service`
- `deploy/systemd/madex-tunnels.service` + `.timer`
- `deploy/scripts/keep-tunnels.sh`
- `deploy/scripts/e2e-push-push-smoke.sh`
- `deploy/madmail-endpoint-cache.sh`

## Out of scope (later phases)

- Pull polling, routing UI polish, SOCKS per-destination (B/C).
