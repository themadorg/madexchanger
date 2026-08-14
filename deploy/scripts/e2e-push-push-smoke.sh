#!/usr/bin/env bash
# Phase A smoke: health + optional isolation + POST via tunnel from Server1.
# Map SSH hosts via env (no hardcoded operator names):
#   SERVER1_SSH  SERVER2_SSH  EXCHANGER_SSH  EXCHANGER_PORT
set -euo pipefail

SERVER1_SSH="${SERVER1_SSH:-server1}"
SERVER2_SSH="${SERVER2_SSH:-server2}"
EXCHANGER_SSH="${EXCHANGER_SSH:-exchanger}"
PORT="${EXCHANGER_PORT:-19080}"
# Backward-compatible aliases
SERVER1_SSH="${ALIREZA_SSH:-$SERVER1_SSH}"
SERVER2_SSH="${DELTA_SSH:-$SERVER2_SSH}"
EXCHANGER_SSH="${ACER_SSH:-$EXCHANGER_SSH}"

echo "== A1/A2 health =="
ssh -o BatchMode=yes -o ConnectTimeout=12 "$EXCHANGER_SSH" "curl -sS --connect-timeout 3 http://127.0.0.1:${PORT}/health"
echo
ssh -o BatchMode=yes -o ConnectTimeout=12 "$SERVER1_SSH" "curl -sS --connect-timeout 3 http://127.0.0.1:${PORT}/health"
echo
ssh -o BatchMode=yes -o ConnectTimeout=12 "$SERVER2_SSH" "curl -sS --connect-timeout 3 http://127.0.0.1:${PORT}/health"
echo

echo "== A3 isolation (optional; 000 if OUTPUT blocked) =="
ssh -o BatchMode=yes -o ConnectTimeout=12 "$SERVER1_SSH" \
  "curl -sk --connect-timeout 2 -o /dev/null -w 'server1->server2_direct=%{http_code}\n' https://\${SERVER2_HTTPS_HOST:-127.0.0.1}/ 2>/dev/null || echo 'server1->server2_direct=skip'"
ssh -o BatchMode=yes -o ConnectTimeout=12 "$SERVER2_SSH" \
  "curl -sk --connect-timeout 2 -o /dev/null -w 'server2->server1_direct=%{http_code}\n' https://\${SERVER1_HTTPS_HOST:-127.0.0.1}/ 2>/dev/null || echo 'server2->server1_direct=skip'"

RCPT="${SMOKE_RCPT_TO:-user@domain1.example}"
FROM="${SMOKE_MAIL_FROM:-user@domain2.example}"

echo "== A4 POST via Server1 tunnel to exchanger =="
ssh -o BatchMode=yes -o ConnectTimeout=15 "$SERVER1_SSH" bash -s <<REMOTE
set -e
code=\$(curl -sS -o /tmp/mxbody -w '%{http_code}' -X POST "http://127.0.0.1:${PORT}/mxdeliv" \
  -H "X-Mail-From: ${FROM}" \
  -H "X-Mail-To: ${RCPT}" \
  -H 'Content-Type: application/octet-stream' \
  --data-binary @- <<'EML'
From: sender@example
To: recipient@example
Subject: phase-a-smoke
MIME-Version: 1.0
Content-Type: multipart/encrypted; protocol="application/pgp-encrypted"; boundary="b"

--b
Content-Type: application/pgp-encrypted

Version: 1

--b
Content-Type: application/octet-stream

-----BEGIN PGP MESSAGE-----
smoke
-----END PGP MESSAGE-----

--b--
EML
)
echo "through_exchanger_http=\$code body=\$(cat /tmp/mxbody)"
REMOTE

echo "== A5 exchanger log tail =="
ssh -o BatchMode=yes -o ConnectTimeout=12 "$EXCHANGER_SSH" \
  "export XDG_RUNTIME_DIR=/run/user/\$(id -u); journalctl --user -u madexchanger --since '2 min ago' --no-pager 2>/dev/null | tail -20 || tail -20 ~/madexchanger/madexchanger.log 2>/dev/null || true"

echo "SMOKE DONE"
