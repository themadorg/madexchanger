# راهنمای جامع Madexchanger — از معرفی تا اجرا و تست

این سند **منبع اصلی** برای فهم، نصب، پیکربندی و تست رلهٔ فدراسیون بین دو (یا چند) سرور Madmail از طریق Madexchanger است.

در تمام مثال‌ها از نام‌های خنثی استفاده می‌شود:

| نام | نقش |
|-----|-----|
| **Server1** | سرور Madmail اول (مثلاً chatmail با دامنهٔ DNS) |
| **Server2** | سرور Madmail دوم (مثلاً chatmail با IP عمومی) |
| **ExchangerHost** | میزبان اجرای Madexchanger (می‌تواند لپ‌تاپ یا VPS باشد) |
| **TunnelPort** | پورت محلی اکسچنجر روی localhost سرورها (پیش‌فرض `19080`) |

هیچ نام واقعی دیتاسنتر/اپراتور در مثال‌ها نیست.

---

## ۱. معرفی

### ۱.۱ Madexchanger چیست؟

**Madexchanger** یک **رلهٔ HTTP** برای پیام‌های سازگار با Madmail / chatmail است.

- پروتکل: همان `POST /mxdeliv` با هدرهای `X-Mail-From` / `X-Mail-To` و بدنهٔ RFC 822
- نقش: مثل روتر شبکه — پیام را می‌گیرد، مقصد را از دامنهٔ گیرنده می‌فهمد، و به `/mxdeliv` مقصد forward می‌کند

### ۱.۲ چرا به رله نیاز است؟

| سناریو | بدون اکسچنجر | با اکسچنجر |
|--------|----------------|------------|
| دو سرور مستقیم به هم می‌رسند | فدراسیون مستقیم HTTP/SMTP | اختیاری |
| یکی یا هر دو پشت NAT/فایروال | fail | مسیر از واسط |
| تست isolation (مسیر مستقیم عمداً بسته) | fail | فقط مسیر رله |
| یکی از دو طرف فقط inbound دارد | پیچیده | Pull (فاز C) |

### ۱.۳ مدل‌های تحویل

```
[Push–Push — پیش‌فرض و فاز A]
  Server1 ──POST /mxdeliv──► Exchanger ──POST /mxdeliv──► Server2

[فدراسیون مستقیم — بدون این پروژه]
  Server1 ──POST /mxdeliv یا SMTP──► Server2

[Pull — فاز C]
  Server1 ──POST /mxdeliv──► Exchanger (صف)
  Server2 ──GET /pull?domain=…──► Exchanger  (می‌گیرد و ack می‌کند)
```

| مدل | کی شروع می‌کند؟ | وضعیت در این repo |
|-----|------------------|-------------------|
| Push–Push | فرستنده هل می‌دهد | **پیاده و تست‌شده** |
| Allowlist (فیلتر) | اپراتور قوانین می‌گذارد | **پیاده و تست‌شده** |
| Pull (صف) | گیرنده/agent می‌کشد | **پیاده و تست‌شده** |
| Discovery (`/peers`) | کلاینت لیست peer می‌گیرد | **پیاده و تست‌شده** |

---

## ۲. فازها (A → D)

| فاز | هدف | تست کلیدی |
|-----|-----|-----------|
| **A** | رله push–push پایدار + تونل + endpoint-cache | health، isolation، forward دو طرفه |
| **B** | فقط ترافیک مجاز (`relay_mode=selected` + فیلتر) | 403 بدون فیلتر، 200 با فیلتر |
| **C** | صف pull وقتی push ممکن/مطلوب نیست | queue → list → ack |
| **D** | دایرکتوری peerها | `GET /peers` |

جزئیات فنی هر فاز: `docs/phase-*.md` و `docs/PHASES.md`.

---

## ۳. معماری مرجع آزمایشگاه

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

اختیاری: روی Server1 خروجی به IP عمومی Server2 را REJECT کنید
         و برعکس — تا فقط مسیر اکسچنجر کار کند.
```

- **Server1** با `endpoint-cache` برای `domain2` → `http://127.0.0.1:19080/mxdeliv`
- **Server2** با `endpoint-cache` برای `domain2`/`[ip]` → همان URL
- **ExchangerHost** به اینترنت/هر دو مقصد برای **خروجی** `/mxdeliv` دسترسی دارد

---

## ۴. پیش‌نیازها

### ExchangerHost
- Linux، Docker یا Go 1.24+ برای build
- SSH key به Server1 و Server2 (برای reverse tunnel)
- پورت آزاد `19080` (یا هر `TunnelPort`)

### Server1 / Server2
- Madmail در حال اجرا (ترجیحاً با پچ normalize IP اگر دامنهٔ IP دارید)
- `madmail endpoint-cache` در دسترس
- پورت‌های mail/HTTPS طبق نصب خودتان

### توکن آزمایشگاه
در مثال‌ها:

```text
ADMIN_TOKEN=change-me-to-a-random-token
```

در production حتماً عوض شود.

---

## ۵. نصب و اجرا — ExchangerHost

### ۵.۱ کلون و build

```bash
git clone <your-fork-or-upstream>/madexchanger.git
cd madexchanger
git checkout phase-c/pull-model   # کامل‌ترین برنچ (A+B+C+D)

# Build
CGO_ENABLED=0 go build -buildvcs=false -trimpath -o madexchanger ./cmd/madexchanger
```

### ۵.۲ کانفیگ

```bash
cp deploy/config.push-push.example.yml config.yml
# ویرایش: token، listen، pull.domains در صورت نیاز
```

نمونهٔ حداقل (dynamic routing + pull):

```yaml
listen: "0.0.0.0:19080"
receive_path: "/mxdeliv"
downstream_url: ""          # خالی = dynamic
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

### ۵.۳ systemd (user)

```bash
mkdir -p ~/.config/systemd/user
cp deploy/systemd/madexchanger.service ~/.config/systemd/user/
cp deploy/systemd/madex-tunnels.service ~/.config/systemd/user/
cp deploy/systemd/madex-tunnels.timer ~/.config/systemd/user/
# در unitها مسیر %h/madexchanger را با مسیر واقعی یکی کنید

export XDG_RUNTIME_DIR=/run/user/$(id -u)
systemctl --user daemon-reload
systemctl --user enable --now madexchanger.service
systemctl --user enable --now madex-tunnels.timer
```

Health:

```bash
curl -sS http://127.0.0.1:19080/health
# انتظار: "status":"ok" و در صورت فعال بودن pull: "pull_enabled":true
```

### ۵.۴ Reverse SSH از ExchangerHost

فایل `~/.ssh/config` روی ExchangerHost:

```sshconfig
Host server1-mx
  HostName <IP-or-DNS-Server1>
  User root
  IdentityFile ~/.ssh/id_ed25519_mx
  IdentitiesOnly yes
  ServerAliveInterval 30
  ExitOnForwardFailure yes

Host server2-mx
  HostName <IP-or-DNS-Server2>
  User root
  IdentityFile ~/.ssh/id_ed25519_mx
  IdentitiesOnly yes
  ServerAliveInterval 30
  ExitOnForwardFailure yes
```

```bash
export MADEX_TUNNEL_HOSTS="server1:server1-mx server2:server2-mx"
export MADEX_PORT=19080
./deploy/scripts/keep-tunnels.sh
# یا: MADEX_TUNNEL_HOSTS=... در unit تونل
```

روی هر سرور Madmail:

```bash
curl -sS http://127.0.0.1:19080/health
```

---

## ۶. پیکربندی Madmail (Server1 / Server2)

### Server1 (دامنهٔ `domain1.example`)

```bash
madmail endpoint-cache set domain2.example "http://127.0.0.1:19080/mxdeliv" "via madexchanger"
# اگر Server2 فقط IP دارد:
madmail endpoint-cache set 203.0.113.50 "http://127.0.0.1:19080/mxdeliv"
madmail endpoint-cache set "[203.0.113.50]" "http://127.0.0.1:19080/mxdeliv"
madmail endpoint-cache list
```

### Server2

```bash
madmail endpoint-cache set domain1.example "http://127.0.0.1:19080/mxdeliv" "via madexchanger"
madmail endpoint-cache list
```

**نکته:** برای `TARGET_HOST` ترجیحاً **URL کامل** (`http://127.0.0.1:19080/mxdeliv`) بگذارید.  
اگر Madmail قدیمی دارید، پچ `host:port` و normalize IP را از پروژه madmail اعمال کنید.

### اختیاری — isolation (مسیر مستقیم بسته)

روی Server1:

```bash
iptables -I OUTPUT -d <PUBLIC_IP_SERVER2> -j REJECT --reject-with icmp-host-unreachable
```

روی Server2:

```bash
iptables -I OUTPUT -d <PUBLIC_IP_SERVER1> -j REJECT --reject-with icmp-host-unreachable
```

برداشتن: همان قوانین با `-D`.

---

## ۷. راهنمای تست فازبه‌فاز

همهٔ دستورات زیر را بعد از بالا بودن سرویس و تونل اجرا کنید.  
متغیرها:

```bash
export EXCHANGER=http://127.0.0.1:19080   # از ExchangerHost یا از تونل روی Server*
export TOKEN=change-me-to-a-random-token
export DOMAIN1=domain1.example
export DOMAIN2=domain2.example            # یا IP / [IP]
export USER1=user1@domain1.example
export USER2=user2@domain2.example
```

### فاز A — Push–Push

| # | کار | معیار قبولی |
|---|-----|-------------|
| A1 | `curl $EXCHANGER/health` روی ExchangerHost | `"status":"ok"` |
| A2 | همان از Server1 و Server2 روی `127.0.0.1:19080` | ok |
| A3 | HTTPS مستقیم Server1→Server2 (اگر isolation) | fail / code 000 |
| A4 | POST از Server1 به localhost exchanger با `X-Mail-To: $USER2` | HTTP **200** |
| A5 | POST از Server2 با `X-Mail-To: $USER1` | HTTP **200** |
| A6 | لاگ ExchangerHost: `email forwarded successfully` | دو طرف دیده شود |

نمونهٔ POST (multipart encrypted ساختگی برای عبور از PGP gate):

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

اسکریپت آماده:

```bash
# از ماشینی که SSH به هر سه دارد:
SERVER1_SSH=server1 SERVER2_SSH=server2 EXCHANGER_SSH=exchanger \
  ./deploy/scripts/e2e-push-push-smoke.sh
```

*(اسکریپت را با env به hostهای SSH خودتان map کنید؛ پیش‌فرض‌های قدیمی را در env override کنید.)*

### فاز B — Allowlist

روی **ExchangerHost**:

```bash
# selected بدون فیلتر → 403
curl -sS -X POST $EXCHANGER/api/admin -H 'Content-Type: application/json' \
  -d "{\"method\":\"POST\",\"resource\":\"/admin/config\",\"headers\":{\"Authorization\":\"Bearer $TOKEN\"},\"body\":{\"relay_mode\":\"selected\"}}"

curl -sS -o /dev/null -w "%{http_code}\n" -X POST $EXCHANGER/mxdeliv \
  -H "X-Mail-From: a@x" -H "X-Mail-To: u@$DOMAIN1" --data-binary 'x'
# انتظار: 403

# افزودن فیلتر و تست 200
curl -sS -X POST $EXCHANGER/api/admin -H 'Content-Type: application/json' \
  -d "{\"method\":\"POST\",\"resource\":\"/admin/filters\",\"headers\":{\"Authorization\":\"Bearer $TOKEN\"},\"body\":{\"enabled\":true,\"field\":\"domain\",\"pattern\":\"$DOMAIN1\",\"comment\":\"allow\"}}"

# ... POST موفق به دامنه مجاز ...

# cleanup
curl -sS -X POST $EXCHANGER/api/admin -H 'Content-Type: application/json' \
  -d "{\"method\":\"POST\",\"resource\":\"/admin/config\",\"headers\":{\"Authorization\":\"Bearer $TOKEN\"},\"body\":{\"relay_mode\":\"all\"}}"
```

اسکریپت: `MADEX_ADMIN_TOKEN=$TOKEN ./deploy/scripts/phase-b-filter-test.sh`

### فاز C — Pull

```bash
# C1 — دامنه always-pull
curl -sS -w "%{http_code}\n" -X POST $EXCHANGER/mxdeliv \
  -H "X-Mail-From: a@src.test" -H "X-Mail-To: u@pull-test.invalid" \
  --data-binary "Subject: pull-test\n\nbody\n"
# 200 و health: queued_pull >= 1

# C2
curl -sS -H "Authorization: Bearer $TOKEN" \
  "$EXCHANGER/pull?domain=pull-test.invalid"

# C3
curl -sS -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"ids":[1]}' "$EXCHANGER/pull/ack"
```

اسکریپت: `MADEX_ADMIN_TOKEN=$TOKEN ./deploy/scripts/phase-c-pull-test.sh`

### فاز D — Discovery

```bash
curl -sS $EXCHANGER/peers
# JSON: version + peers[]
```

---

## ۸. API خلاصه

| متد | مسیر | توضیح |
|-----|------|--------|
| POST | `/mxdeliv` | دریافت و رله (یا صف pull) |
| GET | `/health` | وضعیت + شمارنده‌ها |
| GET | `/pull?domain=` | لیست صف pull (Bearer) |
| POST | `/pull/ack` | `{"ids":[…]}` حذف از صف |
| GET | `/peers` | لیست peerهای آزمایشگاه |
| POST | `/api/admin` | Admin RPC (config, filters, …) |

---

## ۹. عیب‌یابی

| علامت | علت محتمل | کار |
|-------|-----------|-----|
| health روی Server* fail | تونل قطع | `keep-tunnels.sh` / SSH key |
| forward 502 | ExchangerHost به مقصد نمی‌رسد | فایروال خروجی، DNS، TLS |
| 200 از exchanger ولی mailbox خالی | silent-drop Madmail / PGP | پچ normalize IP؛ پیام encrypted |
| endpoint-cache بی‌اثر | URL ناقص / باگ host:port | URL کامل `http://127.0.0.1:PORT/mxdeliv` |
| 403 در selected | فیلتر نیست | فیلتر domain یا `relay_mode=all` |
| pull 401 | توکن اشتباه | `Authorization: Bearer …` |

---

## ۱۰. چک‌لیست اطمینان از پاس بودن تست‌ها

قبل از اعلام «سبز»، همه باید برقرار باشند:

- [ ] `madexchanger` روی ExchangerHost `active`
- [ ] `/health` از ExchangerHost، Server1، Server2 یکسان `status=ok`
- [ ] (اختیاری) مستقیم Server1↔Server2 fail
- [ ] A4 و A5 هر دو HTTP 200 + لاگ `forwarded successfully` دو طرفه
- [ ] B: 403 سپس 200 با فیلتر؛ در پایان `relay_mode=all`
- [ ] C: queue → list count≥1 → ack → count=0
- [ ] D: `/peers` حداقل دو peer برمی‌گرداند
- [ ] بعد از تست‌های B، حالت سرویس `all` مانده و push–push هنوز کار می‌کند

### نتیجهٔ مرجع آزمایشگاه (اجرای کامل روی سه میزبان واقعی)

در آخرین اجرای یکپارچه روی میزبان‌های آزمایش:

| فاز | نتیجه |
|-----|--------|
| A | PASS — health×3، isolation 000، forward دو طرفه 200 |
| B | PASS — 403 / 200 / restore all |
| C | PASS — queue / list / ack |
| D | PASS — peers list |
| رگرسیون A بعد از C | PASS |

اگر هر موردی fail شود، تا سبز شدن همان فاز جلو نروید.

---

## ۱۱. ساختار repo مربوط به deploy

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
  GUIDE.md          ← این فایل
  PHASES.md
  phase-a-….md … phase-d-….md
  general.md
  admin_api.md
```

---

## ۱۲. امنیت

- توکن admin و pull را قوی بگذارید؛ در lab عمومی نگذارید.
- در production: `relay_mode=selected` + فیلتر دامنه/فرستنده.
- ExchangerHost کل envelope را می‌بیند (محتوا معمولاً PGP end-to-end است).
- Reverse SSH را با کلید محدود و `AllowTcpForwarding` کنترل کنید.

---

## ۱۳. گام‌های بعدی پیشنهادی

1. Agent روی Server* که `/pull` را poll کند و به madmail محلی `/mxdeliv` بدهد  
2. `peers.yml` به‌جای لیست hardcode  
3. انتقال ExchangerHost از لپ‌تاپ به VPS پایدار  
4. Push برنچ‌ها به fork خودتان (upstream themadorg فقط در صورت PR)

---

*آخرین هم‌ترازسازی با برنچ `phase-c/pull-model` و تست end-to-end روی سه میزبان آزمایشگاهی.*
