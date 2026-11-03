# Design Doc: Siap Publik (pmess)

- **Tanggal:** 2026-11-03
- **Status:** Menunggu review pemilik produk
- **Konteks:** Sub-proyek 2 dari persiapan rilis publik (urutan: fondasi operasional ✅ → **siap publik** → showcase). App live di `https://chat.rahmattomy.com` (Cloudflare DNS only, Nginx, Watchtower).
- **Sudah selesai sebelumnya:** hapus akun (perombakan frontend); teks kebijakan privasi (saat ini di halaman Info).

## 1. Tujuan

- Guest anonim tidak mudah dipakai untuk spam.
- Pengguna bisa memblokir orang yang mengganggu.
- Browser dilindungi lewat header keamanan.
- Ada aturan main tertulis berupa syarat penggunaan.

## 2. Keputusan

| # | Topik | Keputusan | Trade-off |
|---|-------|-----------|-----------|
| 1 | Blokir | **Antar-pengguna.** Blokir membekukan percakapan **dua arah**, berlaku sampai dibuka kembali oleh pihak yang memblokir. Developer bisa memblokir guest. | Hanya melindungi pihak yang memblokir; akun pengganggu tetap ada. Tidak ada blokir global oleh admin. |
| 2 | Spam guest | **Cloudflare Turnstile**, hanya di pembuatan guest. Token diverifikasi di server. Rate limit 5/jam/IP tetap berlaku. | Bergantung pada Cloudflare: bila Cloudflare tidak bisa dihubungi, pembuatan guest ditolak (sengaja gagal dengan aman). |
| 3 | Persetujuan syarat | **Implisit.** Teks "Dengan melanjutkan, Anda menyetujui Syarat Penggunaan dan Kebijakan Privasi" di bawah tombol login dan guest. Tidak ada yang disimpan di database. | Tidak ada catatan kapan seseorang menyetujui versi yang mana. |
| 4 | Halaman legal | `/terms` dan `/privacy` berdiri sendiri (EN/ID). Halaman Info menautkan ke keduanya. | — |
| 5 | Header keamanan | Dipasang di **server Go** lewat middleware, bukan di Nginx. | Nginx bisa menambah header ganda bila dikonfigurasi terpisah; contoh config Nginx tidak menambahkan header ini. |

## 3. Blokir

**Data:** migrasi `0003_blocks`:

```sql
blocks (
  blocker_id uuid references users on delete cascade,
  blocked_id uuid references users on delete cascade,
  created_at timestamptz default now(),
  primary key (blocker_id, blocked_id),
  check (blocker_id <> blocked_id)
)
index on blocks(blocked_id)
```

**Aturan di paket `chat`.** "Terkait blokir" berarti A memblokir B **atau** B memblokir A.

- `Send`: bila pengirim dan lawan bicara terkait blokir → `not_found`. Pesan dari pihak yang diblokir dan dari pihak yang memblokir sama-sama ditolak.
- `OpenConversation`: bila terkait blokir → `not_found`.
- `Search`: hasil tidak memuat pengguna yang terkait blokir dengan pencari.
- `MarkRead` dan `PeerOf` (typing): event `read` dan `typing` tidak diteruskan ke lawan bicara bila terkait blokir. Penyimpanan `last_read_seq` milik sendiri tetap berjalan.
- `Block(u, targetID)`: target harus pengguna yang ada dan bukan diri sendiri (selain itu `not_found` atau `bad_request`). Guest tidak bisa memblokir (`forbidden`). Idempoten: blokir dua kali tetap satu baris.
- `Unblock(u, targetID)`: idempoten; menghapus baris bila ada.
- `Blocks(u)`: daftar `{id, username, display_name, blocked_at}` milik `u`.
- `Conversations(u)`: setiap ringkasan mendapat dua field baru, `blocked_by_me bool` dan `blocked_me bool`. Field `blocked_me` **tidak** dikirim ke klien (tetap `false` di JSON) supaya pihak yang diblokir tidak tahu; field ini hanya dipakai di server.

**API:**

| Method | Path | Keterangan |
|--------|------|------------|
| POST | `/api/blocks` `{user_id}` | 204 |
| DELETE | `/api/blocks/:user_id` | 204 |
| GET | `/api/blocks` | `[{id, username, display_name, blocked_at}]` |
| GET | `/api/conversations` | ringkasan menambah `blocked_by_me` |

**Frontend:**
- Header chat punya menu "Blokir" dengan dialog konfirmasi, dan "Buka blokir" bila sudah diblokir.
- Bila `blocked_by_me`: label "Diblokir" di daftar dan header, kotak tulis pesan diganti teks "Anda memblokir pengguna ini" dengan tombol "Buka blokir".
- Pihak yang diblokir tidak melihat penanda apa pun. Kiriman pesannya mendapat error umum (toast seperti percakapan tidak ditemukan).
- Settings punya bagian "Pengguna yang diblokir" berisi daftar dan tombol buka blokir.
- Guest tidak melihat menu blokir.

## 4. Turnstile

- **Config:** env `TURNSTILE_SITE_KEY` dan `TURNSTILE_SECRET_KEY`. Keduanya kosong → Turnstile nonaktif (dev dan E2E lokal). Hanya salah satu yang terisi → server menolak start dengan error yang jelas.
- **`GET /api/config`** (publik): `{"turnstile_site_key": "<key atau kosong>"}`.
- **Landing:** bila site key ada, script `https://challenges.cloudflare.com/turnstile/v0/api.js` dimuat dan widget dirender. Tombol "Chat sebagai guest" aktif setelah token didapat. Token kedaluwarsa atau error → tombol nonaktif lagi dan widget di-reset.
- **`POST /api/auth/guest {turnstile_token}`:** bila aktif, server memanggil `siteverify` (POST form `secret`, `response`, `remoteip` = ClientIP) dengan timeout 5 detik, **sebelum** rate limit guest dan sebelum menyentuh slot.
  - `success:false` atau token kosong → 400 `captcha_failed`.
  - Galat jaringan, timeout, atau status non-200 → 503 `captcha_unavailable`.
- URL `siteverify` bisa diganti lewat field di struct (untuk test), bukan lewat env.
- **E2E/CI:** memakai test key resmi Cloudflare yang selalu lolos (site key `1x00000000000000000000AA`, secret `1x0000000000000000000000000000000AA`).

## 5. Header keamanan

Middleware `SecurityHeaders(secure bool)` dipasang global (termasuk `/healthz`, API, dan file statis):

```
Content-Security-Policy: default-src 'self'; script-src 'self' https://challenges.cloudflare.com; frame-src https://challenges.cloudflare.com; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'; object-src 'none'
X-Content-Type-Options: nosniff
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: camera=(), microphone=(), geolocation=()
X-Frame-Options: DENY
Strict-Transport-Security: max-age=31536000   (hanya bila COOKIE_SECURE=true)
```

`form-action 'self'` tetap mengizinkan redirect login Google, karena tombol login berupa link biasa, bukan form.

## 6. Halaman legal

- **`/terms`** (EN/ID): proyek pribadi tanpa jaminan layanan; dilarang spam, pelecehan, dan konten ilegal; developer boleh menghapus akun yang melanggar; pesan tidak dienkripsi end-to-end; guest dihapus 30 menit setelah pesan terakhir; percakapan dihapus setelah 1 bulan tanpa aktivitas; pengguna bisa menghapus akunnya kapan saja; batas 300 akun; syarat bisa berubah.
- **`/privacy`** (EN/ID): isi kebijakan privasi yang sudah ada dipindah dari Info, ditambah penjelasan bahwa Cloudflare Turnstile memproses data browser saat membuat guest.
- Kedua halaman menampilkan "Terakhir diperbarui: 3 November 2026".
- Info menautkan ke keduanya. Landing menampilkan teks persetujuan implisit dengan link.

## 7. Testing dan verifikasi

| Area | Test | Cara verifikasi |
|------|------|-----------------|
| Blokir (store/chat) | Kirim dan buka percakapan ke dan dari pihak terkait blokir → `not_found`; pencarian tidak memuatnya; buka blokir → normal; blokir diri sendiri ditolak; blokir dua kali satu baris; guest tidak bisa memblokir; developer bisa memblokir guest; hapus akun menghapus baris blokir | `go test -race ./internal/chat/ ./internal/store/` |
| Blokir (realtime) | `typing`/`read` tidak diteruskan antar pihak terkait blokir; `send` mendapat frame `error not_found` | `go test -race ./internal/realtime/` |
| Turnstile | Token valid → 201; salah → 400 `captcha_failed`; server verifikasi mati/timeout → 503 `captcha_unavailable` dan slot tidak berubah; nonaktif → tanpa token tetap 201; hanya satu key terisi → `config.Load` error | `go test -race ./internal/auth/ ./internal/config/` (server palsu `httptest`) |
| Header | Semua header ada di `/healthz`, `/api/me`, dan `/`; HSTS hanya bila secure | `go test -race ./internal/api/` |
| E2E | A memblokir B → kirim dari B gagal, B hilang dari pencarian A, label "Diblokir" di A; buka blokir → kirim berhasil; `/terms` dan `/privacy` terbuka dari landing; console tanpa pelanggaran CSP di landing, chat, dan settings; guest flow dengan test key Turnstile | `npx playwright test` |

## 8. Di luar scope

Blokir global oleh admin; laporan atau report pengguna; persetujuan syarat yang dicatat di database; moderasi isi pesan; CAPTCHA pada login Google.
