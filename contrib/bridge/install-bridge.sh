#!/usr/bin/env bash
# Remote command strings are assembled locally and expanded on the relay on
# purpose ($S = optional "sudo -n"), so single-quoted "$" and client-side
# string building are intended throughout this file.
# shellcheck disable=SC2016,SC2029
#
# =============================================================================
# install-bridge.sh - one-command Madexchanger bridge for Delta Chat relays
# =============================================================================
#
# Purpose
#   During an international internet shutdown a domestic (IR) Madmail relay and
#   a foreign Madmail relay can no longer reach each other. A "bridge device"
#   (a Linux laptop or mini-PC that can still reach BOTH relays) runs
#   madexchanger (https://github.com/themadorg/madexchanger) and carries the
#   relays' /mxdeliv traffic between them.
#
# What this script does (install action)
#   1. Collects the relay addresses (interactive prompts or flags)
#   2. Checks local tools and the SSH key, verifies SSH + madmail on both relays
#   3. Builds/installs madexchanger and writes config.yml with a random token
#   4. Starts the exchanger and the reverse SSH tunnels (systemd --user, or a
#      nohup fallback) with a PID-file based tunnel keeper
#   5. Points each relay at the bridge with `madmail endpoint-cache set`
#   6. Optionally simulates the outage with tagged iptables rules
#   7. Runs a health check and prints a summary
#
# Usage
#   bash <(curl -fsSL https://raw.githubusercontent.com/themadorg/madexchanger/main/contrib/bridge/install-bridge.sh)            # interactive
#   ./install-bridge.sh --yes --ir-host root@ir.example --foreign-host root@203.0.113.50
#   ./install-bridge.sh --status | --restore [--purge] | --dry-run | --help
#
# Safety
#   - --dry-run prints every local and remote command and changes nothing
#   - The admin token is written only to config.yml (mode 600), never printed
#   - --restore undoes everything this script changed on the relays
# =============================================================================
set -euo pipefail

BRIDGE_VERSION="0.2.0"

# -----------------------------------------------------------------------------
# Defaults. Paths can be overridden with environment variables (used by tests).
# -----------------------------------------------------------------------------
REPO_URL="${MADEX_REPO_URL:-https://github.com/themadorg/madexchanger}"
MADEX_HOME="${MADEX_HOME:-$HOME/madexchanger}"          # binary, config, logs
STATE_DIR="${MADEX_STATE_DIR:-$HOME/.config/madexchanger-bridge}"
STATE_FILE="$STATE_DIR/state.env"                       # what we changed, for --restore
LOG_FILE="${MADEX_LOG:-$STATE_DIR/install.log}"

ACTION="install"          # install | status | restore
IR_HOST="" ; FOREIGN_HOST=""          # SSH targets (user@host allowed)
IR_DOMAIN="" ; FOREIGN_DOMAIN=""      # mail domains (or IP) of each relay
SSH_USER="root" ; SSH_PORT="22" ; SSH_KEY=""
SSH_EXTRA="${MADEX_SSH_OPTS:-}"       # extra raw ssh options (space separated)
PORT="19080"              # exchanger port (also the tunnel port on each relay)
MODE="tunnel"             # tunnel | direct
BRIDGE_ADDR=""            # direct mode: address the relays use to reach the bridge
INSTALL_METHOD="go"       # go | docker | binary
BINARY_SRC=""             # binary method: local path or URL
REBUILD=0                 # 1 = rebuild even if a binary already exists
SIMULATE=0                # 1 = add iptables outage simulation
SERVICE_MGR="auto"        # auto | systemd | nohup
ASSUME_YES=0              # 1 = never prompt
DRY_RUN=0                 # 1 = print commands only
PURGE=0                   # 1 = with --restore, also delete local files
IR_IP="" ; FOREIGN_IP=""  # outage simulation: addresses to block (auto-resolved)

# -----------------------------------------------------------------------------
# Output helpers. Colors only when stdout is a terminal.
# -----------------------------------------------------------------------------
if [ -t 1 ]; then C_G=$'\e[32m'; C_Y=$'\e[33m'; C_R=$'\e[31m'; C_B=$'\e[1m'; C_0=$'\e[0m'
else C_G=""; C_Y=""; C_R=""; C_B=""; C_0=""; fi
say()  { printf '%s\n' "${C_B}==>${C_0} $*"; }            # section header
ok()   { printf '%s\n' "${C_G}  [ok]${C_0} $*"; }         # success line
warn() { printf '%s\n' "${C_Y}  [warn]${C_0} $*" >&2; }   # non-fatal problem
die()  { printf '%s\n' "${C_R}  [error]${C_0} $*" >&2; exit 1; }  # fatal

usage() {
cat <<'EOF'
Madexchanger bridge installer - keeps Delta Chat relays connected during shutdowns

Usage: install-bridge.sh [options]

Actions (default: install)
  --status                 show bridge, tunnels and relay health
  --restore, --uninstall   remove endpoint-cache entries and outage rules, stop services
  --purge                  with --restore: also delete ~/madexchanger and state
  -h, --help               this help
  --version                print the installer version

Relays
  --ir-host HOST           SSH host of the domestic relay (user@host allowed)
  --foreign-host HOST      SSH host of the foreign relay (user@host allowed)
  --ir-domain DOMAIN       mail domain of the domestic relay   (default: ir host)
  --foreign-domain DOMAIN  mail domain/IP of the foreign relay (default: foreign host)
  --foreign-key DOMAIN     alias of --foreign-domain
  --ssh-user USER          default SSH user (default: root)
  --ssh-port PORT          SSH port (default: 22)
  --ssh-key FILE           private key to use (default: ~/.ssh/id_ed25519, created if missing)

Bridge
  --port PORT              exchanger port (default: 19080)
  --mode tunnel|direct     tunnel: reverse SSH tunnels (default, works behind NAT)
                           direct: relays reach the bridge at --bridge-addr
  --bridge-addr ADDR       direct mode only: IP/host the relays use to reach this device
  --install-method go|docker|binary   how to get madexchanger (default: go)
  --binary PATH|URL        binary method: prebuilt madexchanger
  --rebuild                rebuild/reinstall even if a binary exists
  --service-manager auto|systemd|nohup   (default: auto)

Testing
  --simulate-outage        block direct relay<->relay traffic with iptables (undo with --restore)
  --ir-ip IP / --foreign-ip IP   addresses to block (default: resolved on the other relay)
  --dry-run                print every local and remote command, change nothing
  -y, --yes                no questions, use flags/defaults

Env: MADEX_HOME, MADEX_STATE_DIR, MADEX_LOG, MADEX_REPO_URL, MADEX_SSH_OPTS
EOF
}

# -----------------------------------------------------------------------------
# Argument parsing. Every option that takes a value is checked by need_arg.
# -----------------------------------------------------------------------------
need_arg() { if [ "$#" -lt 2 ] || [ -z "$2" ]; then die "option $1 needs a value"; fi; }
while [ "$#" -gt 0 ]; do
  case "$1" in
    --ir-host) need_arg "$@"; IR_HOST="$2"; shift 2 ;;
    --foreign-host) need_arg "$@"; FOREIGN_HOST="$2"; shift 2 ;;
    --ir-domain) need_arg "$@"; IR_DOMAIN="$2"; shift 2 ;;
    --foreign-domain|--foreign-key) need_arg "$@"; FOREIGN_DOMAIN="$2"; shift 2 ;;
    --ssh-user) need_arg "$@"; SSH_USER="$2"; shift 2 ;;
    --ssh-port) need_arg "$@"; SSH_PORT="$2"; shift 2 ;;
    --ssh-key) need_arg "$@"; SSH_KEY="$2"; shift 2 ;;
    --ir-ip) need_arg "$@"; IR_IP="$2"; shift 2 ;;
    --foreign-ip) need_arg "$@"; FOREIGN_IP="$2"; shift 2 ;;
    --port) need_arg "$@"; PORT="$2"; shift 2 ;;
    --mode) need_arg "$@"; MODE="$2"; shift 2 ;;
    --bridge-addr) need_arg "$@"; BRIDGE_ADDR="$2"; shift 2 ;;
    --install-method) need_arg "$@"; INSTALL_METHOD="$2"; shift 2 ;;
    --binary) need_arg "$@"; BINARY_SRC="$2"; INSTALL_METHOD="binary"; shift 2 ;;
    --rebuild) REBUILD=1; shift ;;
    --service-manager) need_arg "$@"; SERVICE_MGR="$2"; shift 2 ;;
    --simulate-outage) SIMULATE=1; shift ;;
    --status) ACTION="status"; shift ;;
    --restore|--uninstall) ACTION="restore"; shift ;;
    --purge) PURGE=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    -y|--yes) ASSUME_YES=1; shift ;;
    -h|--help) usage; exit 0 ;;
    --version) echo "$BRIDGE_VERSION"; exit 0 ;;
    *) die "unknown option: $1 (see --help)" ;;
  esac
done

# -----------------------------------------------------------------------------
# Logging: everything printed from here on is also appended to LOG_FILE.
# Prompts are written to /dev/tty, so they never end up in the log.
# -----------------------------------------------------------------------------
mkdir -p "$STATE_DIR"; chmod 700 "$STATE_DIR"
exec > >(tee -a "$LOG_FILE") 2>&1
printf '\n----- %s  install-bridge %s  action=%s dry_run=%s -----\n' "$(date '+%F %T %Z')" "$BRIDGE_VERSION" "$ACTION" "$DRY_RUN"

# -----------------------------------------------------------------------------
# Prompt helpers. With --yes or without a terminal they return the default.
# -----------------------------------------------------------------------------

# ask VAR "prompt" "default" - read a value into VAR
ask() {
  local __var="$1" __prompt="$2" __def="${3:-}" __ans=""
  if [ "$ASSUME_YES" -eq 1 ] || ! [ -r /dev/tty ]; then printf -v "$__var" '%s' "$__def"; return; fi
  if [ -n "$__def" ]; then printf '%s [%s]: ' "$__prompt" "$__def" >/dev/tty; else printf '%s: ' "$__prompt" >/dev/tty; fi
  IFS= read -r __ans </dev/tty || true
  printf -v "$__var" '%s' "${__ans:-$__def}"
}

# confirm_no "question" - yes/no question that defaults to NO
confirm_no() {
  local a
  [ "$ASSUME_YES" -eq 1 ] && return 1
  [ -r /dev/tty ] || return 1
  printf '%s [y/N]: ' "$1" >/dev/tty; IFS= read -r a </dev/tty || true
  case "$a" in y|Y|yes|YES) return 0 ;; *) return 1 ;; esac
}

# confirm "question" - yes/no question that defaults to YES
confirm() {
  local a
  [ "$ASSUME_YES" -eq 1 ] && return 0
  [ -r /dev/tty ] || return 0
  printf '%s [Y/n]: ' "$1" >/dev/tty; IFS= read -r a </dev/tty || true
  case "$a" in n|N|no|NO) return 1 ;; *) return 0 ;; esac
}

# -----------------------------------------------------------------------------
# Small utilities
# -----------------------------------------------------------------------------
q() { printf '%q' "$1"; }                       # shell-quote one argument for remote use
host_part() { printf '%s' "${1#*@}"; }          # "user@host" -> "host"
ssh_target() { case "$1" in *@*) printf '%s' "$1" ;; *) printf '%s@%s' "$SSH_USER" "$1" ;; esac; }
is_ipv4() { [[ "$1" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]]; }

# run CMD ARGS... - execute a local command, or just print it in dry-run mode
run() {
  if [ "$DRY_RUN" -eq 1 ]; then printf '  [dry-run] local:'; printf ' %q' "$@"; printf '\n'; return 0; fi
  "$@"
}

# -----------------------------------------------------------------------------
# SSH plumbing. All remote access goes through rssh so dry-run is complete.
# -----------------------------------------------------------------------------
SSH_OPTS=()
build_ssh_opts() {
  # BatchMode: never ask for a password inside the script (keys only)
  SSH_OPTS=(-o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new -p "$SSH_PORT")
  [ -n "$SSH_KEY" ] && SSH_OPTS+=(-i "$SSH_KEY" -o IdentitiesOnly=yes)
  if [ -n "$SSH_EXTRA" ]; then
    local extra; read -r -a extra <<<"$SSH_EXTRA"; SSH_OPTS+=("${extra[@]}")
  fi
}

# Prefix for every remote command: $S becomes "sudo -n" when the SSH user is
# not root, so relay-side commands run as root without ever asking a password.
REMOTE_PRE='if [ "$(id -u)" -eq 0 ]; then S=""; else S="sudo -n"; fi; '

# rssh HOST "command string" - run a command on a relay (printed in dry-run)
rssh() {
  local tgt; tgt="$(ssh_target "$1")"
  if [ "$DRY_RUN" -eq 1 ]; then printf '  [dry-run] ssh %s: %s\n' "$tgt" "$2"; return 0; fi
  ssh "${SSH_OPTS[@]}" "$tgt" "$REMOTE_PRE$2"
}

# rcheck HOST "command" - read-only remote check (same as rssh, kept separate
# so read-only and mutating calls are easy to tell apart when reading the code)
rcheck() { rssh "$@"; }

# -----------------------------------------------------------------------------
# State file: remembers hosts, keys and blocked IPs so --status/--restore work
# later without repeating every flag.
# -----------------------------------------------------------------------------
save_state() {
  [ "$DRY_RUN" -eq 1 ] && { printf '  [dry-run] write state %s\n' "$STATE_FILE"; return 0; }
  umask 077
  {
    printf 'IR_HOST=%q\nFOREIGN_HOST=%q\nIR_DOMAIN=%q\nFOREIGN_DOMAIN=%q\n' "$IR_HOST" "$FOREIGN_HOST" "$IR_DOMAIN" "$FOREIGN_DOMAIN"
    printf 'SSH_USER=%q\nSSH_PORT=%q\nSSH_KEY=%q\nSSH_EXTRA=%q\n' "$SSH_USER" "$SSH_PORT" "$SSH_KEY" "$SSH_EXTRA"
    printf 'PORT=%q\nMODE=%q\nBRIDGE_ADDR=%q\nSERVICE_MGR=%q\n' "$PORT" "$MODE" "$BRIDGE_ADDR" "$SERVICE_MGR"
    printf 'IR_KEYS=%q\nFOREIGN_KEYS=%q\nIR_IP=%q\nFOREIGN_IP=%q\nSIMULATED=%q\n' "$IR_KEYS" "$FOREIGN_KEYS" "${IR_IP:-}" "${FOREIGN_IP:-}" "${SIMULATED:-0}"
  } >"$STATE_FILE"
}
load_state() {
  [ -f "$STATE_FILE" ] || die "no installation found ($STATE_FILE), run the installer first"
  # shellcheck source=/dev/null
  . "$STATE_FILE"
}

# -----------------------------------------------------------------------------
# Routing helpers
# -----------------------------------------------------------------------------

# endpoint_keys DOMAIN - lookup keys to register on the peer relay. An IP-only
# relay is addressed both as "1.2.3.4" and "[1.2.3.4]", so register both.
endpoint_keys() {
  if is_ipv4 "$1"; then printf '%s [%s]' "$1" "$1"; else printf '%s' "$1"; fi
}

# target_url - full URL each relay should deliver to. A full http:// URL avoids
# the host:port misparse seen in older madmail versions.
target_url() {
  if [ "$MODE" = "tunnel" ]; then printf 'http://127.0.0.1:%s/mxdeliv' "$PORT"
  else printf 'http://%s:%s/mxdeliv' "$BRIDGE_ADDR" "$PORT"; fi
}

# resolve_ip NAME - IPv4 of NAME as seen from this device (fallback only)
resolve_ip() {
  if is_ipv4 "$1"; then printf '%s' "$1"; return; fi
  getent ahostsv4 "$1" 2>/dev/null | awk 'NR==1{print $1}'
}

# remote_resolve HOST NAME - IPv4 of NAME as seen from HOST. This is the
# address that relay would really connect to, so it is the one to block.
remote_resolve() {
  if is_ipv4 "$2"; then printf '%s' "$2"; return; fi
  [ "$DRY_RUN" -eq 1 ] && return 0
  rssh "$1" "getent ahostsv4 $(q "$2") | awk 'NR==1{print \$1}'" 2>/dev/null || true
}

# pick_service_mgr - use systemd --user when a user manager is running,
# otherwise fall back to nohup supervisor loops.
pick_service_mgr() {
  if [ "$SERVICE_MGR" = "auto" ]; then
    if command -v systemctl >/dev/null 2>&1 && systemctl --user show-environment >/dev/null 2>&1; then
      SERVICE_MGR="systemd"
    else
      SERVICE_MGR="nohup"
    fi
  fi
}

# =============================================================================
# Install steps
# =============================================================================

# gather_inputs - fill missing values from prompts, validate, show a summary
gather_inputs() {
  say "Relay details"
  [ -n "$IR_HOST" ] || ask IR_HOST "SSH address of the domestic relay (e.g. root@ir.example.ir)" ""
  [ -n "$FOREIGN_HOST" ] || ask FOREIGN_HOST "SSH address of the foreign relay (e.g. root@203.0.113.50)" ""
  [ -n "$IR_HOST" ] || die "the domestic relay address is required (--ir-host)"
  [ -n "$FOREIGN_HOST" ] || die "the foreign relay address is required (--foreign-host)"
  [ -n "$IR_DOMAIN" ] || ask IR_DOMAIN "Mail domain of the domestic relay" "$(host_part "$IR_HOST")"
  [ -n "$FOREIGN_DOMAIN" ] || ask FOREIGN_DOMAIN "Mail domain or IP of the foreign relay" "$(host_part "$FOREIGN_HOST")"
  if [ "$ASSUME_YES" -eq 0 ] && [ -r /dev/tty ]; then
    ask MODE "Connection mode: tunnel or direct" "$MODE"
    ask PORT "Bridge port" "$PORT"
    ask INSTALL_METHOD "Install method: go, docker or binary" "$INSTALL_METHOD"
    if [ "$SIMULATE" -eq 0 ] && confirm_no "Simulate an outage for testing?"; then SIMULATE=1; fi
  fi
  case "$MODE" in tunnel|direct) ;; *) die "unknown mode: $MODE" ;; esac
  case "$INSTALL_METHOD" in go|docker|binary) ;; *) die "unknown install method: $INSTALL_METHOD" ;; esac
  [[ "$PORT" =~ ^[0-9]+$ ]] || die "port must be a number"
  if [ "$MODE" = "direct" ] && [ -z "$BRIDGE_ADDR" ]; then
    ask BRIDGE_ADDR "Address the relays use to reach this device" ""
    [ -n "$BRIDGE_ADDR" ] || die "direct mode needs the bridge address (--bridge-addr)"
  fi
  IR_KEYS="$(endpoint_keys "$IR_DOMAIN")"
  FOREIGN_KEYS="$(endpoint_keys "$FOREIGN_DOMAIN")"
  cat <<EOF

  Domestic relay  : $IR_HOST  (domain: $IR_DOMAIN)
  Foreign relay   : $FOREIGN_HOST  (domain: $FOREIGN_DOMAIN)
  Mode            : $MODE   port: $PORT   install: $INSTALL_METHOD
  Bridge URL      : $(target_url)
  Outage simulation: $([ "$SIMULATE" -eq 1 ] && echo yes || echo no)

EOF
  if ! confirm "Everything correct, start now?"; then say "OK, run it again whenever you are ready"; exit 0; fi
}

# check_deps - make sure the local tools for the chosen method exist
check_deps() {
  say "Checking local tools"
  local missing=() c
  for c in ssh curl awk; do command -v "$c" >/dev/null 2>&1 || missing+=("$c"); done
  case "$INSTALL_METHOD" in
    go) for c in git go; do command -v "$c" >/dev/null 2>&1 || missing+=("$c"); done ;;
    docker) for c in git docker; do command -v "$c" >/dev/null 2>&1 || missing+=("$c"); done ;;
  esac
  if [ "${#missing[@]}" -gt 0 ]; then
    if [ "$DRY_RUN" -eq 1 ]; then warn "missing tools: ${missing[*]}"
    else die "install these first: ${missing[*]} (e.g. sudo apt install ${missing[*]})"; fi
  fi
  ok "tools ready"
}

# ensure_ssh_key - use the given key, or ~/.ssh/id_ed25519, creating it if needed
ensure_ssh_key() {
  say "SSH key"
  if [ -z "$SSH_KEY" ]; then SSH_KEY="$HOME/.ssh/id_ed25519"; fi
  if [ ! -f "$SSH_KEY" ]; then
    run mkdir -p "$(dirname "$SSH_KEY")"
    run ssh-keygen -q -t ed25519 -N "" -C "madexchanger-bridge@$(hostname)" -f "$SSH_KEY"
    ok "created a new key: $SSH_KEY"
  else
    ok "using existing key: $SSH_KEY"
  fi
  build_ssh_opts
}

# check_relay HOST LABEL - passwordless SSH works, madmail exists, root/sudo works.
# If SSH fails, offer ssh-copy-id (interactive) or print the exact command.
check_relay() {
  local h="$1" label="$2" tgt
  tgt="$(ssh_target "$h")"
  if [ "$DRY_RUN" -eq 0 ] && ! ssh "${SSH_OPTS[@]}" "$tgt" true 2>/dev/null; then
    warn "passwordless SSH to the $label ($tgt) failed"
    if [ "$ASSUME_YES" -eq 0 ] && [ -r /dev/tty ] && confirm "Run ssh-copy-id for you? (asks for the server password)"; then
      ssh-copy-id -i "$SSH_KEY.pub" -p "$SSH_PORT" "$tgt" </dev/tty
      ssh "${SSH_OPTS[@]}" "$tgt" true || die "still cannot connect, check SSH access to $tgt"
    else
      die "run this and start again: ssh-copy-id -i $SSH_KEY.pub -p $SSH_PORT $tgt"
    fi
  fi
  rcheck "$h" 'command -v madmail >/dev/null || { echo "madmail not found" >&2; exit 3; }; $S true || { echo "need root or passwordless sudo" >&2; exit 4; }; madmail version 2>/dev/null | head -1' \
    || die "madmail not found on the $label, or no root access"
  ok "$label ready ($tgt)"
}

# install_madexchanger - get the binary into $MADEX_HOME/bin (skipped when it
# already exists, unless --rebuild). Built with CGO disabled: the SQLite driver
# is pure Go, so the binary is static and portable.
install_madexchanger() {
  say "Installing madexchanger"
  local bin="$MADEX_HOME/bin/madexchanger"
  run mkdir -p "$MADEX_HOME/bin"
  if [ -x "$bin" ] && [ "$REBUILD" -eq 0 ]; then ok "binary already present: $bin"; return; fi
  case "$INSTALL_METHOD" in
    go|docker)
      # clone once, fast-forward on later runs
      if [ -d "$MADEX_HOME/src/.git" ]; then
        run git -C "$MADEX_HOME/src" pull -q --ff-only
      else
        run git clone -q --depth 1 "$REPO_URL" "$MADEX_HOME/src"
      fi
      if [ "$INSTALL_METHOD" = "go" ]; then
        if [ "$DRY_RUN" -eq 1 ]; then printf '  [dry-run] local: (cd %q && CGO_ENABLED=0 go build -trimpath -o %q ./cmd/madexchanger)\n' "$MADEX_HOME/src" "$bin"
        else (cd "$MADEX_HOME/src" && CGO_ENABLED=0 go build -trimpath -buildvcs=false -o "$bin" ./cmd/madexchanger); fi
      else
        # build inside the official Go image, as the current user
        run docker run --rm -u "$(id -u):$(id -g)" -e HOME=/tmp -e CGO_ENABLED=0 \
          -v "$MADEX_HOME/src:/src" -v "$MADEX_HOME/bin:/out" -w /src golang:1.24-bookworm \
          go build -trimpath -buildvcs=false -o /out/madexchanger ./cmd/madexchanger
      fi ;;
    binary)
      [ -n "$BINARY_SRC" ] || die "the binary method needs a file or URL via --binary"
      case "$BINARY_SRC" in
        http://*|https://*) run curl -fsSL -o "$bin" "$BINARY_SRC" ;;
        *) run install -m 755 "$BINARY_SRC" "$bin" ;;
      esac
      run chmod 755 "$bin" ;;
  esac
  [ "$DRY_RUN" -eq 1 ] || "$bin" -version
  ok "madexchanger installed"
}

# write_config - config.yml for the bridge. Tunnel mode listens on loopback
# only (reachable just through the tunnels). An existing token is reused so
# re-runs do not change credentials. The token is never printed.
write_config() {
  say "Bridge configuration"
  local cfg="$MADEX_HOME/config.yml" token="" listen
  if [ "$MODE" = "tunnel" ]; then listen="127.0.0.1:$PORT"; else listen="0.0.0.0:$PORT"; fi
  if [ -f "$cfg" ]; then
    token="$(awk -F'"' '/^[[:space:]]*token:/{print $2; exit}' "$cfg")"
  fi
  if [ -z "$token" ]; then
    token="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
  fi
  if [ "$DRY_RUN" -eq 1 ]; then printf '  [dry-run] write %s (listen %s, token hidden, mode 600)\n' "$cfg" "$listen"; return; fi
  umask 077
  cat >"$cfg.tmp" <<EOF
# generated by install-bridge.sh $BRIDGE_VERSION
listen: "$listen"
receive_path: "/mxdeliv"
tls:
  cert_file: ""
  key_file: ""
downstream_url: ""
forward_timeout: 60
skip_tls_verify: true
proxy:
  url: ""
max_body_size: 33554432
relay_mode: "all"
database_path: "$MADEX_HOME/madexchanger.db"
log_level: "info"
admin_web:
  enabled: true
  path: "/admin"
  token: "$token"
EOF
  mv "$cfg.tmp" "$cfg"; chmod 600 "$cfg"
  ok "config saved: $cfg (contains the admin token)"
}

# write_tunnel_keeper - generates keep-tunnels.sh. For each relay it keeps one
# reverse tunnel (relay 127.0.0.1:PORT -> bridge 127.0.0.1:PORT). Liveness is
# checked via the PID file AND the process command line, so an unrelated ssh
# to the same host (or a reused PID) is never mistaken for our tunnel.
write_tunnel_keeper() {
  local keeper="$MADEX_HOME/keep-tunnels.sh"
  if [ "$DRY_RUN" -eq 1 ]; then printf '  [dry-run] write %s\n' "$keeper"; return; fi
  {
    printf '#!/usr/bin/env bash\n# generated by install-bridge.sh - keeps reverse tunnels up (PID files, no pgrep)\nset -euo pipefail\n'
    printf 'ROOT=%q\nPORT=%q\n' "$MADEX_HOME" "$PORT"
    printf 'SSH_OPTS=('; printf ' %q' "${SSH_OPTS[@]}"; printf ' )\n'
    printf 'HOSTS=(%q %q)\nNAMES=(ir foreign)\n' "$(ssh_target "$IR_HOST")" "$(ssh_target "$FOREIGN_HOST")"
    cat <<'EOF'
# alive PIDFILE - 0 if that exact tunnel process is still running
alive() {
  local pid
  [ -f "$1" ] || return 1
  pid="$(cat "$1" 2>/dev/null || true)"
  [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null || return 1
  tr '\0' ' ' <"/proc/$pid/cmdline" 2>/dev/null | grep -q -- "127.0.0.1:${PORT}:127.0.0.1:${PORT}"
}
for i in "${!HOSTS[@]}"; do
  pidf="$ROOT/tunnel-${NAMES[$i]}.pid"; logf="$ROOT/tunnel-${NAMES[$i]}.log"
  if alive "$pidf"; then continue; fi
  # setsid: detach from the keeper so the tunnel outlives it
  setsid ssh -N "${SSH_OPTS[@]}" -o ExitOnForwardFailure=yes -o ServerAliveInterval=30 \
    -o ServerAliveCountMax=3 -R "127.0.0.1:${PORT}:127.0.0.1:${PORT}" "${HOSTS[$i]}" \
    >>"$logf" 2>&1 </dev/null &
  echo $! >"$pidf"
  echo "$(date '+%F %T') tunnel ${NAMES[$i]} started pid $! -> ${HOSTS[$i]}" >>"$logf"
done
EOF
  } >"$keeper"
  chmod 700 "$keeper"
}

# setup_services - start the exchanger and (tunnel mode) the tunnel keeper.
# systemd: user units + timer + linger. nohup: supervisor loops with PID files.
setup_services() {
  say "Services"
  pick_service_mgr
  local bin="$MADEX_HOME/bin/madexchanger" cfg="$MADEX_HOME/config.yml"
  [ "$MODE" = "tunnel" ] && write_tunnel_keeper
  if [ "$SERVICE_MGR" = "systemd" ]; then
    local ud="$HOME/.config/systemd/user"
    if [ "$DRY_RUN" -eq 1 ]; then
      printf '  [dry-run] write %s/madexchanger.service%s\n' "$ud" "$([ "$MODE" = tunnel ] && echo ' madex-tunnels.service madex-tunnels.timer')"
    else
      mkdir -p "$ud"
      cat >"$ud/madexchanger.service" <<EOF
[Unit]
Description=Madexchanger bridge (install-bridge.sh)
After=network-online.target

[Service]
WorkingDirectory=$MADEX_HOME
ExecStart=$bin -config $cfg
Restart=always
RestartSec=3

[Install]
WantedBy=default.target
EOF
      if [ "$MODE" = "tunnel" ]; then
        cat >"$ud/madex-tunnels.service" <<EOF
[Unit]
Description=Madexchanger reverse SSH tunnels (install-bridge.sh)
After=network-online.target madexchanger.service

[Service]
Type=oneshot
# keep the ssh children alive after the keeper exits; never RemainAfterExit,
# otherwise the timer stops re-checking the tunnels
KillMode=process
ExecStart=$MADEX_HOME/keep-tunnels.sh
EOF
        cat >"$ud/madex-tunnels.timer" <<'EOF'
[Unit]
Description=Check madexchanger tunnels every minute

[Timer]
OnBootSec=20
OnUnitActiveSec=60
AccuracySec=10

[Install]
WantedBy=timers.target
EOF
      fi
    fi
    run systemctl --user daemon-reload
    run systemctl --user enable madexchanger.service
    run systemctl --user restart madexchanger.service
    if [ "$MODE" = "tunnel" ]; then
      run systemctl --user enable --now madex-tunnels.timer
      run systemctl --user start madex-tunnels.service
    fi
    # linger keeps user services running without an active login session
    if command -v loginctl >/dev/null 2>&1; then run loginctl enable-linger "$USER" || warn "could not enable linger, services may stop after logout"; fi
  else
    warn "no systemd user session, using nohup (run the script again after a reboot)"
    local sup="$MADEX_HOME/supervise.sh"
    if [ "$DRY_RUN" -eq 1 ]; then printf '  [dry-run] write+start %s\n' "$sup"
    else
      stop_nohup
      # supervisor: restart the exchanger if it exits; re-check tunnels every minute
      cat >"$sup" <<EOF
#!/usr/bin/env bash
# generated by install-bridge.sh - nohup fallback supervisor
cd $(q "$MADEX_HOME")
( while true; do $(q "$bin") -config $(q "$cfg") >>$(q "$MADEX_HOME/madexchanger.log") 2>&1; sleep 3; done ) &
echo \$! > $(q "$MADEX_HOME/supervise-mx.pid")
EOF
      if [ "$MODE" = "tunnel" ]; then
        cat >>"$sup" <<EOF
( while true; do $(q "$MADEX_HOME/keep-tunnels.sh") || true; sleep 60; done ) &
echo \$! > $(q "$MADEX_HOME/supervise-tun.pid")
EOF
      fi
      chmod 700 "$sup"
      setsid nohup "$sup" >/dev/null 2>&1 </dev/null &
      sleep 1
    fi
  fi
  ok "services started ($SERVICE_MGR)"
}

# stop_nohup - stop the nohup supervisor loops and the exchanger they started
stop_nohup() {
  local p pid
  for p in supervise-mx supervise-tun; do
    if [ -f "$MADEX_HOME/$p.pid" ]; then
      pid="$(cat "$MADEX_HOME/$p.pid")"
      kill "$pid" 2>/dev/null || true
      rm -f "$MADEX_HOME/$p.pid"
    fi
  done
  # belt and braces: any leftover loop of OUR supervisor (exact path match only),
  # e.g. if a PID file was lost or rewritten during a quick re-run
  for pid in $(pgrep -f -x "bash $MADEX_HOME/supervise.sh" 2>/dev/null || true); do kill "$pid" 2>/dev/null || true; done
  # the exchanger itself is a child of the loop; match its exact command line
  if [ -x "$MADEX_HOME/bin/madexchanger" ]; then
    for pid in $(pgrep -f -x "$MADEX_HOME/bin/madexchanger -config $MADEX_HOME/config.yml" 2>/dev/null || true); do kill "$pid" 2>/dev/null || true; done
  fi
}

# stop_tunnels - kill only processes that really are our tunnels, then drop PID files
stop_tunnels() {
  local f pid
  for f in "$MADEX_HOME"/tunnel-*.pid; do
    [ -f "$f" ] || continue
    pid="$(cat "$f")"
    if [ -r "/proc/$pid/cmdline" ] && tr '\0' ' ' <"/proc/$pid/cmdline" | grep -q -- "127.0.0.1:${PORT}:127.0.0.1:${PORT}"; then
      run kill "$pid" || true
    fi
    run rm -f "$f"
  done
}

# wait_local_health - wait up to 20 s for the exchanger's /health on this device
wait_local_health() {
  [ "$DRY_RUN" -eq 1 ] && return 0
  for _ in $(seq 1 20); do
    curl -fsS -m 3 "http://127.0.0.1:$PORT/health" >/dev/null 2>&1 && return 0
    sleep 1
  done
  return 1
}

# wait_tunnel HOST - wait up to 20 s until the relay reaches the bridge via its tunnel
wait_tunnel() {
  [ "$DRY_RUN" -eq 1 ] && return 0
  for _ in $(seq 1 20); do
    rcheck "$1" "curl -fsS -m 3 http://127.0.0.1:$PORT/health >/dev/null 2>&1" 2>/dev/null && return 0
    sleep 1
  done
  return 1
}

# set_endpoints - point each relay at the bridge for the OTHER relay's domain.
# `endpoint-cache set` is an upsert, so re-runs are safe.
set_endpoints() {
  say "Pointing both relays at the bridge"
  local url k; url="$(target_url)"
  # on the foreign relay: mail for the domestic domain goes to the bridge
  for k in $IR_KEYS; do
    rssh "$FOREIGN_HOST" "\$S madmail endpoint-cache set $(q "$k") $(q "$url") 'via madexchanger bridge' >/dev/null" \
      || die "endpoint-cache set failed on the foreign relay"
  done
  ok "foreign relay: $IR_KEYS -> $url"
  # on the domestic relay: mail for the foreign domain/IP goes to the bridge
  for k in $FOREIGN_KEYS; do
    rssh "$IR_HOST" "\$S madmail endpoint-cache set $(q "$k") $(q "$url") 'via madexchanger bridge' >/dev/null" \
      || die "endpoint-cache set failed on the domestic relay"
  done
  ok "domestic relay: $FOREIGN_KEYS -> $url"
}

# ipt_rule ACTION DEST - remote iptables command for our tagged outage rule
# (ACTION: C=check, I=insert, D=delete). The comment tag makes removal exact.
ipt_rule() {
  printf '$S iptables -%s OUTPUT -d %s -m comment --comment madex-bridge-sim -j REJECT --reject-with icmp-host-unreachable' "$1" "$2"
}

# simulate_outage - block relay<->relay traffic in both directions (insert only
# if the rule is not there yet, so re-runs never duplicate it)
simulate_outage() {
  say "Simulating the outage"
  # resolve each relay the way the OTHER relay sees it (that is the path we block)
  [ -n "${IR_IP:-}" ] || IR_IP="$(remote_resolve "$FOREIGN_HOST" "$IR_DOMAIN")"
  [ -n "${IR_IP:-}" ] || IR_IP="$(resolve_ip "$(host_part "$IR_HOST")")"
  [ -n "${FOREIGN_IP:-}" ] || FOREIGN_IP="$(remote_resolve "$IR_HOST" "$FOREIGN_DOMAIN")"
  [ -n "${FOREIGN_IP:-}" ] || FOREIGN_IP="$(resolve_ip "$(host_part "$FOREIGN_HOST")")"
  if [ -z "$IR_IP" ] || [ -z "$FOREIGN_IP" ]; then die "could not resolve the IP of one of the relays"; fi
  rssh "$IR_HOST" "$(ipt_rule C "$FOREIGN_IP") 2>/dev/null || $(ipt_rule I "$FOREIGN_IP")"
  rssh "$FOREIGN_HOST" "$(ipt_rule C "$IR_IP") 2>/dev/null || $(ipt_rule I "$IR_IP")"
  SIMULATED=1
  ok "direct path $IR_IP <-> $FOREIGN_IP blocked, undo with: $0 --restore"
}

# remove_outage - delete every copy of our tagged rule on both relays
remove_outage() {
  [ -n "${IR_IP:-}" ] && [ -n "${FOREIGN_IP:-}" ] || return 0
  rssh "$IR_HOST" "while $(ipt_rule C "$FOREIGN_IP") 2>/dev/null; do $(ipt_rule D "$FOREIGN_IP"); done" || warn "could not remove the iptables rule on the domestic relay"
  rssh "$FOREIGN_HOST" "while $(ipt_rule C "$IR_IP") 2>/dev/null; do $(ipt_rule D "$IR_IP"); done" || warn "could not remove the iptables rule on the foreign relay"
  ok "outage simulation rules removed"
}

# health - /health on the bridge, /health through each tunnel, and the relay's
# own view (endpoint-cache list + federation status). Returns the failure count.
health() {
  say "Health check"
  local fails=0 out
  if [ "$DRY_RUN" -eq 1 ]; then
    printf '  [dry-run] local: curl -fsS http://127.0.0.1:%s/health\n' "$PORT"
  elif out="$(curl -fsS -m 5 "http://127.0.0.1:$PORT/health" 2>/dev/null)"; then ok "bridge: $out"
  else warn "bridge is not answering (http://127.0.0.1:$PORT/health)"; fails=$((fails+1)); fi
  local h label
  for h in "$IR_HOST" "$FOREIGN_HOST"; do
    if [ "$h" = "$IR_HOST" ]; then label="domestic relay"; else label="foreign relay"; fi
    if [ "$DRY_RUN" -eq 1 ]; then
      [ "$MODE" = "tunnel" ] && rcheck "$h" "curl -fsS -m 5 http://127.0.0.1:$PORT/health"
      rcheck "$h" '$S madmail endpoint-cache list; $S madmail federation status'
      continue
    fi
    if [ "$MODE" = "tunnel" ]; then
      if out="$(rcheck "$h" "curl -fsS -m 5 http://127.0.0.1:$PORT/health")"; then ok "$label via tunnel: ${out:-ok}"
      else warn "$label cannot reach the bridge, check the tunnel"; fails=$((fails+1)); fi
    fi
    if out="$(rcheck "$h" '$S madmail endpoint-cache list 2>&1 | head -20; echo "--"; $S madmail federation status 2>&1 | head -20')"; then
      printf '%s\n' "$out" | sed "s/^/      [$label] /"
    fi
  done
  return "$fails"
}

# summary - where everything lives and what to run next
summary() {
  cat <<EOF

${C_B}Bridge is ready${C_0}
  Config + admin token : $MADEX_HOME/config.yml
  Install log          : $LOG_FILE
  Status               : $0 --status
  Back to normal       : $0 --restore
  Admin dashboard      : http://127.0.0.1:$PORT/admin/
EOF
}

# =============================================================================
# Actions
# =============================================================================

# do_install - the full install flow (safe to re-run)
do_install() {
  gather_inputs
  check_deps
  ensure_ssh_key
  check_relay "$IR_HOST" "domestic relay"
  check_relay "$FOREIGN_HOST" "foreign relay"
  install_madexchanger
  write_config
  save_state
  setup_services
  wait_local_health || die "the bridge did not start, see $MADEX_HOME/madexchanger.log"
  if [ "$MODE" = "tunnel" ]; then
    wait_tunnel "$IR_HOST" || warn "domestic relay tunnel is not up yet"
    wait_tunnel "$FOREIGN_HOST" || warn "foreign relay tunnel is not up yet"
  fi
  set_endpoints
  SIMULATED=0
  [ "$SIMULATE" -eq 1 ] && simulate_outage
  save_state
  if health; then summary; else summary; warn "some checks failed, see above"; exit 2; fi
}

# do_status - services, tunnel processes and the health check, from saved state
do_status() {
  load_state; build_ssh_opts; pick_service_mgr
  say "Services ($SERVICE_MGR)"
  if [ "$SERVICE_MGR" = "systemd" ]; then
    systemctl --user --no-pager is-active madexchanger.service madex-tunnels.timer 2>/dev/null || true
  fi
  local f
  for f in "$MADEX_HOME"/tunnel-*.pid; do
    [ -f "$f" ] || continue
    if kill -0 "$(cat "$f")" 2>/dev/null; then ok "$(basename "$f" .pid) running"; else warn "$(basename "$f" .pid) stopped"; fi
  done
  health
}

# do_restore - undo everything on the relays, stop local services; --purge
# also removes the local files
do_restore() {
  load_state; build_ssh_opts; pick_service_mgr
  say "Restoring normal operation"
  local k
  for k in $IR_KEYS; do rssh "$FOREIGN_HOST" "\$S madmail endpoint-cache remove $(q "$k") >/dev/null 2>&1 || true"; done
  for k in $FOREIGN_KEYS; do rssh "$IR_HOST" "\$S madmail endpoint-cache remove $(q "$k") >/dev/null 2>&1 || true"; done
  ok "endpoint-cache entries removed, relays talk directly again"
  remove_outage
  if [ "$SERVICE_MGR" = "systemd" ]; then
    run systemctl --user disable --now madex-tunnels.timer madexchanger.service 2>/dev/null || true
  else
    [ "$DRY_RUN" -eq 1 ] || stop_nohup
  fi
  stop_tunnels
  ok "services and tunnels stopped"
  if [ "$PURGE" -eq 1 ]; then
    run rm -rf "$MADEX_HOME"
    run rm -f "$HOME/.config/systemd/user/madexchanger.service" "$HOME/.config/systemd/user/madex-tunnels.service" "$HOME/.config/systemd/user/madex-tunnels.timer"
    run rm -f "$STATE_FILE"
    ok "all bridge files removed"
  else
    SIMULATED=0
    save_state
  fi
}

case "$ACTION" in
  install) do_install ;;
  status) do_status ;;
  restore) do_restore ;;
esac
