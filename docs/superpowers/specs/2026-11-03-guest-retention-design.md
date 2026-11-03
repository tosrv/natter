# Design Doc: Percakapan Guest Tetap di Sisi Developer

- **Tanggal:** 2026-11-03
- **Status:** Menunggu review pemilik produk
- **Mengubah:** aturan PRD bagian 4 "Setelah sesi ditutup, akun dan seluruh percakapan (termasuk info kontak) dihapus" dan kriteria 5. Perubahan ini atas keputusan pemilik produk.
- **Alasan:** saat guest berakhir, developer kehilangan percakapan beserta info kontak yang sukarela diberikan guest, sehingga tidak bisa menghubungi balik.

## 1. Keputusan

| # | Topik | Keputusan | Trade-off |
|---|-------|-----------|-----------|
| 1 | Cakupan | **Hanya guest.** Pengguna Google yang menghapus akunnya tetap terhapus total di kedua sisi (tidak berubah). | — |
| 2 | Yang disimpan | **Seluruh percakapan guest** disimpan di sisi developer bila berisi minimal satu pesan. Guest tanpa pesan dihapus total. Percakapan yang disimpan tunduk pada retensi 1 bulan sejak aktivitas terakhir. | Info kontak guest tersimpan sampai 1 bulan; dinyatakan jujur di dialog kontak, `/privacy`, dan `/terms`. |
| 3 | Akun guest | **Dinonaktifkan, tidak dihapus** (`users.deactivated_at`). Sesi dihapus, `username` dikosongkan, display name "Guest XXXX" tetap, dan slot dikembalikan. | Ada baris akun yang tetap tersimpan sampai percakapannya kena retensi; dibersihkan oleh job harian. |

## 2. Data

Migrasi `0004_user_deactivation`:

```sql
ALTER TABLE users ADD COLUMN deactivated_at timestamptz;
CREATE INDEX ON users (deactivated_at) WHERE deactivated_at IS NOT NULL;
```

FK `ON DELETE CASCADE` yang ada tidak diubah. Penonaktifan tidak menghapus baris user, jadi cascade tidak terpicu.

## 3. Mengakhiri guest

Berlaku untuk tiga jalur yang sudah ada, dan semuanya memakai satu fungsi store yang sama:
- pembersih guest (30 menit tanpa aktivitas);
- `DELETE /api/me` oleh guest ("Akhiri sesi");
- `POST /api/auth/logout` oleh guest.

`store.EndGuest(ctx, id) (result EndGuestResult, err error)` dijalankan dalam satu transaksi:

1. Kunci baris user (`kind='guest' AND deactivated_at IS NULL`). Kalau tidak ditemukan, kembalikan `ErrNotFound`.
2. Kalau guest **tidak punya pesan** (`NOT EXISTS (SELECT 1 FROM messages WHERE sender_id = id)`): `DELETE FROM users` (cascade seperti sekarang), lalu `used - 1`. Hasil: `Deleted`, dengan daftar `DeletedConversation` untuk event yang sudah ada.
3. Kalau ada pesan: `UPDATE users SET deactivated_at = now(), username = NULL`, `DELETE FROM sessions WHERE user_id = id`, lalu `used - 1`. Hasil: `Deactivated`, dengan daftar `{conversation_id, peer_id}`.

Setelah commit:
- Hasil `Deleted`: kirim event `conversation_deleted` seperti sekarang.
- Hasil `Deactivated`: kirim frame baru `{type:"conversation_ended", conversation_id}` ke setiap lawan bicara yang online.
- Dalam kedua kasus, tutup koneksi guest dengan `Disconnect`.

Slot hanya dikurangi sekali per guest. Guest yang sudah nonaktif tidak bisa diakhiri lagi, karena langkah 1 mengembalikan `ErrNotFound`.

## 4. Aturan untuk akun yang dinonaktifkan

- **Autentikasi:** sesi sudah dihapus, jadi permintaan berikutnya mendapat 401.
- **`chat.Send` ke lawan bicara yang nonaktif:** ditolak dengan kode baru `peer_ended` (410). Kode ini dicek setelah pemeriksaan keanggotaan dan blokir, dan hanya relevan bagi developer.
- **`OpenConversation` dan `SearchUsers`:** akun yang nonaktif dikecualikan (`not_found`).
- **`typing` dan `read` ke akun nonaktif:** tidak diteruskan, karena orangnya memang tidak terhubung.
- **`Conversations`:** ringkasan mendapat field `peer_ended bool`.
- **`Messages`:** tetap bisa dibaca oleh developer.
- **`ExpiredGuests`:** hanya mengembalikan guest dengan `deactivated_at IS NULL`.

## 5. Pembersihan

Job harian (bersama retensi pukul 03:00) menghapus total guest yang sudah nonaktif dan tidak lagi memiliki percakapan:

```sql
DELETE FROM users u
WHERE u.kind = 'guest'
  AND u.deactivated_at IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM conversations c WHERE c.user_a = u.id OR c.user_b = u.id)
```

Langkah ini tidak mengubah `used`, karena slot-nya sudah dikembalikan saat guest dinonaktifkan.

## 6. Frontend

- **Ringkasan percakapan:** tipe `ConversationSummary` mendapat field `peer_ended: boolean`.
- **Di sisi developer, untuk percakapan dengan `peer_ended`:**
  - label "Sesi berakhir" / "Session ended" di daftar percakapan dan header;
  - kotak tulis pesan diganti teks: "Guest ini sudah meninggalkan chat. Hubungi lewat kontak yang ia berikan, kalau ada." / "This guest has left. Reach out through the contact they shared, if any."
- **Frame `conversation_ended`:** tandai percakapan sebagai berakhir tanpa reload. Kalau percakapan itu sedang terbuka, tampilkan toast.
- **Error `peer_ended`:** tampilkan toast dengan pesan yang sama, buang entri outbox untuk percakapan itu, dan tandai percakapannya sebagai berakhir.
- **Dialog kontak guest:** teks baru "Jawaban Anda disimpan bersama percakapan ini hingga 1 bulan agar developer bisa menghubungi Anda." / "Your answer is kept with this conversation for up to 1 month so the developer can reach you."
- **i18n:** semua teks baru ditulis di `en.json` dan `id.json`.

## 7. Teks legal dan dokumentasi

- **`/privacy`:** saat sesi guest berakhir, akunnya dinonaktifkan dan tidak bisa dipakai lagi. Percakapan, termasuk info kontak sukarela, disimpan di sisi developer hingga 1 bulan sejak pesan terakhir, lalu dihapus otomatis. Guest yang tidak mengirim pesan dihapus seketika. "Terakhir diperbarui" diubah ke tanggal rilis.
- **`/terms`:** bagian retensi diubah dengan aturan yang sama.
- **README (EN dan ID) dan `docs/operations*.md`:** baris tentang penghapusan guest disesuaikan.

## 8. Testing

| Area | Test | Verifikasi |
|------|------|------------|
| Store | `TestEndGuestWithoutMessagesDeletes` (baris hilang, `used - 1`); `TestEndGuestWithMessagesDeactivates` (baris tetap, `deactivated_at` terisi, `username` NULL, sesi hilang, percakapan dan pesan tetap, `used - 1`); `TestEndGuestTwice` (kedua kalinya `ErrNotFound`, `used` tidak turun lagi); `TestPurgeDeactivatedGuests` (hanya guest nonaktif tanpa percakapan yang terhapus) | `go test -race ./internal/store/` |
| Chat | `TestSendToEndedGuest` (developer → `peer_ended` 410); `TestEndedGuestHiddenFromOpen`; `TestGoogleDeleteAccountStillFull` (regresi) | `go test -race ./internal/chat/` |
| Jobs | `TestCleanupGuestsDeactivates` (guest dengan pesan dinonaktifkan; tanpa pesan dihapus); retensi lalu purge menghapus guest yatim | `go test -race ./internal/jobs/` |
| API dan realtime | Guest `DELETE /api/me` → 204, sesi 401; developer online menerima `conversation_ended`; `peer_ended` di ringkasan | `go test -race ./internal/api/ ./internal/realtime/` |
| Web | Helper murni `markEnded(state, convId)`; i18n parity | `npx vitest run` |
| E2E | Guest mengisi kontak lalu mengakhiri sesi → developer masih melihat percakapan dan kontak, label "Session ended", kotak tulis pesan nonaktif | `npx playwright test` |

## 9. Di luar scope

Developer menghapus percakapan guest secara manual; ekspor kontak; masa simpan khusus untuk guest; perubahan perilaku hapus akun Google.
