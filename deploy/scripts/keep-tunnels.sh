#!/usr/bin/env bash
# Keep reverse SSH tunnels: remote 127.0.0.1:19080 → local madexchanger :19080
# Requires SSH config hosts (e.g. alireza-mx, delta-mx) with key auth.
set -euo pipefail

ROOT="${MADEX_HOME:-$HOME/madexchanger}"
mkdir -p "$ROOT"
PORT="${MADEX_PORT:-19080}"

ensure_one() {
  local name="$1"
  local host="$2"
  local pidfile="$ROOT/tunnel-${name}.pid"
  local logfile="$ROOT/tunnel-${name}.log"

  if [[ -f "$pidfile" ]]; then
    local old
    old="$(cat "$pidfile" 2>/dev/null || true)"
    if [[ -n "${old:-}" ]] && kill -0 "$old" 2>/dev/null; then
      return 0
    fi
  fi
  rm -f "$pidfile"
  # -f backgrounds after auth; ExitOnForwardFailure fails fast if port busy
  ssh -f -N \
    -o BatchMode=yes \
    -o ExitOnForwardFailure=yes \
    -o ServerAliveInterval=30 \
    -o ServerAliveCountMax=3 \
    -o StrictHostKeyChecking=accept-new \
    -R "127.0.0.1:${PORT}:127.0.0.1:${PORT}" \
    "$host" >>"$logfile" 2>&1 || {
      echo "WARN: tunnel $name ($host) failed; see $logfile" >&2
      return 0
    }
  # best-effort pid capture
  pgrep -n -f "ssh -f -N .* ${host}" >"$pidfile" 2>/dev/null || true
  echo "tunnel $name ok → $host"
}

# Override with env: MADEX_TUNNEL_HOSTS="name:sshHost name2:sshHost2"
if [[ -n "${MADEX_TUNNEL_HOSTS:-}" ]]; then
  for pair in $MADEX_TUNNEL_HOSTS; do
    ensure_one "${pair%%:*}" "${pair##*:}"
  done
else
  ensure_one alireza alireza-mx
  ensure_one delta delta-mx
fi
