# Madexchanger operator guide

How to deploy and test Madexchanger as an HTTP federation relay between Madmail (chatmail) servers.

## Naming in this document

| Name | Role |
|------|------|
| **Server1** | First Madmail instance (typically a DNS domain) |
| **Server2** | Second Madmail instance (DNS domain or public IP) |
| **ExchangerHost** | Machine that runs Madexchanger |
| **TunnelPort** | Local port exposed on each Madmail host via reverse SSH (default `19080`) |

Examples use placeholder domains and RFC 5737 addresses only.

---

## 1. What Madexchanger does

Madexchanger is an HTTP mail relay for the Madmail / chatmail wire format:

```
POST /mxdeliv
X-Mail-From: sender@domain
X-Mail-To: recipient@domain
Content-Type: application/octet-stream

<RFC 822 message>
```

It accepts that request from an upstream Madmail server, decides the next hop from the recipient domain (or a fixed downstream URL), and posts the same envelope to the destination Madmail server.

### When you need a relay

| Situation | Direct federation | Via Madexchanger |
|-----------|-------------------|------------------|
| Both servers can reach each other | Works | Optional |
| NAT, firewall, or split routing blocks direct path | Fails | Works if ExchangerHost can reach both sides |
| Intentional isolation (block peer IPs for testing) | Fails | Works if traffic is forced through the relay |
| Destination cannot accept inbound push | Hard | Use pull queue (Phase C) |

### Delivery models

**Push–push (default, Phase A)**

```
Server1 ──POST /mxdeliv──► Exchanger ──POST /mxdeliv──► Server2
```

**Direct federation (Madmail only, no exchanger)**

```
Server1 ──POST /mxdeliv or SMTP──► Server2
```

**Pull (Phase C)**

```
Server1 ──POST /mxdeliv──► Exchanger (store)
Server2 ──GET /pull?domain=…──► Exchanger
Server2 ──POST /pull/ack──► Exchanger
```

| Model | Initiator | Status in this tree |
|-------|-----------|---------------------|
| Push–push | Upstream Madmail | Implemented and lab-tested |
| Allowlist filters | Operator config | Implemented and lab-tested |
| Pull queue | Downstream client / agent | Implemented and lab-tested |
| Peer list (`GET /peers`) | Operator / tooling | Implemented and lab-tested |

---

## 2. Phases

| Phase | Purpose | Pass criteria |
|-------|---------|---------------|
| **A** | Reliable push–push path, tunnels, Madmail `endpoint-cache` | Health on all hosts; bidirectional relay HTTP 200 |
| **B** | Restrict who may use the relay (`relay_mode` + filters) | 403 without filter; 200 with allow rule; restore `all` |
| **C** | Queue mail for later pull | Enqueue → list → ack → empty queue |
| **D** | Static peer directory | `GET /peers` returns a valid list |

Branch that contains the full stack: **`phase-c/pull-model`**.

---

## 3. Reference topology

```
                    ┌─────────────────────┐
                    │   ExchangerHost     │
                    │   madexchanger      │
                    │   0.0.0.0:19080     │
                    └──────────▲──────────┘
           reverse SSH -R      │      reverse SSH -R
        127.0.0.1:19080        │        127.0.0.1:19080
               │               │               │
        ┌──────┴──────┐               ┌───────┴──────┐
        │  Server1    │               │   Server2    │
        │  Madmail    │               │   Madmail    │
        │  domain1    │               │   domain2    │
        └─────────────┘               └──────────────┘
```

Optional isolation: on Server1, reject outbound traffic to Server2’s public IP (and the reverse on Server2) so only the exchanger path remains.

Requirements:

- Each Madmail host rewrites outbound federation for the peer domain to `http://127.0.0.1:19080/mxdeliv`.
- ExchangerHost can open outbound HTTPS/HTTP to both Madmail public endpoints for dynamic delivery.
- Reverse SSH publishes the exchanger on `127.0.0.1:TunnelPort` on each Madmail host.

---

## 4. Prerequisites

**ExchangerHost**

- Linux
- Go 1.24+ (or a containerized Go toolchain) to build
- SSH key access to Server1 and Server2
- Free listen port (default `19080`)

**Server1 / Server2**

- Running Madmail
- Working `madmail endpoint-cache` CLI
- Prefer a Madmail build that normalizes bare IP addresses (`user@1.2.3.4` vs `user@[1.2.3.4]`) if either side is IP-based

**Secrets**

Examples use:

```text
ADMIN_TOKEN=change-me-to-a-random-token
```

Replace in any real deployment.

---

## 5. Install Madexchanger on ExchangerHost

### 5.1 Clone and build

```bash
git clone <repository-url> madexchanger
cd madexchanger
git checkout phase-c/pull-model

CGO_ENABLED=0 go build -buildvcs=false -trimpath -o madexchanger ./cmd/madexchanger
```

### 5.2 Configuration

```bash
cp deploy/config.push-push.example.yml config.yml
```

Minimal example (dynamic routing + pull):

```yaml
listen: "0.0.0.0:19080"
receive_path: "/mxdeliv"
downstream_url: ""
forward_timeout: 60
skip_tls_verify: true
relay_mode: "all"
database_path: "madexchanger.db"
log_level: "info"
admin_web:
  enabled: true
  path: "/admin"
  token: "change-me-to-a-random-token"
pull:
  enabled: true
  on_failure: true
  domains:
    - "pull-test.invalid"
  path: "/pull"
  token: "change-me-to-a-random-token"
```

`downstream_url` empty means **dynamic** routing: for each recipient domain, try `https://<domain>/mxdeliv`, then `http://<domain>/mxdeliv`. Set `downstream_url` only when every message must go to one fixed next hop.

### 5.3 systemd (user unit)

```bash
mkdir -p ~/.config/systemd/user
cp deploy/systemd/madexchanger.service ~/.config/systemd/user/
cp deploy/systemd/madex-tunnels.service ~/.config/systemd/user/
cp deploy/systemd/madex-tunnels.timer ~/.config/systemd/user/
# Adjust WorkingDirectory / ExecStart if the install path is not ~/madexchanger

export XDG_RUNTIME_DIR=/run/user/$(id -u)
systemctl --user daemon-reload
systemctl --user enable --now madexchanger.service
systemctl --user enable --now madex-tunnels.timer
```

Check health:

```bash
curl -sS http://127.0.0.1:19080/health
```

Expect `"status":"ok"`. With pull enabled, expect `"pull_enabled":true`.

### 5.4 Reverse SSH tunnels

On ExchangerHost, `~/.ssh/config`:

```sshconfig
Host server1-mx
  HostName <Server1-address>
  User root
  IdentityFile ~/.ssh/id_ed25519_mx
  IdentitiesOnly yes
  ServerAliveInterval 30
  ExitOnForwardFailure yes

Host server2-mx
  HostName <Server2-address>
  User root
  IdentityFile ~/.ssh/id_ed25519_mx
  IdentitiesOnly yes
  ServerAliveInterval 30
  ExitOnForwardFailure yes
```

Start tunnels:

```bash
export MADEX_TUNNEL_HOSTS="server1:server1-mx server2:server2-mx"
export MADEX_PORT=19080
chmod +x deploy/scripts/keep-tunnels.sh
./deploy/scripts/keep-tunnels.sh
```

On each Madmail host:

```bash
curl -sS http://127.0.0.1:19080/health
```

---

## 6. Configure Madmail

### Server1 (`domain1.example`)

```bash
madmail endpoint-cache set domain2.example "http://127.0.0.1:19080/mxdeliv" "via madexchanger"

# If Server2 is address-literal only:
madmail endpoint-cache set 203.0.113.50 "http://127.0.0.1:19080/mxdeliv"
madmail endpoint-cache set "[203.0.113.50]" "http://127.0.0.1:19080/mxdeliv"

madmail endpoint-cache list
```

### Server2

```bash
madmail endpoint-cache set domain1.example "http://127.0.0.1:19080/mxdeliv" "via madexchanger"
madmail endpoint-cache list
```

Prefer a **full URL** as `TARGET_HOST` (`http://127.0.0.1:19080/mxdeliv`). Older Madmail builds mishandled bare `host:port` as IPv6.

Helper script on each Madmail host:

```bash
PEER_DOMAIN=domain2.example ./deploy/madmail-endpoint-cache.sh
```

### Optional isolation

Server1:

```bash
iptables -I OUTPUT -d <Server2-public-IP> -j REJECT --reject-with icmp-host-unreachable
```

Server2:

```bash
iptables -I OUTPUT -d <Server1-public-IP> -j REJECT --reject-with icmp-host-unreachable
```

Remove with the same rules and `-D`.

---

## 7. Phase tests

Run only after Madexchanger is up and tunnels answer on both Madmail hosts.

```bash
export EXCHANGER=http://127.0.0.1:19080
export TOKEN=change-me-to-a-random-token
export DOMAIN1=domain1.example
export DOMAIN2=domain2.example
export USER1=user1@domain1.example
export USER2=user2@domain2.example
```

### Phase A — push–push

| ID | Action | Pass |
|----|--------|------|
| A1 | Health on ExchangerHost | `"status":"ok"` |
| A2 | Health on Server1 and Server2 at `127.0.0.1:19080` | ok |
| A3 | Direct HTTPS Server1→Server2 if isolation is on | failure / HTTP 000 |
| A4 | POST `/mxdeliv` on Server1 tunnel toward `$USER2` | HTTP **200** |
| A5 | POST `/mxdeliv` on Server2 tunnel toward `$USER1` | HTTP **200** |
| A6 | ExchangerHost logs show successful forwards both ways | present |

Sample POST body (synthetic multipart encrypted message for chatmail PGP checks):

```bash
curl -sS -o /tmp/body -w "%{http_code}\n" -X POST http://127.0.0.1:19080/mxdeliv \
  -H "X-Mail-From: $USER1" \
  -H "X-Mail-To: $USER2" \
  -H "Content-Type: application/octet-stream" \
  --data-binary @- <<'EML'
From: user1@domain1.example
To: user2@domain2.example
Subject: phase-a-test
MIME-Version: 1.0
Content-Type: multipart/encrypted; protocol="application/pgp-encrypted"; boundary="b"

--b
Content-Type: application/pgp-encrypted

Version: 1

--b
Content-Type: application/octet-stream

-----BEGIN PGP MESSAGE-----
test
-----END PGP MESSAGE-----

--b--
EML
```

Automated smoke (from a host that can SSH to all three machines):

```bash
SERVER1_SSH=server1 SERVER2_SSH=server2 EXCHANGER_SSH=exchanger \
  ./deploy/scripts/e2e-push-push-smoke.sh
```

### Phase B — allowlist

On ExchangerHost:

```bash
# selected, no filters → 403
curl -sS -X POST "$EXCHANGER/api/admin" -H 'Content-Type: application/json' \
  -d "{\"method\":\"POST\",\"resource\":\"/admin/config\",\"headers\":{\"Authorization\":\"Bearer $TOKEN\"},\"body\":{\"relay_mode\":\"selected\"}}"

curl -sS -o /dev/null -w "%{http_code}\n" -X POST "$EXCHANGER/mxdeliv" \
  -H "X-Mail-From: a@x" -H "X-Mail-To: u@$DOMAIN1" --data-binary 'x'
# expect 403

# allow domain, then successful POST
curl -sS -X POST "$EXCHANGER/api/admin" -H 'Content-Type: application/json' \
  -d "{\"method\":\"POST\",\"resource\":\"/admin/filters\",\"headers\":{\"Authorization\":\"Bearer $TOKEN\"},\"body\":{\"enabled\":true,\"field\":\"domain\",\"pattern\":\"$DOMAIN1\",\"comment\":\"allow\"}}"

# restore
curl -sS -X POST "$EXCHANGER/api/admin" -H 'Content-Type: application/json' \
  -d "{\"method\":\"POST\",\"resource\":\"/admin/config\",\"headers\":{\"Authorization\":\"Bearer $TOKEN\"},\"body\":{\"relay_mode\":\"all\"}}"
```

Script:

```bash
MADEX_ADMIN_TOKEN=$TOKEN MADEX_BASE=$EXCHANGER \
  FILTER_DOMAIN=$DOMAIN1 \
  ./deploy/scripts/phase-b-filter-test.sh
```

### Phase C — pull queue

```bash
# C1 always-pull domain
curl -sS -w "%{http_code}\n" -X POST "$EXCHANGER/mxdeliv" \
  -H "X-Mail-From: a@src.test" -H "X-Mail-To: u@pull-test.invalid" \
  --data-binary "Subject: pull-test\n\nbody\n"
# expect 200; health shows queued_pull >= 1

# C2
curl -sS -H "Authorization: Bearer $TOKEN" \
  "$EXCHANGER/pull?domain=pull-test.invalid"

# C3
curl -sS -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"ids":[1]}' "$EXCHANGER/pull/ack"
```

Script:

```bash
MADEX_ADMIN_TOKEN=$TOKEN MADEX_BASE=$EXCHANGER \
  ./deploy/scripts/phase-c-pull-test.sh
```

### Phase D — peer list

```bash
curl -sS "$EXCHANGER/peers"
```

Expect JSON with `version` and a non-empty `peers` array.

---

## 8. HTTP API

| Method | Path | Description |
|--------|------|-------------|
| POST | `/mxdeliv` | Accept relay traffic (or enqueue for pull) |
| GET | `/health` | Process health and counters |
| GET | `/pull?domain=` | List queued messages (Bearer token) |
| POST | `/pull/ack` | Body `{"ids":[…]}`; remove after delivery |
| GET | `/peers` | Static peer directory |
| POST | `/api/admin` | Admin RPC (config, filters, rewrites, proxies) |

Admin config updates use **POST** on resource `/admin/config` (not PUT).

---

## 9. Troubleshooting

| Symptom | Likely cause | Action |
|---------|--------------|--------|
| Health fails on Server1/Server2 | Tunnel down | Run `keep-tunnels.sh`; check SSH keys |
| Relay returns 502 | ExchangerHost cannot reach destination | Outbound firewall, DNS, TLS |
| Relay returns 200 but mailbox empty | Madmail silent drop / PGP policy | Use encrypted body; fix IP address normalize on Madmail |
| `endpoint-cache` ignored | Bad target form | Use full URL `http://127.0.0.1:PORT/mxdeliv` |
| 403 under `selected` | No matching filter | Add domain filter or set `relay_mode=all` |
| Pull returns 401 | Wrong token | `Authorization: Bearer <token>` |

---

## 10. Release checklist

Do not call the lab green until all of the following hold:

1. Madexchanger unit is active on ExchangerHost.
2. `/health` is ok on ExchangerHost, Server1, and Server2 (via tunnel).
3. Optional: direct Server1↔Server2 path fails under isolation rules.
4. Phase A: both directions return HTTP 200; exchanger logs successful forwards.
5. Phase B: 403 without filter, 200 with filter; `relay_mode` restored to `all`.
6. Phase C: enqueue, list `count >= 1`, ack, then `count == 0`.
7. Phase D: `/peers` returns at least two entries.
8. After B and C, a Phase A-style POST still succeeds (no sticky `selected` mode).

### Lab reference run

Integrated run on three live hosts (operator lab):

| Phase | Result |
|-------|--------|
| A | PASS — health ×3, isolation 000, bidirectional 200 |
| B | PASS — 403 / 200 / restore `all` |
| C | PASS — queue / list / ack |
| D | PASS — peer list |
| A after C | PASS |

Stop at the first failure and fix that phase before continuing.

---

## 11. Repository layout

```text
deploy/
  config.push-push.example.yml
  madmail-endpoint-cache.sh
  scripts/
    keep-tunnels.sh
    e2e-push-push-smoke.sh
    phase-b-filter-test.sh
    phase-c-pull-test.sh
    admin-set-relay-mode.sh
  systemd/
    madexchanger.service
    madex-tunnels.service
    madex-tunnels.timer
docs/
  GUIDE.md          ← this file
  PHASES.md
  phase-a-….md … phase-d-….md
  general.md
  admin_api.md
```

---

## 12. Security notes

- Use strong admin and pull tokens outside disposable labs.
- Prefer `relay_mode=selected` plus explicit domain filters in production.
- The exchanger sees envelope headers and the message blob; content is usually OpenPGP end-to-end encrypted.
- Restrict reverse-SSH keys and remote forward permissions on Madmail hosts.

---

## 13. Follow-on work

1. Agent on Server1/Server2 that polls `/pull` and posts into local Madmail `/mxdeliv`.
2. Load peer data from a config file instead of the built-in list.
3. Run ExchangerHost on a stable VPS rather than a laptop when uptime matters.
4. Open a pull request against upstream only after review on your own fork.

---

*Aligned with branch `phase-c/pull-model` and a full three-host lab regression.*
