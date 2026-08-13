#!/usr/bin/env bash
# Set relay_mode via madexchanger admin RPC (method must be POST, not PUT).
# Usage: MADEX_ADMIN_TOKEN=secret ./admin-set-relay-mode.sh all|selected [base_url]
set -euo pipefail
MODE="${1:?mode all|selected}"
TOKEN="${MADEX_ADMIN_TOKEN:?set MADEX_ADMIN_TOKEN}"
BASE="${2:-http://127.0.0.1:19080}"
curl -sS -X POST "$BASE/api/admin" \
  -H 'Content-Type: application/json' \
  -d "{\"method\":\"POST\",\"resource\":\"/admin/config\",\"headers\":{\"Authorization\":\"Bearer ${TOKEN}\"},\"body\":{\"relay_mode\":\"${MODE}\"}}"
echo
