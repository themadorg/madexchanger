# Phase C — Pull model

## Behaviour
1. **Always-pull domains** (`pull.domains`): message is stored immediately; no push attempt.
2. **On failure** (`pull.on_failure: true`): if push HTTPS/HTTP fails, store for later pull.
3. Destination agent: `GET /pull?domain=X` with `Authorization: Bearer <token>`, then `POST /pull/ack` with `{"ids":[…]}`.

## Config
```yaml
pull:
  enabled: true
  on_failure: true
  domains:
    - "pull-test.invalid"
  path: "/pull"
  token: ""   # defaults to admin_web.token
```

## Lab tests
| # | Test | Pass |
|---|------|------|
| C1 | POST to always-pull domain | 200, queued_pull++ |
| C2 | GET /pull?domain=… | messages list |
| C3 | POST /pull/ack | count decreases |
