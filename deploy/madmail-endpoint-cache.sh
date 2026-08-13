#!/usr/bin/env bash
# Configure madmail endpoint-cache → local madexchanger tunnel.
# Usage on each Madmail host:
#   PEER_DOMAIN=domain2.example ./madmail-endpoint-cache.sh
#   PEER_DOMAIN=203.0.113.50 PEER_IP_LITERAL=1 ./madmail-endpoint-cache.sh
set -euo pipefail

PORT="${EXCHANGER_PORT:-19080}"
TARGET="http://127.0.0.1:${PORT}/mxdeliv"
PEER_DOMAIN="${PEER_DOMAIN:?set PEER_DOMAIN to the remote mail domain or IP}"

madmail endpoint-cache set "$PEER_DOMAIN" "$TARGET" "via madexchanger reverse tunnel"
if [[ -n "${PEER_IP_LITERAL:-}" ]] || [[ "$PEER_DOMAIN" =~ ^[0-9.]+$ ]]; then
  bare="${PEER_DOMAIN#[}"
  bare="${bare%]}"
  madmail endpoint-cache set "[${bare}]" "$TARGET" "bracket form"
fi
madmail endpoint-cache list
