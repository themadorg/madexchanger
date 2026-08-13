#!/usr/bin/env bash
# B3 deny + B4 allow via /admin/filters then restore all
set -euo pipefail
TOKEN="${MADEX_ADMIN_TOKEN:?}"
BASE="${MADEX_BASE:-http://127.0.0.1:19080}"
rpc() {
  local method="$1" resource="$2" body="$3"
  curl -sS -X POST "$BASE/api/admin" -H 'Content-Type: application/json' \
    -d "{\"method\":\"$method\",\"resource\":\"$resource\",\"headers\":{\"Authorization\":\"Bearer $TOKEN\"},\"body\":$body}"
}
echo "== set selected =="
rpc POST /admin/config '{"relay_mode":"selected"}'
echo
echo "== B3 no filter (expect 403) =="
code=$(curl -sS -o /tmp/b -w '%{http_code}' -X POST "$BASE/mxdeliv" \
  -H 'X-Mail-From: a@x' -H 'X-Mail-To: u@delta.sudoshz.ir' --data-binary 'x')
echo "code=$code body=$(cat /tmp/b)"
test "$code" = "403"

echo "== add allow filter domain *@delta.sudoshz.ir via domain field =="
# field domain matches recipient domain
out=$(rpc POST /admin/filters '{"enabled":true,"field":"domain","pattern":"delta.sudoshz.ir","comment":"phase-b allow delta"}')
echo "$out"
FID=$(echo "$out" | python3 -c "import sys,json; b=json.load(sys.stdin).get('body') or {}; print(b.get('id',''))")
echo "filter_id=$FID"

echo "== B4 with filter (expect 200) =="
code=$(curl -sS -o /tmp/b -w '%{http_code}' -X POST "$BASE/mxdeliv" \
  -H 'X-Mail-From: smoke@[172.104.234.13]' -H 'X-Mail-To: i5ql7dircw2u@delta.sudoshz.ir' \
  --data-binary $'From: a\r\nTo: b\r\nSubject: b4\r\nMIME-Version: 1.0\r\nContent-Type: multipart/encrypted; protocol="application/pgp-encrypted"; boundary="b"\r\n\r\n--b\r\nContent-Type: application/pgp-encrypted\r\n\r\nVersion: 1\r\n\r\n--b\r\nContent-Type: application/octet-stream\r\n\r\n-----BEGIN PGP MESSAGE-----\r\nz\r\n-----END PGP MESSAGE-----\r\n\r\n--b--\r\n')
echo "code=$code body=$(cat /tmp/b)"
test "$code" = "200"

echo "== cleanup filter + all mode =="
if [[ -n "$FID" ]]; then
  rpc DELETE /admin/filters "{\"id\":$FID}" || true
  echo
fi
rpc POST /admin/config '{"relay_mode":"all"}'
echo
echo "PHASE B OK"
