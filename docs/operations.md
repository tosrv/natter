# pmess Operations Guide

> Versi Indonesia: [operations.id.md](operations.id.md) · Back to the [README](../README.md)

## Prerequisites

- Go 1.24+
- Node.js 22+ and npm
- Docker + Docker Compose v2 (for Postgres, the app image, and testcontainers integration tests)

## Google OAuth setup

1. Open [Google Cloud Console](https://console.cloud.google.com/) → create or pick a project.
2. **APIs & Services → OAuth consent screen**: fill in the app name and support email; the `openid` and `email` scopes are enough. Add test users while the app is in *Testing*.
3. **APIs & Services → Credentials → Create credentials → OAuth client ID**, type **Web application**.
4. **Authorized redirect URIs**: exactly the value of `OAUTH_REDIRECT_URL`, for example
   - production: `https://chat.example.com/api/auth/google/callback`
   - local: `http://localhost:8080/api/auth/google/callback`
5. Copy the **Client ID** and **Client secret** into `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET`.

## Environment variables

Copy `.env.example` to `.env` and fill it in. Never commit `.env`.

| Variable | Required | Default | Notes |
|---|---|---|---|
| `DATABASE_URL` | yes | – | Postgres URL, e.g. `postgres://pmess:PASSWORD@localhost:5432/pmess?sslmode=disable`. Built automatically from `POSTGRES_PASSWORD` under `docker compose` |
| `POSTGRES_PASSWORD` | yes (compose) | – | Postgres password for `docker compose`; compose refuses to run without it |
| `GOOGLE_CLIENT_ID` | yes (Google login) | – | From Google Cloud Console |
| `GOOGLE_CLIENT_SECRET` | yes (Google login) | – | Secret — never commit it |
| `OAUTH_REDIRECT_URL` | yes (Google login) | – | Must equal the Authorized redirect URI |
| `DEVELOPER_EMAIL` | yes | – | Google email of the developer account (receives guest messages) |
| `ALLOWED_ORIGINS` | no | empty | Comma-separated origins for CORS/WebSocket, e.g. `https://chat.example.com` |
| `COOKIE_SECURE` | no | `false` | Set `true` in production (HTTPS) |
| `LISTEN_ADDR` | no | `:8080` | HTTP listen address |
| `TRUSTED_PROXY` | no | `127.0.0.1` (`172.28.0.1` in compose) | Peer IP trusted for the `X-Real-IP` header (your Nginx). Compose gives the default network the fixed subnet `172.28.0.0/16` with gateway `172.28.0.1`, so Nginx on the host appears as `172.28.0.1` — the compose default is already correct. The server logs one WARN when `X-Real-IP` arrives from an untrusted peer |
| `STATIC_DIR` | no | `web/dist` | Frontend build folder (`/app/web` in the Docker image) |
| `TURNSTILE_SITE_KEY` | no | empty | Cloudflare Turnstile site key (guest captcha). Leave **both** keys empty to disable; setting only one makes the server refuse to start |
| `TURNSTILE_SECRET_KEY` | no | empty | Turnstile secret key — never commit it |
| `E2E_FAKE_LOGIN` | no | `0` | **E2E tests only.** `1` enables `POST /api/test/login` |

> ⚠️ **Never set `E2E_FAKE_LOGIN=1` in production.** That route lets anyone log in as anyone. The server prints a loud warning when it is on, and **refuses to start** when `E2E_FAKE_LOGIN=1` is combined with `COOKIE_SECURE=true`.

## Cloudflare Turnstile (guest captcha)

The "Continue as guest" button is protected by Turnstile; the server verifies the token before the rate limiter and before taking a slot. If Cloudflare is unreachable, guest sign-in fails closed (503 `captcha_unavailable`).

1. Cloudflare dashboard → **Turnstile** → **Add widget**.
2. Name it (e.g. `pmess`), hostname `chat.rahmattomy.com`, mode **Managed**.
3. Copy the **site key** and **secret key** into `.env` on the VPS (`TURNSTILE_SITE_KEY`, `TURNSTILE_SECRET_KEY`).
4. Run `docker compose up -d` so the app container picks up the new env.

For local dev/E2E leave both empty (captcha off). Cloudflare's always-pass test keys: site `1x00000000000000000000AA`, secret `1x0000000000000000000000000000000AA`. The Playwright test for the real widget runs only with `E2E_TURNSTILE=1` and needs access to `challenges.cloudflare.com` (CI sets this up).

## Embeddable widget (host sites)

To let a website embed `<pmess-chat>` (see [`widget/README.md`](../widget/README.md)), for each host site:

1. Add `https://<site>` to `ALLOWED_ORIGINS` in the VPS `.env` (comma-separated), then run `docker compose up -d` to restart the app with the new env.
2. Cloudflare dashboard → **Turnstile** → the pmess widget → add the site's hostname.
3. Verify the script is served cross-origin: `curl -I https://chat.rahmattomy.com/widget.js` must show `Cross-Origin-Resource-Policy: cross-origin`.
4. If the host site has a Content-Security-Policy, it must allow `script-src https://chat.rahmattomy.com https://challenges.cloudflare.com`, `connect-src https://chat.rahmattomy.com wss://chat.rahmattomy.com` and `frame-src https://challenges.cloudflare.com`.

## Security headers

The server adds these to every response: `Content-Security-Policy`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, `Permissions-Policy: camera=(), microphone=(), geolocation=()`, `X-Frame-Options: DENY`, and `Strict-Transport-Security: max-age=31536000` only when `COOKIE_SECURE=true`. **Do not add the same headers in Nginx** (`add_header`), or they will be duplicated; the policy lives in `internal/api/security.go`.

## Blocking users

Registered users can block a peer from the conversation menu (and unblock from Settings). Messages and opening a conversation between block-related users (whoever blocked whom) return 404 `not_found`; neither appears in the other's search; typing/read indicators are not forwarded. Only the blocker sees a "Blocked" label — the blocked person is never told and only ever gets a generic error. Guests cannot block (403); the developer can block guests. Block/unblock are idempotent.

## Running locally (dev mode)

```bash
# 1. Postgres (the dev file publishes 127.0.0.1:5432)
echo POSTGRES_PASSWORD=pmess >> .env
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d postgres

# 2. Backend (terminal 1)
export DATABASE_URL='postgres://pmess:pmess@localhost:5432/pmess?sslmode=disable'
export TRUSTED_PROXY=127.0.0.1
export DEVELOPER_EMAIL=you@example.com
export GOOGLE_CLIENT_ID=... GOOGLE_CLIENT_SECRET=...
export OAUTH_REDIRECT_URL=http://localhost:8080/api/auth/google/callback
go run ./cmd/server          # DB migrations run automatically

# 3. Frontend (terminal 2)
cd web && npm install && npm run dev
```

Open the URL Vite prints (it proxies `/api` to `:8080`), or run `npm run build` and open `http://localhost:8080` directly.

## Running with Docker

```bash
cp .env.example .env   # fill it in; POSTGRES_PASSWORD is required (DATABASE_URL is built from it)
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build -d   # local build; prod compose pulls from GHCR
# app on http://127.0.0.1:8080 (localhost only; public through Nginx). Postgres is not published to the host.
docker compose logs -f app
```

The image is multi-stage (Node 22 → Go 1.24 `CGO_ENABLED=0` → `debian:bookworm-slim` + `postgresql-client-16`, non-root uid 65532) and contains the binary plus static files in `/app/web`.

**Dev/sandbox only:** `docker-compose.dev.yml` publishes Postgres on `127.0.0.1:5432` and adds an `extra_ca` build secret (CA bundle for TLS-intercepting proxies). Use `EXTRA_CA_FILE=./ca.pem docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build -d`. Do not use it in production; without the secret the Dockerfile mount has no effect.

## Verification

Everything can be run yourself. Docker must be running for the `store` tests (testcontainers).

```bash
# All Go tests with the race detector
go test -race ./...

# By acceptance criterion
go test -race ./internal/chat/...
go test -race ./internal/store/...
go test -race -run TestConcurrentSend ./...     # 50 concurrent senders
go test -race -run TestResend ./...             # reconnect / resend
go test -race -run TestSlotRace ./...           # the 300th slot
go test -race -run TestAuthz ./internal/api/... # authorization (404)
go test -race ./internal/api/...                # XSS-safe text, rate limit, length limit
go test -race -run TestRetention ./internal/jobs/... # retention

# Frontend unit tests
cd web && npm test
```

**Load test (latency < 1 s)** — needs a running server and Postgres reachable from the host (dev stack above):

```bash
DATABASE_URL='postgres://pmess:pmess@localhost:5432/pmess?sslmode=disable' go run ./cmd/loadtest
# prints p50/p95/max; exits 0 with "OK" when no message was lost or duplicated
```

The load test sends a different `X-Real-IP` per client so the per-IP rate limit doesn't apply; the server must trust its peer (`TRUSTED_PROXY`). For a local `go run ./cmd/server` the default `127.0.0.1` is enough; under Docker the compose default `172.28.0.1` (the gateway) already matches.

**Playwright E2E** — two browsers chatting, guest flow, notifications, language toggle, blocking, legal pages, CSP:

```bash
printf 'E2E_FAKE_LOGIN=1\nCOOKIE_SECURE=false\nDEVELOPER_EMAIL=dev@e2e.test\nPOSTGRES_PASSWORD=test\n' > .env
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build -d
cd web
npm install
npx playwright install chromium   # once
E2E_BASE_URL=http://localhost:8080 npm run e2e
cd .. && docker compose -f docker-compose.yml -f docker-compose.dev.yml down -v && rm .env
```

## Nginx (production)

A complete server block is in [`deploy/nginx.conf.example`](../deploy/nginx.conf.example): HTTP→HTTPS redirect, TLS (replace the certificate paths), `location /api/ws` with `Upgrade`/`Connection` headers, `proxy_read_timeout 75s` (WS pings every 25 s), and `X-Real-IP $remote_addr` (used by the rate limiter; make sure `TRUSTED_PROXY` matches). In production also set `COOKIE_SECURE=true` and `ALLOWED_ORIGINS=https://your-domain`.

## Operations

Flow: push to `main` → CI (tests + backup-test) → image `ghcr.io/tosrv/pmess` (`latest` and `sha-<7 chars>`) → Watchtower on the VPS pulls it automatically. `docker-compose.yml` is the production compose file (image from GHCR, no `build`).

### GHCR setup (once)

After the first successful `image` job: GitHub → profile/organization → **Packages** → `pmess` → **Package settings** → **Change visibility** → **Public**. The VPS can then pull without logging in. The image contains no secrets (all config comes from `.env`).

### First deploy on the VPS

```bash
git clone https://github.com/tosrv/pmess.git && cd pmess
cp .env.example .env            # fill in POSTGRES_PASSWORD, GOOGLE_*, DEVELOPER_EMAIL, etc.
sudo install -d -o 65532 -g 65532 -m 700 /var/backups/pmess   # uid 65532 = the user inside the container
docker compose up -d
docker compose ps               # app must be healthy
curl -fsS http://127.0.0.1:8080/healthz
```

Install Nginx from `deploy/nginx.conf.example` (it already forwards `/healthz`).

### How Watchtower works

The `watchtower` service (`nickfedor/watchtower`, version pinned in compose; an active fork of the archived `containrrr/watchtower`) checks the registry every 300 seconds. Only containers labelled `com.centurylinklabs.watchtower.enable=true` are updated, which is just `app`; `postgres` and `backup` are not. Before replacing `app`, Watchtower runs the hook `/app/deploy/backup.sh preupdate` inside the `app` container (a `preupdate` backup). Logs: `docker compose logs -f watchtower`. DB migrations run automatically when `app` starts.

Note: the `backup` container uses the same image but is not updated automatically; it picks up a new version when you run `docker compose pull && docker compose up -d backup` (now and then).

### Rollback

Set `PMESS_TAG=sha-<old version>` in `.env`, then `docker compose up -d app backup`. While `PMESS_TAG` isn't `latest`, Watchtower doesn't update `app` (the tag stays the same). Set `PMESS_TAG=latest` to follow updates again. Rolling back the image does **not** undo migrations; if a migration is the problem, restore from the `preupdate` backup.

### Backups

- **Location:** `/var/backups/pmess` on the host (change with `BACKUP_DIR`), mounted in the containers as `/backups`. Create it with `sudo install -d -o 65532 -g 65532 -m 700 /var/backups/pmess`.
- **Schedule:** the `backup` service runs `pg_dump -Fc` daily at 02:00 in `TZ` (default `UTC`); Sunday's run is labelled `weekly`. Watchtower creates a `preupdate` backup before every update.
- **File names:** `pmess-YYYYMMDD-HHMM-<daily|weekly|preupdate>.dump`.
- **Retention:** 7 `daily`, 4 `weekly`, 5 `preupdate`; older ones are deleted only after a new backup succeeds.
- **Failed backup:** logged as JSON with `level=ERROR` (`docker compose logs backup`); there is no separate alert.
- **Copy to your own machine (manual, do it regularly):**

```bash
scp 'user@vps:/var/backups/pmess/pmess-*.dump' ./backup-pmess/
```

> **Risk:** backups live only on the VPS. If the VPS is lost or compromised, the backups go with it. Copy them off regularly with the `scp` above.

### Restore

Run on the host from the repo root. Order: stop `watchtower` (restarted when the script ends, even on failure) → safety `preupdate` dump of the current state (if `app` is running) → stop `app` → `pg_restore --single-transaction` (a failure rolls back fully; the old database stays intact) → start `app` → wait up to 60 seconds for `/healthz`. The `backup` service is not touched.

```bash
deploy/restore.sh /var/backups/pmess/pmess-YYYYMMDD-HHMM-daily.dump --yes
```

If the safety dump fails, the script stops before touching anything; pass `--no-safety-dump` as a third argument (`... --yes --no-safety-dump`) to skip it. Without `--yes` the script refuses to run. Use `sudo` when the backup folder is readable only by uid 65532.

### Fixed network subnet

Compose uses the fixed subnet `172.28.0.0/16` (gateway `172.28.0.1`). If it clashes with an existing network on the VPS (`docker network ls`, `ip route`), `docker compose up` fails with "Pool overlaps". Change the `networks.default.ipam.config` block in `docker-compose.yml` (e.g. `subnet: 172.29.0.0/16`, `gateway: 172.29.0.1`) **and** set `TRUSTED_PROXY` in `.env` to the new gateway (`TRUSTED_PROXY=172.29.0.1`); the two must match so `X-Real-IP` from Nginx is trusted. Then `docker compose down && docker compose up -d`.

### Alerts (UptimeRobot)

1. Create a free account at uptimerobot.com.
2. **Add New Monitor** → type **HTTP(s)**, URL `https://<domain>/healthz`, interval **5 minutes**.
3. **Alert Contacts**: add your email; notify after 2 consecutive failures and on recovery.
4. Test it once: `docker compose stop app`, wait for the email (up to ~10 minutes), then `docker compose start app` and confirm the recovery email arrives.

## Privacy

- Messages are **not** end-to-end encrypted; a server admin can read them in the database.
- A user's email is never sent to other users; message bodies are always rendered as plain text.
- A guest session ends 30 minutes after its conversation's last activity, on logout, or on "End session". A guest with messages is deactivated (kept, unusable, username cleared); a guest without messages is deleted. The developer still sees the conversation as "Session ended" and cannot reply.
- Conversations are deleted after 1 month without activity; the daily job (03:00) then purges deactivated guests that have no conversation left.
- Contact info a guest enters is voluntary and is kept with the conversation (up to 1 month), then deleted with it.
