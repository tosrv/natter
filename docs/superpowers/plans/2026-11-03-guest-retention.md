# Guest Retention Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Saat guest berakhir, akun guest yang punya pesan dinonaktifkan (bukan dihapus) sehingga percakapannya — termasuk info kontak — tetap ada di sisi developer sampai retensi 1 bulan.

**Architecture:** Migrasi `0004` menambah `users.deactivated_at`. Satu fungsi store `EndGuest` (hapus total bila tanpa pesan, nonaktifkan bila ada pesan) dipakai oleh tiga jalur: pembersih 30 menit, logout guest, dan `DELETE /api/me` guest — lewat satu fungsi chat `EndGuest`. Notifier/hub mendapat event `conversation_ended`. Web menandai percakapan "Sesi berakhir".

**Tech Stack:** Go 1.24 + Gin + pgx; React + TS + Vitest; Playwright.

**Spec:** `docs/superpowers/specs/2026-11-03-guest-retention-design.md`

## Global Constraints

- Hanya guest. Hapus akun Google tidak berubah (tetap total, event `conversation_deleted`).
- Guest tanpa pesan (`NOT EXISTS messages WHERE sender_id = id`) → hapus total seperti sekarang. Guest dengan pesan → `deactivated_at = now()`, `username = NULL`, semua sesi dihapus, display name tetap; `user_slots.used - 1` tepat sekali.
- Mengakhiri guest yang sudah nonaktif atau tidak ada → `store.ErrNotFound`; `used` tidak berubah.
- `chat.Send` ke lawan bicara nonaktif → kode baru `peer_ended` (HTTP 410), dicek setelah keanggotaan dan blokir.
- `OpenConversation` dan pencarian mengecualikan akun nonaktif; `typing`/`read` ke akun nonaktif tidak diteruskan (tanpa frame error).
- `ConversationSummary.PeerEnded bool` (JSON `peer_ended`). `GET /messages` tetap terbaca developer.
- `ExpiredGuests` hanya guest dengan `deactivated_at IS NULL`.
- Job harian 03:00 (bersama retensi) menghapus guest nonaktif tanpa percakapan; tidak mengubah `used`.
- Frame WS baru: `{type:"conversation_ended", conversation_id}` ke lawan bicara online; koneksi guest ditutup (`Disconnect`).
- Teks UI (EN/ID): label "Session ended" / "Sesi berakhir"; composer "This guest has left. Reach out through the contact they shared, if any." / "Guest ini sudah meninggalkan chat. Hubungi lewat kontak yang ia berikan, kalau ada."; dialog kontak "Your answer is kept with this conversation for up to 1 month so the developer can reach you." / "Jawaban Anda disimpan bersama percakapan ini hingga 1 bulan agar developer bisa menghubungi Anda."
- `/privacy` dan `/terms` (EN/ID) diperbarui sesuai spec §7; tanggal "Last updated: November 3, 2026" / "Terakhir diperbarui: 3 November 2026" tetap (rilis di hari yang sama).
- Semua test Go dengan `-race`. Branch `feat/guest-retention`; push `git push -u origin feat/guest-retention`; trailer: `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>` dan `Claude-Session: https://claude.ai/code/session_017NqdLCXr2x4X6KWjddprEM`.

## Review Focus

1. **Guest berakhir tepat saat developer sedang mengetik balasan** → kiriman developer ditolak `peer_ended`, toast tampil, entri outbox dibuang (tidak dikirim ulang terus-menerus saat reconnect). (Test di Task 3.)
2. **Guest mengirim pesan di detik yang sama dengan pembersih menjalankan `EndGuest`** → hasilnya konsisten: guest nonaktif dan pesan tetap ada, atau pesan gagal `not_found`/401 — tidak pernah "guest terhapus total tapi pesan baru terlanjur ada". (Test di Task 1: `EndGuest` mengunci baris user `FOR UPDATE` dan menghitung pesan di transaksi yang sama.)
3. **Guest nonaktif yang diblokir developer** → percakapan tetap terlihat dengan label berakhir; tidak ada error. (Test di Task 2.)
4. **Retensi menghapus percakapan guest nonaktif lalu purge** → baris guest hilang, `used` tidak berubah; tidak ada guest aktif yang ikut terhapus. (Test di Task 1.)
5. **Guest nonaktif mencoba memakai cookie lama** → 401, UI kembali ke landing; tidak bisa membuat guest baru dengan sesi lama. (Test di Task 2.)

---

## File Structure

```
migrations/0004_user_deactivation.up.sql / .down.sql
internal/store/guests.go            EndGuest, EndGuestResult, EndedConversation, PurgeDeactivatedGuests
internal/store/users.go             scanUser: DeactivatedAt; hapus DeleteGuest (diganti EndGuest)
internal/store/cleanup.go           ExpiredGuests: hanya aktif
internal/store/conversations.go     ConversationSummary.PeerEnded
internal/store/search.go            kecualikan nonaktif
internal/chat/guests.go             Service.EndGuest
internal/chat/service.go            Send peer_ended; OpenConversation; PeerOf/MarkRead deliver=false bila nonaktif; DeleteAccount guest → EndGuest
internal/chat/errors.go             peer_ended 410
internal/auth/session.go            Logout guest → chat.EndGuest
internal/api/router.go, handlers.go Notifier.NotifyConversationEnded
internal/realtime/hub.go            NotifyConversationEnded
internal/jobs/jobs.go               CleanupGuests → EndGuest; daily purge
web/src/lib/api.ts, socket.ts, hooks/useChat.ts, components/*, i18n, pages/{Privacy,Terms}
web/e2e/guest-retention.spec.ts
```

---

### Task 1: Store — nonaktifkan guest

**Files:**
- Create: `migrations/0004_user_deactivation.up.sql`, `.down.sql`, `internal/store/guests.go`, `internal/store/guests_test.go`
- Modify: `internal/store/users.go`, `internal/store/cleanup.go`, `internal/store/conversations.go`, `internal/store/search.go`, semua pemanggil `DeleteGuest` di test store

**Interfaces:**
- Produces:
  - `User.DeactivatedAt *time.Time`.
  - `type EndGuestOutcome int` dengan konstanta `GuestDeleted`, `GuestDeactivated`.
  - `type EndedConversation struct{ ConversationID, PeerID uuid.UUID }`.
  - `(*Store).EndGuest(ctx, id uuid.UUID) (EndGuestOutcome, []EndedConversation, error)` — satu tx: `SELECT … FROM users WHERE id=$1 AND kind='guest' AND deactivated_at IS NULL FOR UPDATE` (tidak ada → `ErrNotFound`); kumpulkan pasangan percakapan; hitung pesan; tanpa pesan → `DELETE` + `used-1` → `GuestDeleted`; ada pesan → `UPDATE … SET deactivated_at=now(), username=NULL` + `DELETE FROM sessions WHERE user_id=$1` + `used-1` → `GuestDeactivated`.
  - `(*Store).PurgeDeactivatedGuests(ctx) (int64, error)` — SQL spec §5.
  - `ExpiredGuests` hanya `deactivated_at IS NULL`. `ConversationSummary.PeerEnded bool` (JSON `peer_ended`) = lawan bicara punya `deactivated_at`. `SearchUsers` mengecualikan nonaktif.
  - `DeleteGuest` dihapus; pemanggil di luar store (jobs, auth) diganti di Task 2 — untuk task ini, sementara arahkan mereka ke `EndGuest` agar tetap terkompilasi.

- [ ] **Step 1: Tulis test** — `TestEndGuestWithoutMessagesDeletes` (guest + percakapan dengan developer tanpa pesan → `GuestDeleted`, baris user dan percakapan hilang, `used` turun 1). `TestEndGuestWithMessagesDeactivates` (guest kirim 2 pesan → `GuestDeactivated`, `GetUser` mengembalikan `DeactivatedAt != nil`, `Username == nil`, display name tetap `Guest XXXX`; `UserBySession` dengan token lama → `ErrNotFound`; `ListMessages` masih 2 pesan; `used` turun 1; `EndedConversation` berisi developer sebagai peer). `TestEndGuestTwice` (kedua kali `ErrNotFound`, `used` hanya turun sekali). `TestEndGuestConcurrentWithSend` (Review Focus 2: goroutine A `EndGuest`, goroutine B `SendMessage` dari guest bersamaan, 20 putaran → tidak pernah hasil `GuestDeleted` sementara ada baris pesan dari guest). `TestExpiredGuestsSkipsDeactivated`. `TestPurgeDeactivatedGuests` (Review Focus 4: guest nonaktif yang percakapannya dihapus retensi → terhapus; guest nonaktif dengan percakapan masih ada → tetap; guest aktif → tetap; `used` tidak berubah). `TestSummaryPeerEnded`, `TestSearchExcludesDeactivated`.
- [ ] **Step 2:** `go test -race ./internal/store/` → FAIL.
- [ ] **Step 3:** Implementasi.
- [ ] **Step 4:** `go test -race ./...` → PASS; `go build ./...`.
- [ ] **Step 5:** Commit `feat(store): deactivate guests instead of deleting` + push.

### Task 2: Chat, auth, jobs, API, realtime

**Files:**
- Create: `internal/chat/guests.go`, `internal/chat/guests_test.go`
- Modify: `internal/chat/service.go`, `internal/chat/errors.go`, `internal/auth/session.go`, `internal/jobs/jobs.go`, `internal/api/router.go`, `internal/api/handlers.go`, `internal/realtime/hub.go`, `cmd/server/main.go`
- Test: `internal/jobs/jobs_test.go`, `internal/api/*_test.go`, `internal/realtime/hub_test.go`, `internal/auth/auth_test.go`

**Interfaces:**
- Consumes: Task 1.
- Produces:
  - `chat.ErrPeerEnded` = `*Error{Code:"peer_ended", Status:410}`.
  - `(*Service).EndGuest(ctx, guestID uuid.UUID) (store.EndGuestOutcome, []store.EndedConversation, error)` — membungkus store; dipakai oleh `DeleteAccount` (bila `u.Kind=="guest"`), `auth.Logout` (guest), dan `jobs.CleanupGuests`.
  - `DeleteAccount` untuk guest mengembalikan peers untuk notifier yang sesuai hasil (lihat di bawah); untuk Google tidak berubah.
  - Notifier: `api.Notifier` mendapat `NotifyGuestEnded(outcome store.EndGuestOutcome, convs []store.EndedConversation, guestID uuid.UUID)` — `GuestDeleted` → frame `conversation_deleted` (perilaku lama), `GuestDeactivated` → frame `conversation_ended`; keduanya lalu `Disconnect(guestID)`. Hub mengimplementasikan; `jobs` memakai interface yang sama (ganti `Disconnector`).
  - `Send`: lawan bicara nonaktif → `ErrPeerEnded`. `OpenConversation` ke nonaktif → `not_found`. `PeerOf`/`MarkRead` → `deliver=false` bila lawan bicara nonaktif.
  - `jobs.Run`: job harian 03:00 memanggil `RunRetention` lalu `PurgeDeactivatedGuests` (log jumlahnya).

- [ ] **Step 1: Tulis test** — chat: `TestSendToEndedGuest` (developer → 410 `peer_ended`), `TestEndedGuestHiddenFromOpen`, `TestBlockedEndedGuestStillListed` (Review Focus 3: developer memblokir guest lalu guest berakhir → `Conversations` developer memuat percakapan dengan `PeerEnded=true` dan `BlockedByMe=true`, tanpa error), `TestGoogleDeleteAccountStillFull` (regresi: percakapan hilang dari lawan bicara). auth: `TestGuestLogoutDeactivates` (guest dengan pesan logout → 204; token lama → 401; developer masih melihat percakapan) dan `TestEndedGuestCookieRejected` (Review Focus 5: `GET /api/me` dengan cookie guest nonaktif → 401; `POST /api/auth/guest` dengan cookie itu membuat guest **baru**, bukan memulihkan yang lama). api: guest `DELETE /api/me` → 204 dan notifier dipanggil dengan `GuestDeactivated`. jobs: `TestCleanupGuestsDeactivates` (guest berpesan → nonaktif; tanpa pesan → terhapus; notifier menerima outcome yang benar). realtime: `TestNotifyGuestEnded` (developer online menerima `conversation_ended`; koneksi guest ditutup; outcome `GuestDeleted` tetap mengirim `conversation_deleted`).
- [ ] **Step 2:** `go test -race ./internal/...` → FAIL.
- [ ] **Step 3:** Implementasi; perbarui `main.go` wiring.
- [ ] **Step 4:** `go test -race ./...` → PASS; `go build ./...`.
- [ ] **Step 5:** Commit `feat: guest sessions end without deleting the developer's history` + push.

### Task 3: Web — percakapan berakhir

**Files:**
- Modify: `web/src/lib/api.ts`, `web/src/lib/socket.ts`, `web/src/hooks/useChat.ts`, `web/src/components/ConversationList.tsx`, `web/src/components/ChatHeader.tsx`, `web/src/pages/Chat.tsx` (composer), `web/src/components/GuestContactDialog.tsx`, `web/src/i18n/{en,id}.json`
- Test: `web/src/hooks/useChat.test.ts`, `web/src/lib/outbox.test.ts`

**Interfaces:**
- Consumes: Task 2 (`peer_ended` field, frame `conversation_ended`, error `peer_ended`).
- Produces: `ConversationSummary.peer_ended: boolean`; helper murni `markEnded(state, convId)` di `useChat.ts`; frame `conversation_ended` → `markEnded` + toast bila aktif; error frame `peer_ended` → `markEnded`, `Outbox.dropConversation(convId)`, toast; label dan composer sesuai Global Constraints; dialog kontak memakai teks baru.

- [ ] **Step 1: Tulis test** — `markEnded` menandai percakapan yang benar dan tidak menyentuh lainnya; penanganan error `peer_ended` membuang entri outbox percakapan itu (Review Focus 1: helper murni `handlePeerEnded(state, outbox, convId)` atau setara; setelah dipanggil, `outbox.pending()` tidak berisi pesan percakapan itu).
- [ ] **Step 2:** `npx vitest run` → FAIL. **Step 3:** Implementasi. **Step 4:** `npx vitest run` PASS; `npm run build` sukses.
- [ ] **Step 5:** Commit `feat(web): show ended guest conversations` + push.

### Task 4: Teks legal, dokumentasi, E2E

**Files:**
- Create: `web/e2e/guest-retention.spec.ts`
- Modify: `web/src/i18n/{en,id}.json` (bagian `privacy` dan `terms`), `README.md`, `README.id.md`, `docs/operations.md`, `docs/operations.id.md`

**Interfaces:**
- Consumes: semua task; fake login `POST /api/test/login`; tombol guest.

- [ ] **Step 1:** Perbarui teks privasi/syarat (EN/ID) sesuai spec §7 dan baris penghapusan guest di README (EN/ID) serta panduan operasional (EN/ID).
- [ ] **Step 2: Tulis E2E** — developer login palsu (`dev@e2e.test`, username `dev`); guest masuk, dialog kontak menampilkan teks baru, isi `contact-<run>@example.com`; guest membuka Settings → "End session"; developer: percakapan guest masih ada, berisi kontak, label "Session ended", composer menampilkan teks "This guest has left…" dan tidak ada input pesan.
- [ ] **Step 3:** Stack lokal (postgres container + server :8099, `E2E_FAKE_LOGIN=1 COOKIE_SECURE=false DEVELOPER_EMAIL=dev@e2e.test STATIC_DIR=web/dist`) → `E2E_BASE_URL=http://localhost:8099 npx playwright test` semua PASS; `go test -race ./...`; `npx vitest run`.
- [ ] **Step 4:** Commit `docs+test: guest retention policy text and e2e` + push.
