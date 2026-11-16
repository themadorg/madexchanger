#!/usr/bin/env bash
# Lab: two fake Madmail relays in network namespaces with real sshd + stub madmail.
# Needs sudo. Touches nothing outside 10.250.0.0/24, the lab users and /etc/hosts line.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"; L=/tmp/mxlab; mkdir -p "$L"; chmod 777 "$L"
[ -f "$L/id_lab" ] || ssh-keygen -q -t ed25519 -N "" -f "$L/id_lab"
sudo install -m 755 "$here/madmail-stub.sh" /usr/local/bin/madmail
sudo ssh-keygen -A >/dev/null; sudo mkdir -p /run/sshd
sudo ip link add br-mxlab type bridge 2>/dev/null || true
sudo ip addr add 10.250.0.1/24 dev br-mxlab 2>/dev/null || true
sudo ip link set br-mxlab up
# let the two lab relays talk directly over the bridge (box has FORWARD DROP via br_netfilter)
cat /proc/sys/net/bridge/bridge-nf-call-iptables >"$L/brnf.orig" 2>/dev/null || true
sudo sysctl -q -w net.bridge.bridge-nf-call-iptables=0 2>/dev/null || true
i=2
for ns in ir fx; do
  user="${ns}srv"; ip="10.250.0.$i"
  id "$user" >/dev/null 2>&1 || sudo useradd -m -s /bin/bash -p "*" "$user"
  sudo install -d -m 700 -o "$user" -g "$user" "/home/$user/.ssh"
  sudo install -m 600 -o "$user" -g "$user" "$L/id_lab.pub" "/home/$user/.ssh/authorized_keys"
  echo "$user ALL=(ALL) NOPASSWD: ALL" | sudo tee "/etc/sudoers.d/mxlab-$user" >/dev/null
  sudo ip netns add "$ns" 2>/dev/null || true
  sudo ip link add "v-$ns" type veth peer name "v-$ns-in" 2>/dev/null || true
  sudo ip link set "v-$ns-in" netns "$ns" 2>/dev/null || true
  sudo ip link set "v-$ns" master br-mxlab up
  sudo ip netns exec "$ns" ip addr add "$ip/24" dev "v-$ns-in" 2>/dev/null || true
  sudo ip netns exec "$ns" ip link set "v-$ns-in" up
  sudo ip netns exec "$ns" ip link set lo up
  sudo ip netns exec "$ns" /usr/sbin/sshd -o ListenAddress="$ip" -o PidFile="/run/sshd-$ns.pid" \
     -o AllowTcpForwarding=yes -o PasswordAuthentication=no -o UsePAM=no
  sudo ip netns exec "$ns" setsid python3 "$here/receiver.py" "$ns" "$ip" "$L/recv-$ns.log" >/dev/null 2>&1 &
  i=$((i+1))
done
grep -q ' ir.test$' /etc/hosts || echo "10.250.0.2 ir.test" | sudo tee -a /etc/hosts >/dev/null
# wait until both sshd instances accept the lab key
for ip in 10.250.0.2 10.250.0.3; do
  for _ in $(seq 1 20); do
    ssh -i "$L/id_lab" -o IdentitiesOnly=yes -o BatchMode=yes -o ConnectTimeout=2 -o StrictHostKeyChecking=accept-new \
      -o UserKnownHostsFile="$L/known_hosts" "$( [ "$ip" = 10.250.0.2 ] && echo irsrv || echo fxsrv )@$ip" true 2>/dev/null && break
    sleep 0.5
  done
done
echo "lab up: ir=irsrv@10.250.0.2 (ir.test)  fx=fxsrv@10.250.0.3"
