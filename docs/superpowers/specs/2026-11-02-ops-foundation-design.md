# Design Doc: Fondasi Operasional (natter)

- **Tanggal:** 2026-11-02
- **Status:** Menunggu review pemilik produk
- **Konteks:** Sub-proyek 1 dari persiapan rilis publik (urutan: fondasi operasional → siap publik → showcase). Memenuhi kriteria 18 PRD: "Ketika image baru lulus test di CI, maka dapat diambil oleh prod."
- **Dasar:** `main` di `fe2605c` (MVP + perombakan frontend).

## 1. Tujuan

- Setiap push ke `main` dites otomatis: Go `-race`, unit test web, E2E, dan uji backup/restore.
- Image yang lulus semua test dipublikasikan ke registry.
- VPS hanya menjalankan image yang sudah lulus, lalu memperbarui dirinya sendiri.
- Data dibackup otomatis, dan cara restore-nya terbukti jalan.
- Pemilik produk menerima email saat server mati.

## 2. Keputusan

| # | Topik | Keputusan | Trade-off |
|---|-------|-----------|-----------|
| 1 | Cara deploy | **Watchtower** di VPS memantau registry tiap 300 detik dan hanya memperbarui container `app` (lewat label). GitHub tidak punya akses SSH ke VPS. | Waktu update kurang bisa diprediksi (≤ 5 menit), dan migrasi berjalan tanpa dipantau. Dimitigasi dengan backup sebelum update. |
| 1a | Versi Watchtower | `containrrr/watchtower` sudah diarsip sejak Desember 2025 dan tidak menerima patch keamanan lagi. Dipakai fork yang masih aktif, **`nicholas-fedor/watchtower`**, yang kompatibel penuh (label dan flag sama). Tag versinya dikunci (bukan `latest`); nomor versi ditetapkan dan dicek di GitHub fork saat implementasi. | Bergantung pada pemelihara fork. Kalau fork berhenti, opsi pengganti adalah WUD (What's Up Docker). |
| 2 | Registry | **GHCR, image publik** `ghcr.io/tosrv/natter`, dengan tag `latest` dan `sha-<7 karakter>`. Push memakai `GITHUB_TOKEN`; VPS tidak butuh kredensial. | Image (kode hasil build) bisa ditarik siapa pun. Image tidak berisi secret, karena semua config lewat `.env`. |
| 3 | Penyimpanan backup | **Di VPS saja**, di `/var/backups/natter` (bind mount di luar volume Docker). | Tidak melindungi dari VPS yang hilang atau diretas. README memuat perintah `scp` untuk menyalin backup ke komputer sendiri secara manual. |
| 4 | Alert | **UptimeRobot** (gratis) memanggil `https://<domain>/healthz` tiap 5 menit dan mengirim **email** setelah 2 kali gagal berturut-turut, serta saat server pulih. | Butuh akun eksternal, dan `/healthz` terbuka publik (hanya berisi status, tanpa data). |

## 3. Alur dari push sampai live

```
push / pull request
   │
GitHub Actions .github/workflows/ci.yml
   ├─ go-test      : go vet + go test -race ./...  (testcontainers; Docker tersedia di runner ubuntu-latest)
   ├─ web-test     : npm ci, npx vitest run, npm run build
   ├─ e2e          : service postgres:16, npm run build, go run ./cmd/server (E2E_FAKE_LOGIN=1, COOKIE_SECURE=false), npx playwright test
   ├─ backup-test  : docker compose stack → buat data → backup.sh → hapus DB → restore.sh → data kembali utuh
   └─ image        : needs [go-test, web-test, e2e, backup-test]; hanya untuk push ke main
                     → docker build → push ghcr.io/tosrv/natter:latest dan :sha-<sha7>
   │
VPS (docker compose prod)
   ├─ watchtower : interval 300 detik, --label-enable, --cleanup, --enable-lifecycle-hooks
   │               pre-update hook pada app → backup "preupdate"
   ├─ app        : image ghcr.io/tosrv/natter:${NATTER_TAG:-latest}; saat start menjalankan migrasi
   ├─ postgres   : postgres:16 (TIDAK diperbarui Watchtower)
   └─ backup     : postgres:16 + cron/loop harian 02:00 → /var/backups/natter
   │
UptimeRobot → GET /healthz tiap 5 menit → email setelah 2 kali gagal
```

- **Pull request:** menjalankan job test, tetapi tidak job `image`.
- **Rollback:** set `NATTER_TAG=sha-<versi lama>` di `.env`, lalu jalankan `docker compose up -d app`. Selama `NATTER_TAG` bukan `latest`, Watchtower tidak memperbarui apa pun. Rollback image tidak membatalkan migrasi; kalau migrasi yang bermasalah, jalurnya adalah restore dari backup `preupdate`.
- **Dev lokal:** `docker-compose.dev.yml` tetap bisa build dari source (`build: .`) di atas compose prod.

## 4. Backup dan restore

- **Container `backup`** memakai image `postgres:16` dan menjalankan `deploy/backup.sh` tiap hari pukul 02:00 waktu server. Jadwalnya sebelum job retensi aplikasi (03:00).
- **Pre-update:** Watchtower menjalankan hook di container `app` sebelum update. Hook memanggil `pg_dump` (dengan kredensial dari env yang sama) dan menyimpan ke volume backup. `pg_dump` ikut dimasukkan ke image app (paket `postgresql-client` di stage akhir). Konsekuensinya, image akhir bukan lagi distroless murni; base image diganti menjadi `debian:bookworm-slim` yang ringan.
- **Format:** `pg_dump -Fc`. Nama file: `natter-YYYYMMDD-HHMM-<daily|weekly|preupdate>.dump`. Backup harian pada hari Minggu diberi label `weekly`.
- **Retensi:** 7 file `daily` terbaru, 4 `weekly`, dan 5 `preupdate`. Yang lebih lama dihapus oleh `backup.sh` setelah backup berhasil.
- **Kegagalan backup** dicatat ke log JSON dengan `level=ERROR`. Tidak ada alert terpisah (sesuai keputusan 4). Risiko ini dicatat.
- **Restore:** `deploy/restore.sh <file.dump>`:
  1. Hentikan `app`.
  2. `pg_restore --clean --if-exists --no-owner` ke database `natter`.
  3. Nyalakan `app` dan tunggu `/healthz` OK.
- **Bukti restore:** job CI `backup-test` menjalankan siklus backup → hapus → restore di setiap push.

## 5. Health check

- `GET /healthz` (di luar `/api`, tanpa autentikasi, tanpa rate limit) membalas `200 {"status":"ok"}` bila `pool.Ping` dengan timeout 2 detik berhasil, dan `503 {"status":"unavailable"}` bila gagal.
- Healthcheck Docker pada `app` memanggil endpoint yang sama, dengan interval 30 detik dan 3 kali percobaan.
- Nginx meneruskan `/healthz` ke app (ditambahkan ke `deploy/nginx.conf.example`).

## 6. Log terstruktur

- `slog` memakai `JSONHandler` ke stdout. Level diatur lewat env `LOG_LEVEL` (`debug|info|warn|error`, default `info`).
- Middleware request mencatat `method`, `path` (tanpa query string), `status`, `duration_ms`, dan `request_id`. `request_id` diambil dari header `X-Request-Id` bila ada, kalau tidak dibuat baru, lalu dikembalikan di header respons. Request `/healthz` dicatat di level `debug`.
- **Tidak pernah dicatat:** isi pesan, email, token, cookie, header `Authorization`, dan query string. Ada test yang memastikannya.
- Rotasi log memakai driver Docker `json-file` dengan `max-size: 10m` dan `max-file: 5` untuk semua service.

## 7. Konfigurasi baru

| Env | Default | Keterangan |
|-----|---------|------------|
| `NATTER_TAG` | `latest` | Tag image app (untuk rollback) |
| `LOG_LEVEL` | `info` | Level log |
| `BACKUP_DIR` (host) | `/var/backups/natter` | Lokasi backup di host |
| `TZ` | `UTC` | Zona waktu jadwal backup dan retensi |

## 8. Testing dan verifikasi

| Area | Test | Cara verifikasi |
|------|------|-----------------|
| `/healthz` | DB hidup → 200; pool ditutup → 503; tanpa cookie tetap 200 | `go test -race ./internal/api/` |
| Log | Middleware menulis JSON dengan field wajib; body, `Authorization`, cookie, dan query string tidak muncul | `go test -race ./internal/api/` |
| Backup/restore | Siklus lengkap di job CI `backup-test`; retensi menghapus file berlebih (test skrip dengan file dummy) | Lihat job hijau di tab Actions; atau jalankan `deploy/test-backup.sh` secara lokal |
| CI | Semua job hijau di push ke `main`; job `image` tidak jalan pada PR | Tab Actions di GitHub |
| Registry | Image `latest` dan `sha-…` muncul di GHCR dan publik | `docker pull ghcr.io/tosrv/natter:latest` tanpa login |
| Watchtower | Setelah push, app di VPS berganti versi ≤ 5 menit; file `preupdate` muncul | `docker compose logs watchtower`, `ls /var/backups/natter` |
| Alert | Hentikan app → email masuk ≤ ~10 menit; nyalakan → email pulih | Uji manual sekali, dicatat di README |

## 9. Risiko

| Risiko | Mitigasi |
|--------|----------|
| Migrasi buruk ter-deploy otomatis | Backup `preupdate`, dan restore yang teruji di CI |
| VPS hilang berarti backup hilang | Diterima (keputusan 3); perintah salin manual ada di README |
| Fork Watchtower berhenti dipelihara | Versi dikunci; alternatif WUD dicatat |
| Push workflow ke `.github/workflows` mungkin butuh izin token `workflow` | Kalau push ditolak, pemilik produk menambahkan file workflow dari lokal |
| Image publik | Tidak ada secret di image; dicek di review |

## 10. Di luar scope

Penyimpanan backup di luar VPS; alert Telegram atau alert backup gagal; staging environment; metrics/Prometheus; blue-green deploy; CAPTCHA dan header keamanan (sub-proyek 2).
