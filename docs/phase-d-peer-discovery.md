# Phase D — Peer discovery

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
