# Phase D — Peer discovery

<<<<<<< HEAD
## Endpoint
`GET /peers` → JSON list of known Madmail peers and this exchanger.

Lab default peers: delta.sudoshz.ir, 172.104.234.13, madexchanger.

## Test
```bash
curl -sS http://127.0.0.1:19080/peers
```

## Future
- Load from `peers.yml` / remote directory URL
- Auth + signed peer lists
- Integration with madmail admin exchanger feeds (poll URLs)
=======
Operator guide: [GUIDE.md](GUIDE.md) section 7 (Phase D).

## Goal

Expose a static peer directory at `GET /peers` for operators and tooling.

## Lab

PASS: JSON with `version` and peers `server1`, `server2`, `exchanger`.
>>>>>>> phase-c/pull-model
