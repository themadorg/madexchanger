# Phase C — Pull queue

See [GUIDE.md](GUIDE.md) §7 phase C.

## Goal
Queue messages for domains that cannot be pushed; deliver via `GET /pull` + `POST /pull/ack`.

## Lab status
**PASS** — enqueue / list / ack / empty queue.
