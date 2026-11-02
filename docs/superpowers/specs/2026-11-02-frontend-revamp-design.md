# Design Doc: Perombakan Frontend (pmess v1)

- **Tanggal:** 2026-11-02
- **Status:** Menunggu review pemilik produk
- **Konteks:** Sub-proyek pertama dari persiapan rilis publik. Urutan: **perombakan frontend** → fondasi operasional → siap publik → showcase.
- **Dasar:** MVP di `main` (`c59d861`), spec `2026-11-02-personal-messenger-design.md`.
- **Mockup arah visual:** https://claude.ai/artifact/1EagevBBrbANzUXo95186g (arah B + status pesan gaya WhatsApp).

## 1. Tujuan

Membuat tampilan pmess layak dipakai publik dan dipamerkan sebagai portfolio. Caranya: landing dan halaman chat dirombak dengan arah visual "Friendly bubbles", ditambah halaman Settings, display name, dan hapus akun.

## 2. Keputusan

| # | Topik | Keputusan | Trade-off |
|---|-------|-----------|-----------|
| 1 | Arah visual | B "Friendly bubbles": hangat, gelembung bulat, aksen oranye `#e2552f`, font Nunito. Tema terang dan gelap. | Kesan personal/santai, kurang "korporat". |
| 2 | Status pesan | Ikon kecil di samping jam: **mengirim** (jam), **terkirim** (satu centang, pesan tersimpan di server), **dibaca** (dua centang bertumpuk, biru muda). Tidak ada status "sampai di perangkat". | Berbeda sedikit dari WhatsApp; tidak menambah beban server. |
| 3 | Display name | Nama yang tampil di mana-mana; username hanya untuk pencarian dan identitas unik. | Nama bisa kembar; @username tetap terlihat di hasil pencarian dan profil lawan bicara. |
| 4 | Aturan display name | 1–40 karakter setelah trim, spasi ganda dirapikan jadi satu. Hanya huruf (`\p{L}`), tanda diakritik (`\p{M}`), angka (`\p{N}`), spasi, dan `.` `-` `'` `_`. **Tanpa emoji.** Tidak unik. Bisa diubah tanpa batas. Divalidasi di server (sumber kebenaran) dan di klien (umpan balik cepat). | Nama dengan simbol lain ditolak. |
| 5 | Display name guest | Otomatis `Guest XXXX` (4 karakter hex acak, sama dengan akhiran username guest). Tidak bisa diubah. | — |
| 6 | Isi Settings | Profil (display name; username dan sisa jatah 3 perubahan), bahasa EN/ID, tema (terang/gelap/ikut sistem), logout, hapus akun. Guest: bahasa, tema, "Akhiri sesi". | — |
| 7 | Hapus akun | Akun, sesi, semua percakapan dan pesannya terhapus (CASCADE), termasuk di sisi lawan bicara. Slot kembali. Konfirmasi dengan mengetik username sendiri. Akun developer tidak bisa dihapus. | Lawan bicara kehilangan riwayat chat itu tanpa bisa mencegah. |

## 3. Halaman

1. **Landing:**
   - Judul hangat dan ilustrasi percakapan berupa gelembung chat.
   - Tombol "Masuk dengan Google" dan "Chat sebagai guest", serta link ke Info.
   - Error dari `?error=` tampil sebagai kartu yang diterjemahkan, lalu dihapus dari URL.
2. **Onboarding:** satu formulir berisi display name (diisi awal dari nama Google) dan username. Validasi tampil saat mengetik.
3. **Chat:**
   - **Tata letak:** di layar ≥ 768px, daftar percakapan di kiri dan percakapan aktif di kanan. Di bawah itu, satu panel saja dengan tombol kembali.
   - **Daftar percakapan:** avatar, display name, potongan pesan terakhir, jam, dan badge belum dibaca.
   - **Header:** avatar, display name, dan "sedang mengetik…" bila ada.
   - **Pesan:** gelembung (milik sendiri di kanan, oranye) dengan jam dan `StatusTicks`. Pemisah tanggal: "Hari ini", "Kemarin", atau tanggal lengkap.
   - **Composer:** sudut membulat penuh, tombol kirim bulat, dan penghitung karakter saat mendekati 2000.
   - **Status koneksi:** banner "Menyambung ulang…" saat socket terputus.
   - **Pencarian:** hasil menampilkan display name dengan `@username` di bawahnya.
   - Dialog kontak guest dan notifikasi toast tetap ada, dengan gaya baru.
4. **Settings (`/settings`):** isi sesuai keputusan 6. Hapus akun ada di bagian "Zona berbahaya" dengan dialog konfirmasi. Tombol hapus baru aktif bila username yang diketik cocok.
5. **Info:** isi tetap, gaya mengikuti tema baru.

## 4. Frontend teknis

- **Tema:**
  - Warna didefinisikan sebagai variabel CSS di `:root` (terang) dan `.dark` (gelap). Tailwind dan shadcn membaca variabel itu.
  - Pilihan disimpan di `localStorage` key `pmess.theme` (`light|dark|system`), dibungkus try/catch. Default-nya `system`, mengikuti `prefers-color-scheme`.
- **Pemecahan `Chat.tsx`:**
  - Hook `useChat` memegang socket, outbox, riwayat, typing, dan read. Perilakunya tetap sama dengan MVP, termasuk catch-up berhalaman dan retry `rate_limited`.
  - Komponen tampilan: `ConversationList`, `ChatHeader`, `MessageList`, `MessageBubble`, `StatusTicks`, `Composer`, `Avatar`.
- **Fungsi murni (diuji Vitest), di `web/src/lib/`:**
  - `statusOf(msg, myId, peerLastReadSeq, pendingIds): "sending" | "sent" | "read"`
  - `dayLabel(date: Date, now: Date, lang: "en" | "id"): string`
  - `avatarColor(userId: string): string`, deterministik, dari palet 8 warna.
  - `validateDisplayName(s: string): { ok: true; value: string } | { ok: false }`, dengan aturan yang sama dengan server.
- **Socket:** event `conversation_deleted` ditangani dengan membuang percakapan itu dari daftar dan dari cache pesan. Bila percakapan itu sedang terbuka, tampilan kembali ke daftar dengan toast.
- **i18n:** semua teks baru ditulis di `en.json` dan `id.json`. Test kesamaan key yang sudah ada tetap berlaku.

## 5. Backend teknis

- **Migrasi `0002_display_name`:**
  - Tambah kolom `users.display_name text`.
  - Isi pengguna lama: Google dari `username` (atau `'User'` bila belum ada), guest dari `'Guest ' || upper(substr(username, 7))`.
  - Lalu set `NOT NULL` dan `CHECK (char_length(display_name) BETWEEN 1 AND 40)`.
- **`chat.ValidateDisplayName(raw string) (string, error)`:** menormalisasi dan memvalidasi sesuai keputusan 4. Error-nya `display_name_invalid` (400).
- **Login Google:** user baru mendapat display name dari klaim `name` di userinfo (sudah divalidasi). Bila tidak valid atau kosong, dipakai `'User'`, dan onboarding meminta pengguna mengisinya.
- **Perubahan API:**
  - `GET /me` menambah `display_name`.
  - `PUT /me/display-name {display_name}` mengembalikan `/me`. Guest mendapat `forbidden`.
  - `PUT /me/username` tidak berubah.
  - `GET /users/search` mengembalikan `[{id, username, display_name}]`.
  - `GET /conversations`: `peer_display_name` ditambahkan ke ringkasan.
  - **`DELETE /api/me`** mengembalikan 204, lalu cookie dihapus. Langkah-langkahnya:
    1. Bila developer, tolak dengan `forbidden`.
    2. Dalam satu transaksi: ambil `peer_id` dari semua percakapan, `DELETE FROM users WHERE id = $1` (cascade), dan `user_slots.used - 1`.
    3. Setelah commit, hub mengirim `{type: "conversation_deleted", conversation_id}` ke tiap lawan bicara yang online, lalu `Disconnect` user tersebut.
  - Guest memakai endpoint yang sama, dengan perilaku setara "Akhiri sesi" (logout guest).
- **Frame WS baru:** `conversation_deleted` (dari server ke klien). Tidak ada perubahan pada frame lain.

## 6. Testing dan verifikasi

| Area | Test | Cara verifikasi |
|------|------|-----------------|
| Validasi display name | `TestValidateDisplayName`: "Rahmat Tomy" OK, "José" OK, "😀" ditolak, "a★b" ditolak, 41 karakter ditolak, "  a   b " → "a b" | `go test -race ./internal/chat/` |
| Migrasi | Data lama terisi; `NOT NULL` dan CHECK aktif | `go test -race ./internal/store/` |
| Hapus akun | Slot −1; percakapan dan pesan lawan bicara hilang; sesi langsung 401; developer ditolak `forbidden` | `go test -race -run TestDeleteAccount ./...` |
| Event hapus | Lawan bicara online menerima `conversation_deleted`; koneksi user ditutup | `go test -race ./internal/realtime/` |
| API | `/me` dan search mengandung `display_name`; respons tidak mengandung email | `go test -race ./internal/api/` |
| Fungsi murni web | `statusOf`, `dayLabel`, `avatarColor`, `validateDisplayName` | `cd web && npx vitest run` |
| E2E | Onboarding dengan display name; header menampilkan display name; centang terkirim → dibaca; ganti tema/bahasa di Settings; hapus akun → landing, dan percakapan hilang dari lawan bicara | `npx playwright test` terhadap stack Docker |
| Visual | Screenshot landing, chat, dan settings (terang dan gelap, desktop dan HP) | Dilampirkan di laporan untuk review pemilik produk |

## 7. Di luar scope

Foto profil; status "sampai di perangkat"; opsi mematikan status dibaca atau mengetik; pagination riwayat lama; CI/CD; backup; CAPTCHA; blokir pengguna; header keamanan; README showcase.
