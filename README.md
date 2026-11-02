# pmess — Personal Messenger

Aplikasi chat pribadi sederhana: login dengan Google (atau masuk sebagai **tamu** untuk menghubungi developer), cari pengguna lewat username, lalu chat realtime lewat WebSocket (status terkirim/dibaca ✓/✓✓, indikator mengetik, notifikasi, UI Indonesia/Inggris).

- Backend: Go (`cmd/server`, `internal/*`), PostgreSQL 16.
- Frontend: React + Vite (`web/`), disajikan oleh server Go dari `STATIC_DIR`.
- Spesifikasi: `docs/superpowers/specs/2026-11-02-personal-messenger-design.md`.

## Prasyarat

- Go 1.24+
- Node.js 22+ dan npm
- Docker + Docker Compose v2 (untuk Postgres, image aplikasi, dan test integrasi testcontainers)

## Setup Google OAuth

1. Buka [Google Cloud Console](https://console.cloud.google.com/) → buat/pilih project.
2. **APIs & Services → OAuth consent screen**: isi nama aplikasi, email dukungan; scope cukup `openid` dan `email`. Tambahkan test user bila status masih *Testing*.
3. **APIs & Services → Credentials → Create credentials → OAuth client ID**, tipe **Web application**.
4. **Authorized redirect URIs**: isi persis sama dengan `OAUTH_REDIRECT_URL`, misalnya
   - produksi: `https://chat.example.com/api/auth/google/callback`
   - lokal: `http://localhost:8080/api/auth/google/callback`
5. Salin **Client ID** dan **Client secret** ke `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET`.

## Variabel lingkungan

Salin `.env.example` ke `.env` lalu isi. `.env` tidak boleh di-commit.

| Variabel | Wajib | Default | Keterangan |
|---|---|---|---|
| `DATABASE_URL` | ya | – | URL Postgres, mis. `postgres://pmess:PASSWORD@localhost:5432/pmess?sslmode=disable`. Di `docker compose` dibangun otomatis dari `POSTGRES_PASSWORD` |
| `POSTGRES_PASSWORD` | ya (compose) | – | Password Postgres untuk `docker compose`; compose menolak jalan bila kosong |
| `GOOGLE_CLIENT_ID` | ya (login Google) | – | Dari Google Cloud Console |
| `GOOGLE_CLIENT_SECRET` | ya (login Google) | – | Rahasia — jangan di-commit |
| `OAUTH_REDIRECT_URL` | ya (login Google) | – | Harus sama dengan Authorized redirect URI |
| `DEVELOPER_EMAIL` | ya | – | Email Google akun developer (penerima pesan tamu) |
| `ALLOWED_ORIGINS` | tidak | kosong | Daftar origin dipisah koma untuk CORS/WebSocket, mis. `https://chat.example.com` |
| `COOKIE_SECURE` | tidak | `false` | Set `true` di produksi (HTTPS) |
| `LISTEN_ADDR` | tidak | `:8080` | Alamat listen HTTP |
| `TRUSTED_PROXY` | tidak | `127.0.0.1` (`172.28.0.1` di compose) | IP peer yang dipercaya untuk header `X-Real-IP` (alamat Nginx). Compose memberi jaringan default subnet tetap `172.28.0.0/16` dengan gateway `172.28.0.1`, sehingga Nginx di host terlihat sebagai `172.28.0.1` — default compose sudah benar. Server mencatat WARN sekali bila `X-Real-IP` datang dari peer tak dipercaya |
| `STATIC_DIR` | tidak | `web/dist` | Folder build frontend (`/app/web` di image Docker) |
| `E2E_FAKE_LOGIN` | tidak | `0` | **Hanya untuk test E2E.** `1` mengaktifkan `POST /api/test/login` |

> ⚠️ **Jangan pernah set `E2E_FAKE_LOGIN=1` di produksi.** Route itu memungkinkan siapa pun login sebagai siapa pun. Server mencetak peringatan keras di log saat aktif, dan **menolak start** bila `E2E_FAKE_LOGIN=1` bersamaan dengan `COOKIE_SECURE=true`.

## Menjalankan lokal (mode dev)

```bash
# 1. Postgres (file dev mempublikasikan 127.0.0.1:5432)
echo POSTGRES_PASSWORD=pmess >> .env
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d postgres

# 2. Backend (terminal 1)
export DATABASE_URL='postgres://pmess:pmess@localhost:5432/pmess?sslmode=disable'
export TRUSTED_PROXY=127.0.0.1
export DEVELOPER_EMAIL=you@example.com
export GOOGLE_CLIENT_ID=... GOOGLE_CLIENT_SECRET=...
export OAUTH_REDIRECT_URL=http://localhost:8080/api/auth/google/callback
go run ./cmd/server          # migrasi DB berjalan otomatis

# 3. Frontend (terminal 2)
cd web && npm install && npm run dev
```

Buka URL yang dicetak Vite (proxy `/api` diarahkan ke `:8080`), atau jalankan `npm run build` lalu buka `http://localhost:8080` langsung.

## Menjalankan dengan Docker

```bash
cp .env.example .env   # isi nilainya; wajib POSTGRES_PASSWORD (DATABASE_URL dibangun darinya)
docker compose up --build -d
# aplikasi di http://127.0.0.1:8080 (hanya localhost; publik lewat Nginx). Postgres tidak dipublikasikan ke host.
docker compose logs -f app
```

Image dibangun multi-stage (Node 22 → Go 1.24 `CGO_ENABLED=0` → `gcr.io/distroless/static`), berisi binary dan file statis di `/app/web`.

**Hanya dev/sandbox:** `docker-compose.dev.yml` mempublikasikan Postgres ke `127.0.0.1:5432` dan menambah build secret `extra_ca` (CA bundle untuk proxy TLS-intercepting). Pakai `EXTRA_CA_FILE=./ca.pem docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build -d`. Jangan dipakai di produksi; tanpa secret, mount di Dockerfile tidak berefek.

## Verifikasi (spec §10)

Semua bisa dijalankan sendiri. Docker harus berjalan untuk test `store` (testcontainers).

```bash
# Seluruh test Go dengan race detector
go test -race ./...

# Per kriteria
go test -race ./internal/chat/...
go test -race ./internal/store/...
go test -race -run TestConcurrentSend ./...     # 11: 50 pengirim bersamaan
go test -race -run TestResend ./...             # 12: reconnect / kirim ulang
go test -race -run TestSlotRace ./...           # 13: slot ke-300
go test -race -run TestAuthz ./internal/api/... # 16: otorisasi (404)
go test -race ./internal/api/...                # 14, 15, 17
go test -race -run TestRetention ./internal/jobs/... # 8: retensi

# Frontend unit test
cd web && npm test
```

**Load test (kriteria 2, latensi < 1 detik)** — butuh server berjalan dan Postgres terjangkau dari host (stack dev, lihat di atas):

```bash
DATABASE_URL='postgres://pmess:pmess@localhost:5432/pmess?sslmode=disable' go run ./cmd/loadtest
# mencetak p50/p95/max; exit 0 + "OK" bila tidak ada pesan hilang/duplikat
```

Load test memakai header `X-Real-IP` berbeda per klien agar tidak terkena rate limit per IP; server harus memercayai peer-nya (`TRUSTED_PROXY`). Untuk `go run ./cmd/server` lokal default `127.0.0.1` sudah cukup; untuk Docker default compose `172.28.0.1` (gateway) sudah cocok.

**E2E Playwright (kriteria 1, 4, 5, 6, 9, 10)** — dua browser saling chat, alur tamu, notifikasi, toggle bahasa:

```bash
printf 'E2E_FAKE_LOGIN=1\nCOOKIE_SECURE=false\nDEVELOPER_EMAIL=dev@e2e.test\nPOSTGRES_PASSWORD=test\n' > .env
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build -d
cd web
npm install
npx playwright install chromium   # sekali saja
E2E_BASE_URL=http://localhost:8080 npm run e2e
cd .. && docker compose -f docker-compose.yml -f docker-compose.dev.yml down -v && rm .env
```

## Nginx (produksi)

Contoh server block lengkap ada di [`deploy/nginx.conf.example`](deploy/nginx.conf.example): redirect HTTP→HTTPS, TLS (ganti path sertifikat), `location /api/ws` dengan header `Upgrade`/`Connection`, `proxy_read_timeout 75s` (ping WS tiap 25 s), dan `X-Real-IP $remote_addr` (dipakai rate limit; pastikan `TRUSTED_PROXY` sesuai). Di produksi set juga `COOKIE_SECURE=true` dan `ALLOWED_ORIGINS=https://domain-anda`.

## Privasi

- Pesan **tidak** dienkripsi end-to-end; admin server dapat membacanya lewat database.
- Email pengguna tidak pernah dikirim ke pengguna lain; isi pesan selalu dirender sebagai teks biasa.
- Akun tamu dihapus 30 menit setelah aktivitas terakhir percakapannya; percakapan dihapus setelah 1 bulan tanpa aktivitas.
- Info kontak yang diisi tamu bersifat sukarela dan ikut terhapus bersama akun tamu.
