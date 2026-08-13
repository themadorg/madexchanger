# Phase B — Allowlist filters

Operator guide: [GUIDE.md](GUIDE.md) section 7 (Phase B).

## Goal

Limit relay traffic with `relay_mode=selected` and domain (or address) filters.

## Lab

PASS: HTTP 403 with no filters; HTTP 200 after adding an allow filter; restore `relay_mode=all`.
