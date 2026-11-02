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
| `DATABASE_URL` | ya | – | URL Postgres, mis. `postgres://pmess:pmess@localhost:5432/pmess?sslmode=disable` |
| `GOOGLE_CLIENT_ID` | ya (login Google) | – | Dari Google Cloud Console |
| `GOOGLE_CLIENT_SECRET` | ya (login Google) | – | Rahasia — jangan di-commit |
| `OAUTH_REDIRECT_URL` | ya (login Google) | – | Harus sama dengan Authorized redirect URI |
| `DEVELOPER_EMAIL` | ya | – | Email Google akun developer (penerima pesan tamu) |
| `ALLOWED_ORIGINS` | tidak | kosong | Daftar origin dipisah koma untuk CORS/WebSocket, mis. `https://chat.example.com` |
| `COOKIE_SECURE` | tidak | `false` | Set `true` di produksi (HTTPS) |
| `LISTEN_ADDR` | tidak | `:8080` | Alamat listen HTTP |
| `TRUSTED_PROXY` | tidak | `127.0.0.1` | IP peer yang dipercaya untuk header `X-Real-IP` (alamat Nginx). Bila app berjalan di Docker dan Nginx di host, isi IP gateway bridge Docker (cek: `docker network inspect pmess_default -f '{{(index .IPAM.Config 0).Gateway}}'`) |
| `STATIC_DIR` | tidak | `web/dist` | Folder build frontend (`/app/web` di image Docker) |
| `E2E_FAKE_LOGIN` | tidak | `0` | **Hanya untuk test E2E.** `1` mengaktifkan `POST /api/test/login` |

> ⚠️ **Jangan pernah set `E2E_FAKE_LOGIN=1` di produksi.** Route itu memungkinkan siapa pun login sebagai siapa pun. Server mencetak peringatan keras di log saat aktif.

## Menjalankan lokal (mode dev)

```bash
# 1. Postgres
docker compose up -d postgres

# 2. Backend (terminal 1)
export DATABASE_URL='postgres://pmess:pmess@localhost:5432/pmess?sslmode=disable'
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
cp .env.example .env   # isi nilainya; DATABASE_URL default sudah menunjuk service postgres
docker compose up --build -d
# aplikasi di http://localhost:8080
docker compose logs -f app
```

Image dibangun multi-stage (Node 22 → Go 1.24 `CGO_ENABLED=0` → `gcr.io/distroless/static`), berisi binary dan file statis di `/app/web`.

Bila build berjalan di belakang proxy TLS yang memakai CA sendiri, letakkan CA bundle di dalam folder project dan jalankan `EXTRA_CA_FILE=./ca.pem docker compose up --build -d` (opsional; tidak dipakai di build normal).

## Verifikasi (spec §10)

Semua bisa dijalankan sendiri. Docker harus berjalan untuk test `store` (testcontainers).

```bash
# Seluruh test Go dengan race detector
go test -race ./...

# Per kriteria
go test ./internal/chat/...
go test ./internal/store/...
go test -race -run TestConcurrentSend ./...     # 11: 50 pengirim bersamaan
go test -race -run TestResend ./...             # 12: reconnect / kirim ulang
go test -race -run TestSlotRace ./...           # 13: slot ke-300
go test -run TestAuthz ./internal/api/...       # 16: otorisasi (404)
go test ./internal/api/...                      # 14, 15, 17
go test -run TestRetention ./internal/jobs/...  # 8: retensi

# Frontend unit test
cd web && npm test
```

**Load test (kriteria 2, latensi < 1 detik)** — butuh server berjalan:

```bash
DATABASE_URL='postgres://pmess:pmess@localhost:5432/pmess?sslmode=disable' go run ./cmd/loadtest
# mencetak p50/p95/max; exit 0 + "OK" bila tidak ada pesan hilang/duplikat
```

Load test memakai header `X-Real-IP` berbeda per klien agar tidak terkena rate limit per IP; server harus memercayai peer-nya (`TRUSTED_PROXY`). Untuk `go run ./cmd/server` lokal default `127.0.0.1` sudah cukup; untuk Docker set `TRUSTED_PROXY` ke IP gateway bridge (lihat tabel env).

**E2E Playwright (kriteria 1, 4, 5, 6, 9, 10)** — dua browser saling chat, alur tamu, notifikasi, toggle bahasa:

```bash
E2E_FAKE_LOGIN=1 DEVELOPER_EMAIL=dev@e2e.test docker compose up --build -d
cd web
npm install
npx playwright install chromium   # sekali saja
E2E_BASE_URL=http://localhost:8080 npm run e2e
cd .. && docker compose down -v    # hapus stack + data test
```

## Nginx (produksi)

Contoh server block lengkap ada di [`deploy/nginx.conf.example`](deploy/nginx.conf.example): redirect HTTP→HTTPS, TLS (ganti path sertifikat), `location /api/ws` dengan header `Upgrade`/`Connection`, `proxy_read_timeout 75s` (ping WS tiap 25 s), dan `X-Real-IP $remote_addr` (dipakai rate limit; pastikan `TRUSTED_PROXY` sesuai). Di produksi set juga `COOKIE_SECURE=true` dan `ALLOWED_ORIGINS=https://domain-anda`.

## Privasi

- Pesan **tidak** dienkripsi end-to-end; admin server dapat membacanya lewat database.
- Email pengguna tidak pernah dikirim ke pengguna lain; isi pesan selalu dirender sebagai teks biasa.
- Akun tamu dihapus 30 menit setelah aktivitas terakhir percakapannya; percakapan dihapus setelah 1 bulan tanpa aktivitas.
- Info kontak yang diisi tamu bersifat sukarela dan ikut terhapus bersama akun tamu.
