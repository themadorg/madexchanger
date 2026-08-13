#!/usr/bin/env bash
# Configure madmail endpoint-cache to push via local madexchanger tunnel.
# Usage:
#   ./madmail-endpoint-cache.sh alireza-side    # on external IP server
#   ./madmail-endpoint-cache.sh delta-side      # on internal DNS server
set -euo pipefail

PORT="${EXCHANGER_PORT:-19080}"
TARGET="http://127.0.0.1:${PORT}/mxdeliv"

case "${1:-}" in
  alireza-side|external)
    madmail endpoint-cache set delta.sudoshz.ir "$TARGET" "via madexchanger reverse tunnel"
    madmail endpoint-cache list
    ;;
  delta-side|internal)
    madmail endpoint-cache set 172.104.234.13 "$TARGET" "via madexchanger reverse tunnel"
    madmail endpoint-cache set "[172.104.234.13]" "$TARGET" "bracket form"
    madmail endpoint-cache list
    ;;
  *)
    echo "Usage: $0 alireza-side|delta-side" >&2
    exit 1
    ;;
esac
