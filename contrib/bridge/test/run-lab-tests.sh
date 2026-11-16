#!/usr/bin/env bash
# Full lab test suite for install-bridge.sh (box only; needs sudo, lab-up.sh first).
set -uo pipefail
# shellcheck disable=SC2029  # remote test commands are built client-side on purpose
export PATH="$PATH:/usr/sbin"
here="$(cd "$(dirname "$0")" && pwd)"; S="$here/../install-bridge.sh"; L=/tmp/mxlab
O=(-i "$L/id_lab" -o IdentitiesOnly=yes -o BatchMode=yes -o UserKnownHostsFile="$L/known_hosts")
export MADEX_HOME="$L/bridge" MADEX_STATE_DIR="$L/state" MADEX_SSH_OPTS="-o UserKnownHostsFile=$L/known_hosts"
ARGS=(--yes --ir-host irsrv@10.250.0.2 --ir-domain ir.test --foreign-host fxsrv@10.250.0.3 --ssh-key "$L/id_lab")
pass=0; fail=0
check() { if eval "$2"; then echo "PASS | $1"; pass=$((pass+1)); else echo "FAIL | $1"; fail=$((fail+1)); fi; }
rir() { ssh "${O[@]}" irsrv@10.250.0.2 "$1"; }
rfx() { ssh "${O[@]}" fxsrv@10.250.0.3 "$1"; }
rm -rf "$L/bridge" "$L/state" "$L"/recv-*.log

check "shellcheck clean" "shellcheck -x '$S' >/dev/null"
check "no Persian/Arabic/ZWNJ chars" "! grep -qP '[\x{0600}-\x{06FF}\x{200C}\x{FB50}-\x{FEFF}]' '$S'"
check "--help" "'$S' --help | grep -q 'Usage: install-bridge.sh'"
check "unknown option rejected" "! '$S' --bogus >/dev/null 2>&1"
check "dry-run prints remote endpoint + iptables commands" \
  "MADEX_HOME=$L/dry MADEX_STATE_DIR=$L/drys '$S' --dry-run --yes --ir-host root@ir.example.ir --foreign-host 203.0.113.50 --simulate-outage --ssh-key $L/dry/key >$L/dry.txt 2>&1; grep -q 'endpoint-cache set 203.0.113.50 ' $L/dry.txt && grep -q 'iptables -I OUTPUT' $L/dry.txt && [ ! -e $L/dry/bin/madexchanger ]"
check "full install (go build, outage sim) exit 0" "'$S' ${ARGS[*]} --simulate-outage >$L/install.txt 2>&1"
check "config.yml mode 600" "[ \"\$(stat -c %a $L/bridge/config.yml)\" = 600 ]"
check "direct ir->fx blocked" "! rir 'curl -s -m3 http://10.250.0.3/ >/dev/null'"
check "direct fx->ir blocked" "! rfx 'curl -s -m3 http://ir.test/ >/dev/null'"
check "ir -> bridge -> fx relayed" "[ \"\$(rir 'curl -s -o /dev/null -w %{http_code} -X POST http://127.0.0.1:19080/mxdeliv -H \"X-Mail-From: a@ir.test\" -H \"X-Mail-To: b@[10.250.0.3]\" --data-binary t1')\" = 200 ] && sleep 1 && grep -q 'fx got.*a@ir.test' $L/recv-fx.log"
check "fx -> bridge -> ir relayed" "[ \"\$(rfx 'curl -s -o /dev/null -w %{http_code} -X POST http://127.0.0.1:19080/mxdeliv -H \"X-Mail-From: b@[10.250.0.3]\" -H \"X-Mail-To: a@ir.test\" --data-binary t2')\" = 200 ] && sleep 1 && grep -q 'ir got.*a@ir.test' $L/recv-ir.log"
check "endpoint-cache entries (ir=2, fx=1)" "[ \$(wc -l </var/lib/madmail-stub/irsrv.endpoints) = 2 ] && [ \$(wc -l </var/lib/madmail-stub/fxsrv.endpoints) = 1 ]"
old=$(cat "$L/bridge/tunnel-ir.pid"); kill "$old"; sleep 1; "$L/bridge/keep-tunnels.sh"; sleep 2
check "tunnel kill recovery (new pid, health via tunnel)" "[ \"\$(cat $L/bridge/tunnel-ir.pid)\" != $old ] && rir 'curl -fs -m3 http://127.0.0.1:19080/health' | grep -q ok"
sha=$(sha256sum "$L/bridge/config.yml")
check "re-run install exit 0" "'$S' ${ARGS[*]} --simulate-outage >$L/rerun.txt 2>&1"
check "re-run: config/token unchanged" "[ \"\$(sha256sum $L/bridge/config.yml)\" = \"$sha\" ]"
check "re-run: binary reused" "grep -q 'binary already present' $L/rerun.txt"
check "re-run: 1 iptables rule per side" "[ \$(sudo ip netns exec ir iptables -S OUTPUT | grep -c madex-bridge-sim) = 1 ] && [ \$(sudo ip netns exec fx iptables -S OUTPUT | grep -c madex-bridge-sim) = 1 ]"
check "re-run: no duplicate endpoints" "[ \$(wc -l </var/lib/madmail-stub/irsrv.endpoints) = 2 ] && [ \$(wc -l </var/lib/madmail-stub/fxsrv.endpoints) = 1 ]"
check "re-run: 1 exchanger, 2 tunnels" "[ \$(pgrep -c -f '$L/bridge/bin/madexchanger -config') = 1 ] && [ \$(pgrep -c -f '127.0.0.1:19080:127.0.0.1:19080') = 2 ]"
check "--status healthy" "'$S' --status 2>&1 | grep -q 'foreign relay via tunnel'"
check "token never in log" "! grep -qE '[0-9a-f]{64}' $L/state/install.log"
check "--restore exit 0" "'$S' --restore >$L/restore.txt 2>&1"
check "restore: endpoints removed" "[ ! -s /var/lib/madmail-stub/irsrv.endpoints ] && [ ! -s /var/lib/madmail-stub/fxsrv.endpoints ]"
check "restore: iptables rules removed" "[ \$(sudo ip netns exec ir iptables -S OUTPUT | grep -c madex-bridge-sim) = 0 ] && [ \$(sudo ip netns exec fx iptables -S OUTPUT | grep -c madex-bridge-sim) = 0 ]"
check "restore: no exchanger/tunnel/supervisor left" "! pgrep -f '$L/bridge/bin/madexchanger -config' >/dev/null && ! pgrep -f '127.0.0.1:19080:127.0.0.1:19080' >/dev/null && ! pgrep -f supervise.sh >/dev/null"
check "restore: direct path works again" "rir 'curl -fs -m3 http://10.250.0.3/' | grep -q fx && rfx 'curl -fs -m3 http://ir.test/' | grep -q ir"
check "--restore --purge removes files" "'$S' --restore --purge >$L/purge.txt 2>&1 && [ ! -d $L/bridge ] && [ ! -f $L/state/state.env ]"
check "unreachable relay stops with ssh-copy-id hint" "MADEX_HOME=$L/b2 MADEX_STATE_DIR=$L/s2 MADEX_SSH_OPTS='-o UserKnownHostsFile=$L/known_hosts -o ConnectTimeout=3' '$S' --yes --ir-host irsrv@10.250.0.9 --ir-domain ir.test --foreign-host fxsrv@10.250.0.3 --ssh-key $L/id_lab >$L/unreach.txt 2>&1; [ \$? -ne 0 ] && grep -q 'ssh-copy-id -i' $L/unreach.txt"
echo "TOTAL pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
