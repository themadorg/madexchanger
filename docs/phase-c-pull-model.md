# Phase C — Pull queue

Operator guide: [GUIDE.md](GUIDE.md) section 7 (Phase C).

## Goal

Store messages when push is skipped or fails; deliver later via `GET /pull` and `POST /pull/ack`.

## Config (summary)

```yaml
pull:
  enabled: true
  on_failure: true
  domains:
    - "pull-test.invalid"
  path: "/pull"
  token: "change-me-to-a-random-token"
```

## Lab

PASS: enqueue → list (`count >= 1`) → ack → empty queue.
