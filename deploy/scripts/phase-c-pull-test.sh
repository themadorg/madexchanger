#!/usr/bin/env bash
set -euo pipefail
TOKEN="${MADEX_ADMIN_TOKEN:?}"
BASE="${MADEX_BASE:-http://127.0.0.1:19080}"
DOM="pull-test.invalid"
MARKER="PULL-$(date +%s)"

echo "== C1 enqueue always-pull domain =="
code=$(curl -sS -o /tmp/c1 -w '%{http_code}' -X POST "$BASE/mxdeliv" \
  -H "X-Mail-From: a@src.test" -H "X-Mail-To: u@${DOM}" \
  --data-binary "From: a@src.test
To: u@${DOM}
Subject: ${MARKER}

body ${MARKER}
")
echo "code=$code"
test "$code" = "200"
curl -sS "$BASE/health"; echo

echo "== C2 pull list =="
list=$(curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/pull?domain=${DOM}")
echo "$list" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['count']>=1; print('count', d['count']); print('ids', [m['id'] for m in d['messages']]); open('/tmp/pull_ids.json','w').write(json.dumps([m['id'] for m in d['messages']]))"

echo "== C3 ack =="
IDS=$(cat /tmp/pull_ids.json)
curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"ids\":$IDS}" "$BASE/pull/ack"
echo
list2=$(curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/pull?domain=${DOM}")
echo "$list2" | python3 -c "import sys,json; d=json.load(sys.stdin); print('remaining', d['count']); assert d['count']==0"
echo "PHASE C OK"
