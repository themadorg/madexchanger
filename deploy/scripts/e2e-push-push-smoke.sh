#!/usr/bin/env bash
# Smoke-test push–push path. Run from a host that can SSH to madmail + exchanger.
# Env:
#   ALIREZA_SSH=alireza  DELTA_SSH=delta  ACER_SSH=acer
#   EXCHANGER_PORT=19080
set -euo pipefail

ALIREZA_SSH="${ALIREZA_SSH:-alireza}"
DELTA_SSH="${DELTA_SSH:-delta}"
ACER_SSH="${ACER_SSH:-acer}"
PORT="${EXCHANGER_PORT:-19080}"
MARKER="SMOKE-$(date +%s)"

echo "== A1/A2 health =="
ssh -o BatchMode=yes -o ConnectTimeout=12 "$ACER_SSH" "curl -sS --connect-timeout 3 http://127.0.0.1:${PORT}/health"
echo
ssh -o BatchMode=yes -o ConnectTimeout=12 "$ALIREZA_SSH" "curl -sS --connect-timeout 3 http://127.0.0.1:${PORT}/health"
echo
ssh -o BatchMode=yes -o ConnectTimeout=12 "$DELTA_SSH" "curl -sS --connect-timeout 3 http://127.0.0.1:${PORT}/health"
echo

echo "== A3 isolation (optional; 000 expected if blocked) =="
ssh -o BatchMode=yes -o ConnectTimeout=12 "$ALIREZA_SSH" \
  "curl -sk --connect-timeout 2 -o /dev/null -w 'alireza->delta direct=%{http_code}\n' https://delta.sudoshz.ir/ || true"
ssh -o BatchMode=yes -o ConnectTimeout=12 "$DELTA_SSH" \
  "curl -sk --connect-timeout 2 -o /dev/null -w 'delta->alireza direct=%{http_code}\n' https://172.104.234.13/ || true"

echo "== A4 POST via tunnel (alireza localhost exchanger) =="
ssh -o BatchMode=yes -o ConnectTimeout=15 "$ALIREZA_SSH" bash -s <<REMOTE
set -e
code=\$(curl -sS -o /tmp/mxbody -w '%{http_code}' -X POST "http://127.0.0.1:${PORT}/mxdeliv" \
  -H 'X-Mail-From: smoke@[172.104.234.13]' \
  -H 'X-Mail-To: i5ql7dircw2u@delta.sudoshz.ir' \
  -H 'Content-Type: application/octet-stream' \
  --data-binary @- <<EML
From: smoke@[172.104.234.13]
To: i5ql7dircw2u@delta.sudoshz.ir
Subject: ${MARKER}
MIME-Version: 1.0
Content-Type: multipart/encrypted; protocol="application/pgp-encrypted"; boundary="b"

--b
Content-Type: application/pgp-encrypted

Version: 1

--b
Content-Type: application/octet-stream

-----BEGIN PGP MESSAGE-----
${MARKER}
-----END PGP MESSAGE-----

--b--
EML
)
echo "through_exchanger_http=\$code body=\$(cat /tmp/mxbody)"
REMOTE

echo "== A5 acer recent forward log =="
ssh -o BatchMode=yes -o ConnectTimeout=12 "$ACER_SSH" \
  "export XDG_RUNTIME_DIR=/run/user/\$(id -u); journalctl --user -u madexchanger --since '2 min ago' --no-pager 2>/dev/null | tail -20 || tail -20 ~/madexchanger/madexchanger.log 2>/dev/null || true"

echo "SMOKE DONE marker=${MARKER}"
