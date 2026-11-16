# Madexchanger bridge installer

Keeps a domestic (Iran) and a foreign Delta Chat / Madmail relay connected during an
international internet shutdown. Run it on a **bridge device** (Linux laptop or mini-PC)
that can still reach both relays over SSH; it sets up
[madexchanger](https://github.com/themadorg/madexchanger) in the middle.

```
[domestic Madmail] --X-- direct path down --X--> [foreign Madmail]
        |                                               ^
        | reverse SSH tunnel            reverse SSH tunnel |
        v                                               |
        +------> [bridge device: madexchanger :19080] ---+
```

## One command

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/themadorg/madexchanger/main/contrib/bridge/install-bridge.sh)
```

It asks for the two relays and a few options, then:

1. checks local tools and your SSH key (creates one if missing)
2. verifies passwordless SSH and `madmail` (root or `sudo -n`) on both relays
3. builds madexchanger (Go, Docker, or a given binary) and writes `~/madexchanger/config.yml`
   with a random admin token (mode 600, never printed)
4. starts the exchanger and reverse SSH tunnels (systemd `--user` + timer + linger, or a
   nohup fallback) with a PID-file based tunnel keeper
5. runs `madmail endpoint-cache set` on both relays with the full
   `http://127.0.0.1:19080/mxdeliv` URL (both `IP` and `[IP]` for IP-only relays)
6. optionally blocks the direct path with tagged iptables rules to simulate the outage
7. health-checks the bridge, each tunnel and `madmail federation status`, then prints a summary

## Non-interactive

```bash
./install-bridge.sh --yes \
  --ir-host root@ir.example.ir \
  --foreign-host root@203.0.113.50
```

| Option | Meaning |
|---|---|
| `--ir-domain`, `--foreign-domain` (`--foreign-key`) | mail domain when it differs from the SSH host |
| `--ssh-user`, `--ssh-port`, `--ssh-key` | SSH settings (default `root`, `22`, `~/.ssh/id_ed25519`) |
| `--mode tunnel\|direct`, `--bridge-addr` | tunnel (default, works behind NAT) or relays reach the bridge directly |
| `--install-method go\|docker\|binary`, `--binary PATH\|URL`, `--rebuild` | how to get madexchanger |
| `--port` | bridge port (default 19080) |
| `--service-manager auto\|systemd\|nohup` | service backend |
| `--simulate-outage`, `--ir-ip`, `--foreign-ip` | test mode: block relay-to-relay traffic |
| `--dry-run` | print every local and remote command, change nothing |
| `-y`, `--yes` | no prompts |

## Later

```bash
./install-bridge.sh --status            # bridge, tunnels, relay health
./install-bridge.sh --restore           # remove endpoint entries + outage rules, stop services
./install-bridge.sh --restore --purge   # also delete ~/madexchanger and state
```

Re-running the installer is safe: the binary and admin token are reused, endpoints are
upserted and iptables rules are never duplicated.

## Notes

- Keep the bridge device on; while it is down, mail waits in the relays' queues.
- The bridge sees sender, recipient, size and the encrypted payload, not message contents.
- Install log: `~/.config/madexchanger-bridge/install.log`.
- Tests: see `TEST-NOTES.md` and `test/run-lab-tests.sh`.
