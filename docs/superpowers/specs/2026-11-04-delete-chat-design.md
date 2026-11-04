# Desain: Hapus Chat (untuk saya saja)

Tanggal: 2026-11-04 · Status: menunggu review

## 1. Tujuan

Pengguna (Google maupun tamu) bisa menghapus sebuah chat **dari sisinya sendiri**. Lawan bicara tetap menyimpan seluruh riwayat. Bila ada pesan baru, chat muncul lagi dengan pesan baru saja.

**Kriteria sukses**
- Setelah dihapus, chat hilang dari daftar saya dan pesan lama tidak pernah dikirim lagi ke saya (termasuk lewat catch-up dan scroll riwayat).
- Lawan bicara tidak melihat perubahan apa pun.
- Tab/perangkat lain milik saya ikut menghapus chat itu.

**Di luar cakupan:** hapus untuk kedua pihak; hapus pesan satuan; widget (menyusul bila dibutuhkan); pembatalan (undo).

## 2. Data

Migrasi `0005_cleared_seq`:

```sql
ALTER TABLE conversation_members ADD COLUMN cleared_seq bigint;
```

`NULL` = never cleared (so clearing a zero-message chat still hides it; `0` would be ambiguous).

Down: `DROP COLUMN cleared_seq`. Tidak ada data yang dihapus; retensi 1 bulan berjalan seperti sekarang.

## 3. Backend

- `DELETE /api/conversations/:id` (auth, Google & tamu):
  - Bukan anggota / tidak ada → 404 `not_found` (pola yang ada).
  - `UPDATE conversation_members SET cleared_seq = c.last_seq FROM conversations c WHERE …` → 204.
  - Setelah sukses, hub mengirim `{"type":"conversation_cleared","conversation_id":…}` **hanya ke koneksi milik pemanggil**.
- `GET /api/conversations`: kecualikan baris dengan `c.last_seq <= m.cleared_seq` milik saya.
- `GET /api/conversations/:id/messages`: selalu tambahkan `seq > cleared_seq` milik pemanggil (berlaku untuk `after_seq`, `before_seq`, dan tanpa parameter).
- Frame `message` real-time tidak berubah: pesan baru selalu `seq > cleared_seq`.
- Logika di `internal/chat` (dipakai REST) agar aturan tertulis sekali; query di `internal/store`.

## 4. Web

- Item **"Hapus chat"** di dropdown yang sama dengan Blokir (`ChatHeader` / `BlockMenu`). Dropdown kini juga tampil untuk tamu dan percakapan yang berakhir, dengan item Blokir/Buka blokir hanya bila berlaku seperti sekarang.
- Klik → dialog konfirmasi: judul "Hapus chat ini?", isi "Chat ini dihapus dari daftarmu. Lawan bicara tetap menyimpan riwayatnya.", tombol Batal / Hapus.
- Sukses:
  - Google: chat dikeluarkan dari daftar dan riwayatnya dikosongkan di state; kembali ke daftar (mobile) / tampilan "pilih percakapan" (desktop).
  - Tamu: percakapan tetap terbuka, riwayat dikosongkan, composer tetap aktif.
- Frame `conversation_cleared` (baru; `conversation_deleted` tetap berarti lawan menghapus akun) di tab lain milik pemanggil: Google mengeluarkan chat dari daftar, tamu hanya mengosongkan riwayat, tanpa toast.
- Gagal → toast pesan error yang ada.

Teks (EN / ID): "Delete chat"/"Hapus chat"; "Delete this chat?"/"Hapus chat ini?"; "This chat is removed from your list. The other person keeps their history."/"Chat ini dihapus dari daftarmu. Lawan bicara tetap menyimpan riwayatnya."; "Delete"/"Hapus"; "Cancel"/"Batal".

## 5. Pengujian

- **Go (`-race`, Postgres asli):**
  - Hapus → hilang dari `GET /conversations`; `GET messages` (tanpa param, `after_seq=0`, `before_seq=<besar>`) tidak mengembalikan pesan lama.
  - Pesan baru dari lawan → muncul lagi di daftar dengan hanya pesan baru.
  - Lawan tetap melihat semua pesan; `last_read_seq`/read receipt tidak terpengaruh.
  - Bukan anggota → 404; tamu bisa menghapus percakapannya.
  - Frame `conversation_cleared` hanya ke koneksi pemanggil, tidak ke lawan.
- **Vitest (web):** handler hapus mengosongkan riwayat & mengeluarkan chat (Google) / mengosongkan riwayat saja (tamu); dialog konfirmasi memanggil API hanya setelah "Hapus".
- **Playwright:** A hapus chat → hilang dari daftar A; B masih melihat riwayat; B kirim pesan → chat muncul lagi di A hanya dengan pesan baru.

**Verifikasi sendiri:**

```bash
go test -race ./...
cd web && npx vitest run
```
