#!/usr/bin/env bash
set -uo pipefail
for ns in ir fx; do
  [ -f "/run/sshd-$ns.pid" ] && sudo kill "$(cat /run/sshd-$ns.pid)" 2>/dev/null
  sudo ip netns pids "$ns" 2>/dev/null | xargs -r sudo kill 2>/dev/null
  sudo ip netns del "$ns" 2>/dev/null
  sudo ip link del "v-$ns" 2>/dev/null
  sudo rm -f "/etc/sudoers.d/mxlab-${ns}srv"
done
sudo ip link del br-mxlab 2>/dev/null
sudo sed -i '/ ir.test$/d' /etc/hosts
sudo rm -rf /var/lib/madmail-stub /usr/local/bin/madmail
[ -f /tmp/mxlab/brnf.orig ] && sudo sysctl -q -w "net.bridge.bridge-nf-call-iptables=$(cat /tmp/mxlab/brnf.orig)"
echo "lab down"
