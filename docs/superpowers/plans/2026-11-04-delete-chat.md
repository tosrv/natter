# Delete Chat Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Pengguna (Google & tamu) bisa menghapus chat dari sisinya sendiri; lawan bicara tetap menyimpan riwayat.

**Architecture:** Kolom `conversation_members.cleared_seq` menandai batas hapus per anggota. `DELETE /api/conversations/:id` mengisinya dengan `last_seq`; daftar percakapan dan riwayat pesan menyaring berdasarkan batas itu. Hub mengirim `conversation_deleted` hanya ke koneksi pemanggil. Web menambah item "Hapus chat" di dropdown Blokir.

**Tech Stack:** Go/Gin/pgx/golang-migrate; React + shadcn; Vitest; Playwright.

**Spec:** `docs/superpowers/specs/2026-11-04-delete-chat-design.md`

## Global Constraints

- Migrasi `0005_cleared_seq`: `ALTER TABLE conversation_members ADD COLUMN cleared_seq bigint NOT NULL DEFAULT 0;` (down: `DROP COLUMN cleared_seq`).
- `DELETE /api/conversations/:id` → 204; bukan anggota/tidak ada → 404 `not_found`; berlaku untuk Google & tamu.
- Daftar: kecualikan `c.last_seq <= m.cleared_seq` (milik saya). Riwayat: selalu `seq > cleared_seq` milik pemanggil, untuk `after_seq`, `before_seq`, dan tanpa parameter.
- Frame `{"type":"conversation_deleted","conversation_id":…}` hanya ke koneksi pemanggil, tidak ke lawan.
- Tidak ada data yang dihapus; widget tidak diubah.
- Teks (EN / ID): "Delete chat"/"Hapus chat"; "Delete this chat?"/"Hapus chat ini?"; "This chat is removed from your list. The other person keeps their history."/"Chat ini dihapus dari daftarmu. Lawan bicara tetap menyimpan riwayatnya."; "Delete"/"Hapus"; "Cancel"/"Batal".
- Setiap task: commit + push ke `feat/delete-chat`.

## Review Focus

1. Hapus dua kali tanpa pesan baru → tetap 204, idempoten (Task 1: `TestClearTwice`).
2. Hapus saat ada pesan yang sedang dikirim bersamaan → pesan yang mendapat seq setelah clear tetap terlihat (Task 1: `TestClearThenSendShowsNew` mengecek `seq > cleared_seq` muncul).
3. Unread count/read receipt lawan tidak rusak setelah saya hapus (Task 1: lawan tetap melihat `peer_last_read_seq` saya apa adanya).
4. Tamu menghapus lalu langsung mengirim pesan → pesan terkirim dan percakapan tetap ada di daftar tamu (Task 1 & 2).
5. Developer menghapus chat tamu yang sudah berakhir (`peer_ended`) → boleh dan hilang dari daftar (Task 1: `TestClearEndedGuestConversation`).

---

### Task 1: Backend

**Files:**
- Create: `migrations/0005_cleared_seq.up.sql`, `migrations/0005_cleared_seq.down.sql`
- Modify: `internal/store/conversations.go` (`ListConversations`, baru `ClearConversation`), `internal/store/messages.go` (`ListMessages`), `internal/chat/service.go` (`Messages`, baru `ClearConversation`), `internal/api/router.go` (route + `Notifier`), `internal/api/handlers.go` (handler), `internal/realtime/hub.go` (baru `NotifyConversationCleared`)
- Test: `internal/store/conversations_test.go` (atau file store test yang ada), `internal/chat/service_test.go`, `internal/api/api_test.go`, `internal/realtime/hub_test.go`

**Interfaces:**
- Produces:
  - `func (s *Store) ClearConversation(ctx context.Context, convID, userID uuid.UUID) error` — `ErrNotFound` bila bukan anggota.
  - `ListMessages(ctx, convID, viewerID uuid.UUID, afterSeq, beforeSeq int64, limit int)` — parameter `viewerID` baru; filter `seq > cleared_seq` milik viewer. Sesuaikan semua pemanggil.
  - `func (s *Service) ClearConversation(ctx context.Context, u store.User, convID uuid.UUID) error` — cek keanggotaan dengan pola `s.peer` yang ada, tetapi **izinkan** percakapan dengan peer berakhir/terblokir (hapus selalu boleh bagi anggota).
  - `Notifier.NotifyConversationCleared(userID, convID uuid.UUID)`; `Hub.NotifyConversationCleared` mengirim frame `conversation_deleted` ke koneksi `userID` saja.
  - Route `authed.DELETE("/conversations/:id", h.clearConversation)` → 204.

- [ ] **Step 1: Test gagal:**
  - store: setelah `ClearConversation(conv, A)`, `ListConversations(A)` tidak memuat conv; `ListConversations(B)` memuat; `ListMessages(conv, A, 0, 0, 50)` kosong; `ListMessages(conv, A, 0, 1<<62, 50)` kosong; `ListMessages(conv, B, …)` berisi semua. `TestClearThenSendShowsNew`: B kirim pesan baru → `ListConversations(A)` memuat conv dan `ListMessages(conv, A, …)` hanya pesan baru. `TestClearTwice`: dua kali → tanpa error. Bukan anggota → `ErrNotFound`.
  - api: `DELETE /api/conversations/:id` → 204; bukan anggota → 404 `not_found`; tamu → 204; `TestClearEndedGuestConversation` (developer, peer guest berakhir) → 204 dan hilang dari daftar; `peer_last_read_seq` yang dilihat B tidak berubah.
  - realtime: A punya 2 koneksi, B 1 koneksi; A hapus → kedua koneksi A menerima `conversation_deleted` dengan id tsb; B tidak menerima frame apa pun (tunggu singkat, pola test hub yang ada).
- [ ] **Step 2:** `go test -race ./internal/...` → FAIL.
- [ ] **Step 3:** Implementasi.
- [ ] **Step 4:** `go vet ./... && go test -race ./...` → PASS.
- [ ] **Step 5:** Commit `feat(chat): delete chat for me (cleared_seq)` + push.

### Task 2: Web

**Files:**
- Modify: `web/src/lib/api.ts`, `web/src/hooks/useChat.ts`, `web/src/components/ChatHeader.tsx` (dan komponen `BlockMenu`), `web/src/pages/Chat.tsx`, `web/src/i18n/en.json`, `web/src/i18n/id.json`
- Create (bila perlu): `web/src/components/DeleteChatDialog.tsx`
- Test: `web/src/hooks/` atau `web/src/lib/` test yang relevan, `web/src/components/ChatHeader.test.tsx` (atau sejenis)

**Interfaces:**
- Consumes: Task 1 endpoint & frame.
- Produces: `deleteConversation(id: string): Promise<void>`; `useChat().deleteChat(id: string): Promise<void>` — Google: keluarkan conv dari `convs`, kosongkan pesan/outbox conv itu, `setActiveId(null)`; tamu: kosongkan pesan saja, conv tetap aktif. Handler frame `conversation_deleted` yang ada: untuk tamu kosongkan pesan alih-alih mengeluarkan conv. `ChatHeader` props baru `onDelete?: () => Promise<void> | void`; dropdown tampil bila `onDelete` atau (`onBlock` && `onUnblock`) ada; item Blokir/Buka blokir hanya bila keduanya ada (perilaku sekarang).

- [ ] **Step 1: Test gagal:**
  - dropdown: dengan `onDelete` saja (tamu) → item "Delete chat" ada, item Block tidak ada; klik → dialog "Delete this chat?"; "Cancel" → `onDelete` tidak dipanggil; "Delete" → dipanggil sekali.
  - logika hapus (fungsi state murni atau hook test yang ada): Google → conv hilang dari daftar dan `activeId` null; tamu → conv tetap, pesannya kosong.
- [ ] **Step 2:** `cd web && npx vitest run` → FAIL.
- [ ] **Step 3:** Implementasi; error → toast pesan error yang ada.
- [ ] **Step 4:** `npx vitest run && npx tsc --noEmit && npm run build` → PASS.
- [ ] **Step 5:** Commit `feat(web): delete chat menu` + push.

### Task 3: E2E

**Files:**
- Create: `web/e2e/delete-chat.spec.ts`

- [ ] **Step 1:** Tulis spec (pola login palsu dari `chat.spec.ts`, nilai unik per run): A dan B saling kirim pesan; A buka dropdown → "Delete chat" → "Delete" → chat hilang dari daftar A; B masih melihat pesan lama; B kirim pesan baru → chat muncul lagi di A dan hanya berisi pesan baru (pesan lama tidak terlihat).
- [ ] **Step 2:** Jalankan lokal seperti CI (server :8099): `cd web && E2E_BASE_URL=http://localhost:8099 npx playwright test` → PASS (kecuali yang memang di-skip).
- [ ] **Step 3:** Commit `test(e2e): delete chat` + push.
