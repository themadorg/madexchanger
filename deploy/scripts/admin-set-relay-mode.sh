#!/usr/bin/env bash
set -euo pipefail
MODE="${1:?mode all|selected}"
TOKEN="${2:-${MADEX_ADMIN_TOKEN:?set MADEX_ADMIN_TOKEN}}"
BASE="${3:-http://127.0.0.1:19080}"
curl -sS -X POST "$BASE/api/admin" \
  -H 'Content-Type: application/json' \
  -d "{\"method\":\"PUT\",\"resource\":\"/admin/config\",\"headers\":{\"Authorization\":\"Bearer ${TOKEN}\"},\"body\":{\"relay_mode\":\"${MODE}\"}}"
echo
