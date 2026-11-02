# Ops Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** CI di GitHub Actions yang menguji lalu memublikasikan image ke GHCR; VPS memperbarui diri lewat Watchtower dengan backup sebelum update; backup harian + restore teruji; `/healthz` dan log JSON.

**Architecture:** Server Go mendapat `/healthz` dan middleware log JSON. Image app berbasis `debian:bookworm-slim` + `postgresql-client` agar bisa `pg_dump`. Skrip shell di `deploy/` menangani backup, retensi, dan restore. `docker-compose.yml` prod menarik image GHCR dan menjalankan service `backup` dan `watchtower`. `.github/workflows/ci.yml` menjalankan go-test, web-test, e2e, backup-test, lalu image.

**Tech Stack:** Go 1.24 (`log/slog`, Gin), Docker Compose, `postgres:16`, `nicholas-fedor/watchtower` (versi dikunci), GitHub Actions, GHCR, bash.

**Spec:** `docs/superpowers/specs/2026-11-02-ops-foundation-design.md`

## Global Constraints

- Image: `ghcr.io/tosrv/pmess`, tag `latest` dan `sha-<7 karakter SHA>`; publik; push dengan `GITHUB_TOKEN` (`permissions: packages: write`).
- Job `image` hanya untuk `push` ke `main` dan `needs: [go-test, web-test, e2e, backup-test]`; PR menjalankan test saja.
- Watchtower: image `nickfedor/watchtower:<versi terkunci>` (cek tag terbaru yang stabil di GitHub `nicholas-fedor/watchtower` dan Docker Hub; jangan `latest`); flag `--interval 300 --label-enable --cleanup --enable-lifecycle-hooks`; hanya `app` berlabel `com.centurylinklabs.watchtower.enable=true`; `postgres` dan `backup` tidak berlabel.
- Pre-update hook: label `com.centurylinklabs.watchtower.lifecycle.pre-update=/app/deploy/backup.sh preupdate` pada `app`.
- Backup: `pg_dump -Fc`, file `pmess-YYYYMMDD-HHMM-<daily|weekly|preupdate>.dump` di `BACKUP_DIR` (container: `/backups`, host default `/var/backups/pmess`); harian 02:00 waktu `TZ` (default `UTC`); hari Minggu → `weekly`; retensi 7 daily, 4 weekly, 5 preupdate; hapus file lama hanya setelah dump sukses; dump gagal → exit ≠ 0 dan pesan `ERROR` JSON satu baris ke stderr, file parsial dihapus.
- Restore: `deploy/restore.sh <file>`: stop app → `pg_restore --clean --if-exists --no-owner -d pmess` → start app → tunggu `/healthz` 200 (maks 60 s).
- `/healthz`: `GET`, di luar `/api`, tanpa auth/rate limit; `pool.Ping` timeout 2 s → `200 {"status":"ok"}` / `503 {"status":"unavailable"}`.
- Log: `slog.NewJSONHandler(os.Stdout)`, level dari `LOG_LEVEL` (`debug|info|warn|error`, default `info`, nilai tak dikenal → `info`). Request log: `method`, `path` (tanpa query), `status`, `duration_ms`, `request_id` (dari `X-Request-Id` atau baru 16 hex; dikembalikan di header respons); `/healthz` di level debug. Tidak pernah mencatat body, query string, `Authorization`, `Cookie`, email, token.
- Docker logging semua service: driver `json-file`, `max-size: "10m"`, `max-file: "5"`.
- Healthcheck `app`: memanggil `/healthz` tiap 30 s, 3 kali percobaan.
- Env baru: `PMESS_TAG` (default `latest`), `LOG_LEVEL` (default `info`), `BACKUP_DIR` (default `/var/backups/pmess`), `TZ` (default `UTC`).
- Kerja di branch `feat/ops`; commit + push per task (`git push -u origin feat/ops`); trailer commit: `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>` dan `Claude-Session: https://claude.ai/code/session_017NqdLCXr2x4X6KWjddprEM`.
- Bila push file di `.github/workflows/` ditolak proxy, laporkan sebagai BLOCKED_PUSH; jangan ubah history.

## Review Focus

1. **`/healthz` saat database mati** → 503 cepat (≤ 2.5 s), bukan menggantung atau 500. (Test di Task 1.)
2. **Request dengan token di query string atau header `Authorization`** → tidak muncul di log. (Test di Task 1.)
3. **`pg_dump` gagal di tengah jalan** (DB tidak terjangkau) → tidak ada file `.dump` parsial tertinggal, file lama tidak terhapus, exit ≠ 0. (Test di Task 2.)
4. **Retensi dengan file campuran** (10 daily, 6 weekly, 7 preupdate, plus file asing `notes.txt`) → tersisa tepat 7/4/5 file terbaru per jenis, file asing tidak disentuh. (Test di Task 2.)
5. **Restore ke database yang sudah berisi data berbeda** → hasil akhir sama persis dengan isi backup (tidak tercampur). (Test di Task 2 `test-backup.sh`.)

---

## File Structure

```
internal/api/health.go            /healthz handler
internal/api/requestlog.go        middleware log JSON
internal/logging/logging.go       NewLogger(level string) *slog.Logger
cmd/server/main.go                pakai NewLogger, pasang middleware + /healthz
Dockerfile                        base debian:bookworm-slim + postgresql-client + deploy/*.sh
deploy/backup.sh                  backup <daily|preupdate> + retensi
deploy/restore.sh                 restore <file>
deploy/backup-cron.sh             loop jadwal harian untuk service backup
deploy/test-backup.sh             uji siklus backup→hapus→restore + retensi (dipakai CI)
docker-compose.yml                prod: image GHCR, backup, watchtower, logging, healthcheck
docker-compose.dev.yml            dev: build lokal
deploy/nginx.conf.example         location /healthz
.github/workflows/ci.yml          CI
README.md, .env.example           dokumentasi ops
```

---

### Task 1: `/healthz` dan log JSON

**Files:**
- Create: `internal/logging/logging.go`, `internal/api/health.go`, `internal/api/requestlog.go`
- Modify: `internal/api/router.go`, `cmd/server/main.go`, `internal/config/config.go` (field `LogLevel`)
- Test: `internal/logging/logging_test.go`, `internal/api/health_test.go`, `internal/api/requestlog_test.go`

**Interfaces:**
- Produces:
  - `logging.NewLogger(w io.Writer, level string) *slog.Logger` — JSON handler; level map `debug|info|warn|error`, lainnya → info.
  - `api.Health(p interface{ Ping(context.Context) error }) gin.HandlerFunc`.
  - `api.RequestLog(l *slog.Logger) gin.HandlerFunc`.
  - `api.Deps` mendapat `Pinger interface{ Ping(context.Context) error }` dan `Logger *slog.Logger`; router mendaftarkan `GET /healthz` di root (sebelum static fallback) dan `RequestLog` global.
  - `config.Config.LogLevel string` (env `LOG_LEVEL`, default `info`).

- [ ] **Step 1: Tulis test** — `TestNewLoggerLevels` (`"warn"` → info tidak tertulis, warn tertulis; `"bogus"` → info tertulis; output JSON valid). `TestHealthOK` (pinger nil-error → 200 `{"status":"ok"}`), `TestHealthDown` (Review Focus 1: pinger yang memblok sampai ctx selesai → 503 `{"status":"unavailable"}` dalam < 2.5 s), `TestHealthNoAuthNoRateLimit` (101 request berturut tanpa cookie → semua 200). `TestRequestLogFields` (GET `/api/me?token=abc` dengan header `Authorization: Bearer secret` dan `Cookie: pm_session=xyz` → satu baris JSON berisi `method`, `path=="/api/me"`, `status`, `duration_ms`, `request_id`; teks `abc`, `secret`, `xyz`, `token=` tidak muncul di buffer — Review Focus 2), `TestRequestIDPropagation` (header `X-Request-Id: r-1` → respons `X-Request-Id: r-1` dan log `request_id=="r-1"`; tanpa header → 16 hex di keduanya), `TestHealthzLoggedAtDebug` (logger level info → request `/healthz` tidak menulis apa pun).
- [ ] **Step 2:** `go test -race ./internal/logging/ ./internal/api/` → FAIL.
- [ ] **Step 3:** Implementasi; `main.go` membuat logger dari `cfg.LogLevel`, `slog.SetDefault`, mengisi `Deps.Pinger` dengan pool store (tambahkan `(*store.Store).Ping(ctx) error` bila belum ada) dan `Deps.Logger`.
- [ ] **Step 4:** `go test -race ./...` → PASS; `go build ./...`.
- [ ] **Step 5:** Commit `feat: healthz endpoint and JSON request logging` + push.

### Task 2: Image, skrip backup/restore, dan uji siklus

**Files:**
- Create: `deploy/backup.sh`, `deploy/restore.sh`, `deploy/backup-cron.sh`, `deploy/test-backup.sh`
- Modify: `Dockerfile`, `.dockerignore` (pastikan `deploy/*.sh` ikut)

**Interfaces:**
- Consumes: `/healthz` (Task 1).
- Produces:
  - Image app: `debian:bookworm-slim` + `postgresql-client-15`/`16` sesuai repo PGDG atau versi Debian yang bisa dump server 16 (pakai repo `apt.postgresql.org` untuk `postgresql-client-16`) + `ca-certificates` + `curl`; user non-root; skrip di `/app/deploy/`.
  - `backup.sh <daily|preupdate>` — env: `PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE BACKUP_DIR TZ`; `daily` pada hari Minggu menjadi `weekly`; tulis ke `<file>.partial` lalu `mv`; retensi setelah sukses.
  - `restore.sh <file>` — dijalankan di host; memakai `docker compose` (stop app, `docker compose exec -T postgres pg_restore ...` atau container backup, start app, tunggu `/healthz` via `curl` ke `127.0.0.1:8080`).
  - `backup-cron.sh` — loop: hitung detik sampai 02:00 berikutnya di `TZ`, `sleep`, jalankan `backup.sh daily`; log JSON satu baris per jalan.
  - `test-backup.sh` — tanpa argumen; butuh Docker; exit 0 bila semua cek lulus.

- [ ] **Step 1: Tulis `deploy/test-backup.sh`** berisi tiga uji:
  - (a) **Retensi** (Review Focus 4): buat di dir sementara 10 `daily`, 6 `weekly`, 7 `preupdate` dengan timestamp berbeda + `notes.txt`; jalankan fungsi retensi `backup.sh` (mode `PRUNE_ONLY=1`) → tepat 7/4/5 terbaru tersisa, `notes.txt` tetap.
  - (b) **Gagal** (Review Focus 3): `PGHOST` tak terjangkau → exit ≠ 0, tidak ada `*.partial`/`*.dump` baru, file lama utuh, stderr berisi `"level":"ERROR"`.
  - (c) **Siklus** (Review Focus 5): jalankan `postgres:16` container; buat tabel `t(x int)` isi 1..3; `backup.sh daily`; ubah data (hapus 1..3, isi 99); restore file itu dengan `pg_restore --clean --if-exists --no-owner`; cek isi tepat `1,2,3`.
- [ ] **Step 2:** `bash deploy/test-backup.sh` → FAIL (skrip belum ada).
- [ ] **Step 3:** Implementasi skrip (`set -euo pipefail`, shellcheck-clean) dan ubah `Dockerfile`.
- [ ] **Step 4:** `bash deploy/test-backup.sh` → semua PASS; `shellcheck deploy/*.sh` bersih bila shellcheck tersedia; `docker build .` sukses (gunakan `docker-compose.dev.yml` CA bila sandbox memerlukan); `docker run --rm <img> pg_dump --version` mencetak 16.x.
- [ ] **Step 5:** Commit `feat: backup/restore scripts and slim app image` + push.

### Task 3: Compose prod, Watchtower, Nginx, dokumentasi

**Files:**
- Modify: `docker-compose.yml`, `docker-compose.dev.yml`, `deploy/nginx.conf.example`, `.env.example`, `README.md`

**Interfaces:**
- Consumes: image dan skrip Task 2.
- Produces:
  - `app`: `image: ghcr.io/tosrv/pmess:${PMESS_TAG:-latest}` (tanpa `build`), label enable + pre-update hook, env `PG*`/`BACKUP_DIR=/backups`/`LOG_LEVEL`/`TZ`, volume `${BACKUP_DIR:-/var/backups/pmess}:/backups`, healthcheck `curl -fsS http://127.0.0.1:8080/healthz` 30 s/3x.
  - `backup`: image app yang sama dengan `entrypoint: ["/app/deploy/backup-cron.sh"]`, tanpa label watchtower (atau `enable=false`), volume backup, env PG*.
  - `watchtower`: `nickfedor/watchtower:<versi terkunci>`, volume `/var/run/docker.sock`, `command: --interval 300 --label-enable --cleanup --enable-lifecycle-hooks`.
  - Semua service: `logging` json-file 10m×5.
  - `docker-compose.dev.yml`: `app` dan `backup` `build: .` (+ CA secret lama).
  - Nginx: `location = /healthz { proxy_pass http://127.0.0.1:8080; }`.
  - README bagian "Operasional": setup GHCR publik (ubah visibility package ke public sekali), deploy pertama, cara kerja Watchtower, rollback via `PMESS_TAG`, backup (lokasi, jadwal, retensi, `scp` ke lokal), restore (`deploy/restore.sh`), UptimeRobot (monitor HTTP ke `/healthz`, interval 5 menit, alert email, uji dengan `docker compose stop app`), risiko backup hanya di VPS.

- [ ] **Step 1:** `docker compose -f docker-compose.yml config` dengan `.env` sementara (POSTGRES_PASSWORD=test) → valid; pastikan hanya `app` yang punya label enable (`docker compose config | grep watchtower.enable`).
- [ ] **Step 2:** Jalankan stack dev (`docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build` bila sandbox mengizinkan; jika build ditolak, catat dan verifikasi dengan `config` saja) → `curl -fsS 127.0.0.1:8080/healthz` → `{"status":"ok"}`; `docker compose exec app /app/deploy/backup.sh preupdate` → file muncul di dir backup; `docker compose ps` menunjukkan `app` healthy. Tear down `down -v`, hapus `.env` sementara.
- [ ] **Step 3:** Commit `feat: prod compose with watchtower, backups and docs` + push.

### Task 4: GitHub Actions CI

**Files:**
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: semua task sebelumnya; `deploy/test-backup.sh`; E2E config (`web/playwright.config.ts`, `E2E_BASE_URL`).
- Produces: workflow `CI` pada `push` (branch `main`) dan `pull_request`:
  - `go-test`: `actions/setup-go` (go-version-file `go.mod`), `go vet ./...`, `go test -race ./...`.
  - `web-test`: `actions/setup-node` (node 22, cache npm, `web/package-lock.json`), `npm ci`, `npx vitest run`, `npm run build` di `web/`.
  - `e2e`: service `postgres:16` (port 5432, password `pg`), setup Go+Node, `npm ci`, `npx playwright install --with-deps chromium`, `npm run build`, jalankan server di background (`DATABASE_URL`, `E2E_FAKE_LOGIN=1`, `COOKIE_SECURE=false`, `DEVELOPER_EMAIL=dev@e2e.test`, `STATIC_DIR=web/dist`, `LISTEN_ADDR=:8099`), tunggu `/healthz`, `E2E_BASE_URL=http://localhost:8099 npx playwright test`; upload `web/test-results` sebagai artifact bila gagal.
  - `backup-test`: `bash deploy/test-backup.sh`.
  - `image`: `if: github.event_name == 'push' && github.ref == 'refs/heads/main'`, `needs` keempat job, `permissions: contents: read, packages: write`, `docker/login-action` (ghcr.io, `github.actor`, `secrets.GITHUB_TOKEN`), `docker/build-push-action` dengan tag `ghcr.io/tosrv/pmess:latest` dan `ghcr.io/tosrv/pmess:sha-${GITHUB_SHA::7}` (hitung lewat step `id: vars`).
  - `concurrency: ci-${{ github.ref }}` dengan `cancel-in-progress: true`.
  - Semua action dipin ke versi mayor yang ada (mis. `actions/checkout@v4`, `actions/setup-go@v5`, `actions/setup-node@v4`, `docker/login-action@v3`, `docker/build-push-action@v6`, `actions/upload-artifact@v4`).

- [ ] **Step 1:** Validasi sintaks: `actionlint` bila tersedia (`go run github.com/rhysd/actionlint/cmd/actionlint@latest`), atau parse YAML dengan Python → tanpa error.
- [ ] **Step 2:** Commit `ci: test, e2e, backup and image pipeline` + push ke `feat/ops`. Bila push ditolak karena izin workflow → status BLOCKED_PUSH dengan isi file di laporan.
- [ ] **Step 3:** Bila push berhasil: cek run di GitHub (`gh run list --branch feat/ops` / `gh run view`) → job `go-test`, `web-test`, `e2e`, `backup-test` hijau; `image` dilewati (bukan `main`). Catat hasil di laporan.
