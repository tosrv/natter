# Design Doc: Personal Messenger (MVP / v0)

- **Tanggal:** 2026-11-02
- **Status:** Menunggu review pemilik produk
- **Sumber:** PRD "Personal Messenger (MVP / v0)"
- **Repo:** `tosrv/pmess`

## 1. Ringkasan

Layanan chat web real-time 1:1. Backend berupa API bersih (REST + WebSocket) yang kelak juga dipakai widget di repo terpisah. Tujuan utama: belajar real-time dan konkurensi. Pengguna: akun Google, guest (hanya chat dengan developer), dan satu akun developer.

Fitur di luar scope mengikuti PRD bagian 5 (file, E2E, push notification, grup, multi-device, edit/hapus/cari pesan, mobile, widget, auth lain). Tidak ada fitur tambahan.

## 2. Keputusan yang sudah diambil

| # | Topik | Keputusan | Trade-off utama |
|---|-------|-----------|-----------------|
| 1 | Batas 300 | Menghitung akun yang **ada saat ini**: semua akun Google + guest aktif. Guest dihapus → slot kembali. Akun Google memegang slot permanen (tidak ada hapus akun di MVP). | Perlu counter yang aman terhadap konkurensi, bukan `COUNT(*)`. |
| 2 | Sesi guest selesai | Guest dihapus **30 menit setelah pesan terakhir** di percakapannya (dari pihak mana pun). Putus koneksi/refresh tidak langsung menghapus. Setelah dihapus, harus membuat guest baru. | Sederhana dan pasti; guest yang menutup tab masih memegang slot ≤ 30 menit. |
| 3 | Akun developer | Env var `DEVELOPER_EMAIL`. Login Google dengan email itu → `is_developer = true`. | Ganti developer butuh restart. |
| 4a | Backend | **Go + Gin**, WebSocket via `coder/websocket`. | Satu dependensi tambahan; konkurensi tetap ditulis sendiri. |
| 4b | Protokol | **WebSocket** untuk event real-time; **REST** untuk auth, daftar, riwayat. | Reconnect dan heartbeat ditangani sendiri. |
| 4c | Urutan & duplikasi | `seq` per percakapan via row lock (`last_seq + 1`); `client_msg_id` unik per pengirim untuk idempotency. | Pesan dalam satu percakapan diserialisasi (tidak berdampak pada skala < 100). |
| 5 | Identitas sesi | Cookie `HttpOnly; Secure; SameSite=Lax`, token acak, **hash** disimpan di DB. API juga menerima `Authorization: Bearer` (untuk widget nanti). | Widget lintas domain kelak perlu pengaturan CORS/SameSite. |
| 6 | Job terjadwal | Pembersih guest **tiap 1 menit**; retensi 1 bulan **harian pukul 03.00** (zona waktu server). Keduanya goroutine di dalam proses server. | Dua penjadwal; aman karena hanya satu instance. |
| 7 | Dibaca/mengetik | Selalu aktif. `typing` di-throttle ≤ 1 per 3 detik dan tidak disimpan; `read` disimpan sebagai `last_read_seq`. | Tidak ada opsi privasi untuk mematikan (di luar scope). |
| 8 | Username | Akun Google memilih username saat login pertama. Email **tidak pernah** terlihat oleh pengguna lain. Username bisa diubah **maksimal 3 kali**. Pencarian berdasarkan awalan. | Pencarian awalan memungkinkan enumerasi; dimitigasi dengan batas hasil dan rate limit. |
| 9 | Frontend | **React + Vite + TypeScript + Tailwind + shadcn/ui**, i18n `react-i18next` (EN/ID). | Bundle lebih besar; UI lebih cepat jadi. |
| 10 | Reverse proxy | **Nginx yang sudah ada di VPS** (TLS di Nginx). | Config WebSocket harus diatur manual (lihat §9). |

## 3. Arsitektur

```
Browser (React SPA) ──HTTPS REST──┐
                    ──WSS─────────┤
                    Go + Gin server (1 proses, 1 container)
                      ├─ auth      : Google OAuth, guest, sesi cookie
                      ├─ api       : REST handler
                      ├─ realtime  : hub WebSocket
                      ├─ chat      : aturan bisnis inti
                      ├─ store     : PostgreSQL (pgx + SQL manual)
                      ├─ ratelimit : token bucket di memori
                      └─ jobs      : pembersih guest, retensi
                             │
                        PostgreSQL 16
```

**Batas tanggung jawab tiap unit:**

- `chat`: **satu-satunya** tempat aturan bisnis (guest hanya ke developer, keanggotaan percakapan, panjang pesan, batas username). Dipanggil oleh REST dan WebSocket. Dapat diuji tanpa jaringan.
- `store`: query SQL dan transaksi. Tanpa ORM, supaya lock dan transaksi terlihat jelas. Migrasi memakai `golang-migrate`.
- `realtime`: mengelola koneksi; tidak berisi aturan bisnis.
- `auth`: OAuth Google (`golang.org/x/oauth2`), pembuatan guest, sesi.
- `jobs`: memanggil fungsi `store` sesuai jadwal.
- Frontend di-build menjadi file statis dan disajikan oleh server Go yang sama.

**Layout repo:**

```
cmd/server/          main.go
internal/{auth,api,realtime,chat,store,ratelimit,jobs,config}/
migrations/
web/                 React app
docker-compose.yml, Dockerfile
```

## 4. Model data

```sql
users (
  id uuid pk,
  kind text check (kind in ('google','guest')),
  google_sub text unique,          -- null untuk guest
  email text,                      -- tidak pernah dikirim ke pengguna lain
  username text,                   -- null sampai onboarding selesai; guest: 'guest-xxxx'
  username_changes int default 0 check (username_changes <= 3),
  is_developer bool default false,
  created_at timestamptz
)
unique index on lower(username)
index on lower(username) text_pattern_ops   -- pencarian awalan

sessions (
  token_hash bytea pk,
  user_id uuid references users on delete cascade,
  expires_at timestamptz
)

conversations (
  id uuid pk,
  user_a uuid references users on delete cascade,
  user_b uuid references users on delete cascade,
  check (user_a < user_b),
  unique (user_a, user_b),
  last_seq bigint default 0,
  last_activity_at timestamptz      -- index
)

conversation_members (
  conversation_id uuid references conversations on delete cascade,
  user_id uuid references users on delete cascade,
  last_read_seq bigint default 0,
  primary key (conversation_id, user_id)
)

messages (
  id uuid pk,
  conversation_id uuid references conversations on delete cascade,
  seq bigint,
  sender_id uuid references users on delete cascade,
  client_msg_id uuid,
  body text check (char_length(body) between 1 and 2000),
  created_at timestamptz,
  unique (conversation_id, seq),
  unique (sender_id, client_msg_id)
)

user_slots (
  id int pk check (id = 1),
  used int check (used between 0 and 300)
)
```

**Aturan konkurensi:**

- **Slot (kriteria 7 dan 13):** registrasi Google baru atau pembuatan guest menjalankan, dalam satu transaksi, `UPDATE user_slots SET used = used + 1 WHERE id = 1`, lalu `INSERT users`. Jika `CHECK` gagal, registrasi ditolak dengan `user_limit_reached`. Row lock memaksa pendaftar yang bersamaan mengantre, sehingga di slot ke-300 hanya satu yang berhasil. Penghapusan guest menjalankan `used - 1` di transaksi yang sama dengan `DELETE`.
- **Percakapan unik per pasangan:** `INSERT ... ON CONFLICT (user_a, user_b) DO NOTHING`, lalu `SELECT`.
- **Kirim pesan:** dalam satu transaksi: `UPDATE conversations SET last_seq = last_seq + 1, last_activity_at = now() WHERE id = $1 RETURNING last_seq`, lalu `INSERT messages`. Jika `client_msg_id` sudah ada, transaksi di-rollback dan pesan lama dikembalikan (idempotent).
- **Username:** konflik ditangani oleh unique index (`username_taken`). Batas 3 kali dijaga oleh `CHECK` dan `UPDATE ... SET username_changes = username_changes + 1`.
- **Hapus guest:** `DELETE FROM users` dengan `CASCADE` ikut menghapus sesi, percakapan, pesan, dan info kontak (yang dikirim sebagai pesan).

## 5. Protokol WebSocket

Koneksi: `GET /ws` (diautentikasi dengan cookie/Bearer, `Origin` dicek). Semua frame berupa JSON `{"type": ...}`.

| Arah | type | Isi | Keterangan |
|------|------|-----|------------|
| C→S | `send` | `conversation_id, client_msg_id, body` | Kirim pesan |
| S→C | `ack` | `client_msg_id, message` | Pesan tersimpan → status "terkirim" |
| S→C | `message` | `message` | Pesan baru untuk penerima |
| C→S | `typing` | `conversation_id` | Throttle ≤ 1 per 3 detik di klien |
| S→C | `typing` | `conversation_id, user_id` | Indikator hilang setelah 5 detik tanpa event |
| C→S | `read` | `conversation_id, seq` | Hanya saat percakapan terbuka dan tab aktif |
| S→C | `read` | `conversation_id, user_id, seq` | Status "dibaca" di sisi pengirim |
| S→C | `error` | `code, message, client_msg_id?` | Contoh: `rate_limited`, `message_too_long`, `forbidden` |

`message` = `{id, conversation_id, seq, sender_id, body, created_at}`.

**Reconnect (kriteria 12):** klien menyimpan pesan yang belum mendapat `ack` dan mengirim ulang dengan `client_msg_id` yang sama. Setelah tersambung, klien memanggil `GET /conversations/:id/messages?after_seq=N` untuk percakapan yang terbuka, dan `GET /conversations` untuk memperbarui daftar. Backoff: 1, 2, 4, … maksimal 30 detik.

**Heartbeat:** server mengirim ping setiap 25 detik; tanpa pong dalam 60 detik, koneksi ditutup.

**Model konkurensi hub:**

- Setiap koneksi punya goroutine baca dan goroutine tulis, serta channel keluar berbuffer (64).
- Jika buffer penuh (klien lambat), koneksi diputus. Klien akan mengambil pesan yang terlewat saat reconnect, sehingga tidak ada pesan hilang.
- Peta `user_id → koneksi` dijaga dengan `sync.RWMutex`.
- Satu koneksi per user. Koneksi baru menutup koneksi lama (multi-device di luar scope).
- Penerusan pesan ke penerima dilakukan **setelah commit**.

## 6. REST API

Base path `/api`. Error memakai format `{"error":{"code":"...","message":"..."}}`. Frontend menerjemahkan `code` ke EN/ID.

| Method | Path | Keterangan |
|--------|------|------------|
| GET | `/auth/google/login`, `/auth/google/callback` | OAuth (parameter `state` untuk mencegah CSRF) |
| POST | `/auth/guest` | Membuat guest + percakapan dengan developer. Rate limit 5/jam/IP. |
| POST | `/auth/logout` | Menghapus sesi. Jika guest: akun ikut dihapus. |
| GET | `/me` | Profil sendiri (termasuk sisa jatah ubah username) |
| PUT | `/me/username` | Set atau ubah username |
| GET | `/users/search?q=` | Pencarian awalan: min 2 karakter, maks 10 hasil, 30/menit. Hanya mengembalikan `id, username`. Tidak tersedia untuk guest; guest tidak muncul di hasil. |
| POST | `/conversations` `{peer_id}` | Buat atau ambil percakapan. Guest ditolak. |
| GET | `/conversations` | Daftar: lawan bicara, pesan terakhir, jumlah belum dibaca |
| GET | `/conversations/:id/messages?after_seq=&before_seq=&limit=` | Riwayat atau pesan yang terlewat (maks 100 per halaman) |
| GET | `/ws` | WebSocket |

**Username:** 3–20 karakter `[a-z0-9_]`, unik tanpa membedakan huruf besar-kecil. Pilihan pertama saat onboarding tidak dihitung sebagai perubahan; setelahnya maksimal 3 kali. Pengguna Google tanpa username hanya bisa memanggil `/me` dan `PUT /me/username`.

**Otorisasi (kriteria 16):** setiap query percakapan dan pesan menyertakan syarat keanggotaan. Jika bukan anggota, server menjawab `404` (bukan `403`) agar keberadaan percakapan tidak bocor. Guest hanya bisa mengakses percakapannya dengan developer. Aturan ini ada di paket `chat`, sehingga berlaku sama untuk REST dan WebSocket.

## 7. Alur guest

1. `POST /auth/guest`: ambil slot → buat user `guest-xxxx` → buat percakapan dengan developer → set cookie.
2. UI langsung membuka percakapan tersebut dan menampilkan Dialog **"How can the developer reach you?"** (EN/ID). Bisa dilewati. Jika diisi, jawaban dikirim sebagai pesan biasa.
3. Refresh dengan cookie yang masih valid → kembali ke percakapan yang sama.
4. Pembersih (tiap 1 menit) menghapus guest jika `last_activity_at` percakapannya > 30 menit yang lalu. Untuk guest yang belum pernah mengirim pesan, waktu dihitung dari `created_at`.
5. Cookie dari guest yang sudah dihapus → `401` → UI kembali ke layar awal.

## 8. Keamanan

- **XSS (kriteria 14):** isi pesan ditampilkan sebagai teks oleh React (tanpa `dangerouslySetInnerHTML`).
- **Rate limit (kriteria 15):** token bucket di memori, per user dan per IP: 20 pesan / 10 detik; guest 5/jam/IP; pencarian 30/menit. IP diambil dari `X-Real-IP` yang di-set Nginx (hanya dipercaya dari proxy). Data limit hilang saat restart; dapat diterima untuk satu instance.
- **Panjang pesan (kriteria 17):** maksimal 2000 karakter dan tidak boleh kosong, divalidasi di `chat` dan oleh `CHECK` di DB. Ukuran frame WebSocket dibatasi 8 KB.
- **Transport:** HTTPS/WSS wajib di prod; cookie `Secure`; WebSocket `Origin` dicek terhadap daftar origin yang diizinkan.
- **Sesi:** token 32 byte acak; DB hanya menyimpan SHA-256 dari token; masa berlaku akun Google 30 hari.
- **Halaman informasi (EN/ID):** pesan tidak dienkripsi end-to-end dan admin server dapat membacanya lewat database; aturan penghapusan guest (30 menit) dan retensi (1 bulan); info kontak bersifat sukarela dan ikut terhapus.

## 9. Deployment

- **Dockerfile multi-stage:** build frontend (Node) → build Go → image akhir minimal berisi binary dan file statis.
- **`docker-compose.yml`:** `app` + `postgres:16` (volume). Dipakai di dev dan prod.
- **Konfigurasi via env:** `DATABASE_URL`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `OAUTH_REDIRECT_URL`, `DEVELOPER_EMAIL`, `ALLOWED_ORIGINS`, `COOKIE_SECURE`.
- **Nginx (sudah ada di VPS):** TLS dan proxy ke container. Contoh untuk route WebSocket:

```nginx
location /api/ws {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header X-Real-IP $remote_addr;
    proxy_read_timeout 75s;
}
```

- **CI → prod (kriteria 18):** fase akhir, tidak termasuk plan awal.

## 10. Testing dan verifikasi

Semua test bisa dijalankan sendiri oleh pemilik produk. Hasil test tidak dianggap lulus hanya karena dilaporkan.

| Kriteria | Test | Cara verifikasi |
|----------|------|-----------------|
| Aturan bisnis (`chat`) | Unit test | `go test ./internal/chat/...` |
| `store` | Integration test dengan Postgres asli (testcontainers) | `go test ./internal/store/...` (Docker harus berjalan) |
| 11: 50 pengirim bersamaan | 50 goroutine mengirim ke percakapan yang sama dan berbeda; cek seq rapat 1..N, jumlah pas, tanpa duplikat | `go test -race -run TestConcurrentSend ./...` |
| 12: reconnect | Kirim ulang `client_msg_id` yang sama → satu baris; `after_seq` mengembalikan pesan yang terlewat | `go test -race -run TestResend ./...` |
| 13: slot ke-300 | `used = 299`, dua registrasi bersamaan → tepat satu sukses | `go test -race -run TestSlotRace ./...` |
| 16: otorisasi | User C meminta percakapan A–B → 404 | `go test -run TestAuthz ./internal/api/...` |
| 14, 15, 17 | Test handler | `go test ./internal/api/...` |
| 2: latensi < 1 detik | Skrip load: 50 klien WebSocket, ukur p95 | `go run ./cmd/loadtest` (mencetak p50/p95/maks) |
| 1, 4, 5, 6, 9, 10 | E2E Playwright: dua browser saling chat, alur guest, notifikasi, toggle bahasa | `npm run e2e` di `web/` |
| 8: retensi | Integration test dengan waktu yang dimundurkan | `go test -run TestRetention ./internal/jobs/...` |

Semua test Go dijalankan dengan `-race` (race detector bawaan Go).

## 11. Risiko dan mitigasi

| Risiko | Mitigasi |
|--------|----------|
| Spam guest | Rate limit 5/jam/IP + batas 300 + penghapusan otomatis 30 menit |
| Penutupan tab tidak terdeteksi | Penghapusan berbasis waktu tidak aktif, bukan status koneksi; heartbeat untuk membersihkan koneksi mati |
| Data sensitif | HTTPS/WSS, hash token sesi, otorisasi keanggotaan, halaman informasi |
| Data kontak guest | Sukarela; disimpan sebagai pesan sehingga ikut terhapus oleh `CASCADE` |
| Enumerasi username | Pencarian min 2 karakter, maks 10 hasil, rate limit, hanya username yang terlihat |
| Scope melebar | Daftar di luar scope (PRD bagian 5) dipegang ketat |
