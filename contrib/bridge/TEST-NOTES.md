# install-bridge.sh test notes (box only, 2026-11-15 Tehran)

v0.2.0: script fully rewritten in English (0 Persian/Arabic/ZWNJ characters, pure ASCII).
Run the suite with `test/lab-up.sh && test/run-lab-tests.sh; test/lab-down.sh` (28 checks).

No real servers (delta, alireza, acer) or the user's laptop were touched.

## Lab
- `test/lab-up.sh` / `test/lab-down.sh`: two network namespaces on bridge `br-mxlab`
  - `ir` = 10.250.0.2 (`ir.test` via /etc/hosts), user `irsrv`
  - `fx` = 10.250.0.3 (IP-only relay, like the real foreign one), user `fxsrv`
  - each runs a real `sshd`, a fake `/mxdeliv` receiver on :80 (`test/receiver.py`)
    and a stub `madmail` (`test/madmail-stub.sh`) that records endpoint-cache calls
  - lab users are non-root with passwordless sudo, so the script's `sudo -n` path is exercised
- Bridge = the box itself, nohup service manager (box has no systemd user session)

## Results
| Test | Result |
|---|---|
| `shellcheck -x install-bridge.sh` | clean (SC2016/SC2029 disabled file-wide on purpose: remote command strings) |
| `--help`, unknown option | ok, Persian error |
| `--dry-run` (tunnel+outage, direct+binary) | prints every local + ssh command, changes nothing |
| Interactive prompts (via `script` pty) | all prompts answered, cancel exits cleanly |
| Full install, `--install-method go`, `--simulate-outage` | exit 0 in ~16 s; clone+build, config 600, tunnels up, endpoint-cache set on both, iptables rule each side, health ok locally + through both tunnels |
| Direct relay↔relay after install | blocked (curl 000) |
| ir → bridge (tunnel) → fx `/mxdeliv` | HTTP 200, fx receiver got `a@ir.test → b@[10.250.0.3]` |
| fx → bridge (tunnel) → ir `/mxdeliv` | HTTP 200, ir receiver got `b@[10.250.0.3] → a@ir.test` |
| Kill a tunnel, run keeper | new tunnel PID, health through tunnel ok again |
| Re-run install (idempotent) | binary reused, config/token byte-identical, 1 iptables rule per side, no duplicate endpoints, 1 exchanger, 2 tunnels |
| `--status` | services, tunnels, health ok |
| `--restore` | endpoint entries 0/0, iptables 0/0, no exchanger/tunnel/supervisor processes, direct path works again |
| `--restore --purge` | ~/madexchanger removed, state removed (install.log kept) |
| Unreachable relay | stops with exact `ssh-copy-id` command |
| Token leakage | token never printed; not present in install.log |
| Persian characters in script | 0 (grep for U+0600-06FF, U+200C, U+FB50-FEFF) |
| Suite stability | 28/28 passed three times in a row after hardening `stop_nohup` (one earlier run left a supervisor loop behind; leftover loops are now also matched by exact path) |

## Not tested
- systemd --user units + linger (box has no systemd); unit text is based on the working ایسر units,
  with one deliberate change: the tunnel unit no longer uses `RemainAfterExit=yes`
  (on ایسر that made the timer stop re-checking) and uses `KillMode=process` so ssh children survive
- `--install-method docker` (no docker on box), `--binary URL` download
- real `madmail endpoint-cache` / `federation status` (stub only); syntax matches madmail 2.30.5 `--help`
- real iptables on a production kernel, ssh-copy-id interactive flow, IPv6 relays
- real Delta Chat clients end to end
