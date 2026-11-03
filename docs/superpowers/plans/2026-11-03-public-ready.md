# Public-Ready Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Blokir antar-pengguna, Cloudflare Turnstile pada pembuatan guest, header keamanan, dan halaman `/terms` + `/privacy` dengan persetujuan implisit.

**Architecture:** Migrasi `0003_blocks`; aturan blokir di paket `chat` (sumber tunggal untuk REST dan WebSocket); `auth` memverifikasi Turnstile sebelum rate limit/slot; middleware `SecurityHeaders` global di router; web menambah UI blokir, widget Turnstile, dan dua halaman legal.

**Tech Stack:** Go 1.24 + Gin + pgx; React 18 + Vite + TS + Tailwind + shadcn; Vitest; Playwright; Cloudflare Turnstile.

**Spec:** `docs/superpowers/specs/2026-11-03-public-ready-design.md`

## Global Constraints

- Blokir: "terkait blokir" = A memblokir B **atau** B memblokir A. Kirim pesan dan `OpenConversation` antar pihak terkait blokir → `not_found` (404); pencarian tidak memuat pihak terkait blokir; event `typing`/`read` tidak diteruskan (dan **tidak** ada frame error untuk typing/read); blokir diri sendiri → `bad_request` (400); target tidak ada → `not_found`; guest memblokir → `forbidden` (403); developer boleh memblokir guest; idempoten (blok/unblok dua kali aman).
- Pihak yang diblokir tidak pernah menerima penanda: hanya `blocked_by_me` dikirim ke klien.
- Turnstile: env `TURNSTILE_SITE_KEY` + `TURNSTILE_SECRET_KEY`; keduanya kosong → nonaktif; hanya salah satu → `config.Load` error. Verifikasi `POST https://challenges.cloudflare.com/turnstile/v0/siteverify` (form `secret`, `response`, `remoteip`), timeout **5 s**, dijalankan **sebelum** rate limit guest dan slot. Gagal/kosong → 400 `captcha_failed`; galat jaringan/timeout/non-200 → 503 `captcha_unavailable`.
- Test key Cloudflare (selalu lolos): site `1x00000000000000000000AA`, secret `1x0000000000000000000000000000000AA`.
- `GET /api/config` publik → `{"turnstile_site_key": "..."}`.
- Header (semua respons): CSP persis seperti spec §5; `X-Content-Type-Options: nosniff`; `Referrer-Policy: strict-origin-when-cross-origin`; `Permissions-Policy: camera=(), microphone=(), geolocation=()`; `X-Frame-Options: DENY`; `Strict-Transport-Security: max-age=31536000` hanya bila `COOKIE_SECURE=true`.
- Halaman `/terms` dan `/privacy` (EN/ID), "Terakhir diperbarui: 3 November 2026" / "Last updated: November 3, 2026". Isi sesuai spec §6.
- Semua teks UI baru di `en.json` dan `id.json`. Semua test Go dengan `-race`.
- Branch `feat/public`; push `git push -u origin feat/public`; trailer: `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>` dan `Claude-Session: https://claude.ai/code/session_017NqdLCXr2x4X6KWjddprEM`.

## Review Focus

1. **B (diblokir) membuka percakapan lama dengan A dan mengirim pesan** → 404 umum, tanpa teks yang menyebut "blokir"; A tidak menerima apa pun. (Test di Task 1 dan Task 2.)
2. **A memblokir B saat keduanya online, lalu B mengetik** → A tidak menerima frame `typing`, dan B tidak menerima frame error. (Test di Task 2.)
3. **Akun yang memblokir/diblokir dihapus** → baris `blocks` ikut hilang; akun baru dengan username sama tidak mewarisi blokir. (Test di Task 1.)
4. **Token Turnstile valid tetapi slot penuh / rate limit habis** → error slot/rate (bukan captcha); token gagal tidak memakan jatah rate limit maupun slot. (Test di Task 3.)
5. **Widget Turnstile gagal dimuat (diblokir ekstensi browser)** → tombol guest tetap nonaktif dengan pesan yang bisa dibaca, login Google tetap berfungsi. (Test di Task 6.)

---

## File Structure

```
migrations/0003_blocks.up.sql / .down.sql
internal/store/blocks.go                     Block, Unblock, ListBlocks, BlockRelated
internal/store/conversations.go              ConversationSummary.BlockedByMe
internal/store/search.go                     SearchUsers mengecualikan pihak terkait blokir
internal/chat/blocks.go                      Service.Block/Unblock/Blocks
internal/chat/service.go                     Send/OpenConversation/MarkRead/PeerOf aware blokir
internal/api/handlers_blocks.go              /api/blocks
internal/api/security.go                     SecurityHeaders
internal/api/config_handler.go               GET /api/config
internal/auth/turnstile.go                   Verifier
internal/auth/guest.go                       verifikasi sebelum rate limit
internal/config/config.go                    TurnstileSiteKey/SecretKey
internal/realtime/conn.go                    typing/read tidak diteruskan bila diblokir
web/src/lib/api.ts                           blocks + getConfig
web/src/components/BlockMenu.tsx, BlockedList.tsx, TurnstileWidget.tsx, LegalNote.tsx
web/src/pages/Terms.tsx, Privacy.tsx         halaman legal
web/e2e/public.spec.ts
```

---

### Task 1: Store + chat — blokir

**Files:**
- Create: `migrations/0003_blocks.up.sql`, `migrations/0003_blocks.down.sql`, `internal/store/blocks.go`, `internal/chat/blocks.go`
- Modify: `internal/store/conversations.go`, `internal/store/search.go`, `internal/chat/service.go`, `internal/chat/errors.go` (kode `bad_request` bila belum ada)
- Test: `internal/store/blocks_test.go`, `internal/chat/blocks_test.go`

**Interfaces:**
- Produces:
  - Store: `Block(ctx, blocker, blocked uuid.UUID) error` (ON CONFLICT DO NOTHING; target tidak ada → `ErrNotFound`); `Unblock(ctx, blocker, blocked uuid.UUID) error`; `type BlockedUser struct{ ID uuid.UUID; Username, DisplayName string; BlockedAt time.Time }` (JSON `id, username, display_name, blocked_at`); `ListBlocks(ctx, blocker uuid.UUID) ([]BlockedUser, error)` (urut `blocked_at DESC`); `BlockRelated(ctx, a, b uuid.UUID) (bool, error)`.
  - `SearchUsers(ctx, prefix, excludeID, limit)` mengecualikan pengguna terkait blokir dengan `excludeID`.
  - `ConversationSummary.BlockedByMe bool` (JSON `blocked_by_me`).
  - Chat: `(*Service).Block(ctx, u store.User, target uuid.UUID) error`, `Unblock(ctx, u, target) error`, `Blocks(ctx, u) ([]store.BlockedUser, error)`.
  - `Send` dan `OpenConversation`: terkait blokir → `not_found`.
  - **Signature berubah:** `MarkRead(ctx, u, convID, seq) (int64, uuid.UUID, bool /*notifyPeer*/, error)` dan `PeerOf(ctx, u, convID) (uuid.UUID, bool /*deliver*/, error)` — bool `false` bila terkait blokir (tanpa error).

- [ ] **Step 1: Tulis test** — store: `TestBlockIdempotentAndUnblock`, `TestBlockSelfRejectedByCheck`, `TestBlocksCascadeOnDelete` (Review Focus 3: hapus akun B via `DeleteAccount` → `ListBlocks(A)` kosong), `TestSearchExcludesBlockRelated` (dua arah), `TestSummaryBlockedByMe` (A memblokir B → ringkasan A `BlockedByMe=true`, ringkasan B `false`). Chat: `TestSendBlockedBothWays` (Review Focus 1: A→B dan B→A `not_found`, tidak ada baris pesan baru), `TestOpenConversationBlocked`, `TestUnblockRestoresSend`, `TestBlockSelfBadRequest`, `TestGuestCannotBlock`, `TestDeveloperBlocksGuest` (guest lalu `Send` → `not_found`), `TestPeerOfAndMarkReadSuppressed` (deliver/notify `false`, err nil; `last_read_seq` tetap tersimpan).
- [ ] **Step 2:** `go test -race ./internal/store/ ./internal/chat/` → FAIL.
- [ ] **Step 3:** Implementasi; perbarui pemanggil `MarkRead`/`PeerOf` di `internal/realtime/conn.go` agar tetap terkompilasi (sementara: kirim hanya bila bool true — Task 2 menambah test-nya).
- [ ] **Step 4:** `go test -race ./...` → PASS.
- [ ] **Step 5:** Commit `feat(chat): user blocking` + push.

### Task 2: API + realtime — blokir

**Files:**
- Create: `internal/api/handlers_blocks.go`
- Modify: `internal/api/router.go`, `internal/realtime/conn.go`
- Test: `internal/api/blocks_test.go`, `internal/realtime/hub_test.go`

**Interfaces:**
- Consumes: Task 1.
- Produces: `POST /api/blocks {user_id}` → 204; `DELETE /api/blocks/:user_id` → 204 (uuid tidak valid → 404); `GET /api/blocks` → `[]BlockedUser` (`[]` bila kosong). Realtime: `typing` dan `read` hanya diteruskan bila `deliver`/`notifyPeer` true; tidak ada frame error untuk keduanya saat terkait blokir; `send` terkait blokir → frame `error` `not_found` dengan `client_msg_id`.

- [ ] **Step 1: Tulis test** — `TestBlocksAPI` (POST 204, GET berisi B dengan `display_name`, POST lagi 204, DELETE 204, GET `[]`; `user_id` diri sendiri → 400; guest → 403), `TestConversationsHidesBlockedMe` (respons JSON B tidak mengandung `"blocked_by_me":true`). Realtime `TestTypingAndReadSuppressedWhenBlocked` (Review Focus 2: A memblokir B; B kirim `typing` dan `read` → A tidak menerima frame dalam 500 ms, B tidak menerima frame apa pun), `TestSendBlockedErrorFrame` (B `send` → frame `error` `code=not_found` dengan `client_msg_id` yang sama; A tidak menerima `message`).
- [ ] **Step 2:** FAIL. **Step 3:** Implementasi. **Step 4:** `go test -race ./...` PASS.
- [ ] **Step 5:** Commit `feat(api): block endpoints and realtime suppression` + push.

### Task 3: Turnstile

**Files:**
- Create: `internal/auth/turnstile.go`, `internal/api/config_handler.go`
- Modify: `internal/config/config.go`, `internal/auth/guest.go`, `internal/auth/session.go` atau `auth.New` (wiring), `internal/api/router.go`, `cmd/server/main.go`, `.env.example`
- Test: `internal/auth/turnstile_test.go`, `internal/config/config_test.go`, `internal/api/config_handler_test.go`

**Interfaces:**
- Produces:
  - `config.Config.TurnstileSiteKey`, `TurnstileSecretKey`; `Load` error bila tepat satu terisi.
  - `auth.NewTurnstile(secret string) *Turnstile` dengan field `VerifyURL string` (default URL siteverify), `Client *http.Client` (timeout 5 s); `(*Turnstile).Verify(ctx, token, remoteIP string) error` mengembalikan `*chat.Error` `captcha_failed` (400) atau `captcha_unavailable` (503). `nil` receiver atau secret kosong → selalu lolos (nonaktif).
  - `Auth.Guest` membaca JSON `{turnstile_token}` (body boleh kosong bila nonaktif), memverifikasi **sebelum** limiter guest dan `CreateGuest`.
  - `GET /api/config` → `{"turnstile_site_key": cfg.TurnstileSiteKey}`.
  - Kode error baru `captcha_failed`, `captcha_unavailable` terdaftar di `internal/chat/errors.go` dan di i18n web (Task 6 menambah teksnya; tambahkan key i18n di task ini agar test kesamaan i18n tetap lulus).

- [ ] **Step 1: Tulis test** — `TestLoadTurnstilePair` (hanya site → error; hanya secret → error; keduanya/kosong → ok). Dengan server `httptest`: `TestGuestTurnstileOK` (`success:true` → 201; server menerima `secret`, `response`, `remoteip`), `TestGuestTurnstileFailed` (`success:false` → 400 `captcha_failed`; slot tidak berubah; 6 kali gagal berturut lalu 1 token valid → 201, membuktikan gagal captcha tidak memakan rate limit — Review Focus 4), `TestGuestTurnstileUnavailable` (server 500 → 503; server lambat > 5 s via `VerifyURL` ke handler yang tidur dan `Client.Timeout` disetel 100 ms di test → 503), `TestGuestTurnstileDisabled` (tanpa key, tanpa body → 201), `TestGuestValidTokenSlotFull` (slot 300 → 403 `user_limit_reached`), `TestConfigEndpoint`.
- [ ] **Step 2:** FAIL. **Step 3:** Implementasi. **Step 4:** `go test -race ./...` PASS.
- [ ] **Step 5:** Commit `feat(auth): cloudflare turnstile for guest creation` + push.

### Task 4: Header keamanan

**Files:**
- Create: `internal/api/security.go`
- Modify: `internal/api/router.go` (pasang paling awal, sebelum `/healthz`), `internal/api/router.go` `Deps.SecureCookies bool` atau baca dari config yang sudah ada
- Test: `internal/api/security_test.go`

**Interfaces:**
- Produces: `api.SecurityHeaders(secure bool) gin.HandlerFunc`; CSP string sebagai konstanta `api.ContentSecurityPolicy` (persis spec §5).

- [ ] **Step 1: Tulis test** — `TestSecurityHeadersEverywhere` (GET `/healthz`, `/api/me` (401), `/` (static) → kelima header ada dengan nilai persis; CSP sama dengan konstanta), `TestHSTSOnlyWhenSecure`.
- [ ] **Step 2:** FAIL. **Step 3:** Implementasi. **Step 4:** `go test -race ./...` PASS.
- [ ] **Step 5:** Commit `feat(api): security headers` + push.

### Task 5: Web — UI blokir

**Files:**
- Create: `web/src/components/BlockMenu.tsx`, `web/src/components/BlockedList.tsx`
- Modify: `web/src/lib/api.ts`, `web/src/components/ChatHeader.tsx`, `web/src/components/ConversationList.tsx`, `web/src/components/Composer.tsx` (atau wadah composer di `Chat.tsx`), `web/src/hooks/useChat.ts`, `web/src/pages/Settings.tsx`, `web/src/i18n/{en,id}.json`
- Test: `web/src/lib/api.test.ts` (fungsi baru), `web/src/hooks/useChat.test.ts` (helper murni)

**Interfaces:**
- Consumes: Task 2 API.
- Produces: `api.ts`: `ConversationSummary.blocked_by_me: boolean`; `BlockedUser`; `blockUser(id)`, `unblockUser(id)`, `listBlocks()`. `useChat` mengekspor helper murni `setBlocked(state, convIdOrPeerId, blocked)` dan memperbarui ringkasan setelah blok/unblok tanpa reload. UI sesuai spec §3: menu header (bukan untuk guest) dengan dialog konfirmasi; label "Diblokir" di daftar + header; composer diganti teks + tombol "Buka blokir"; Settings bagian "Pengguna yang diblokir" (kosong → teks "Tidak ada").

- [ ] **Step 1: Tulis test** — `setBlocked` menandai percakapan yang benar dan tidak menyentuh yang lain; `api.blockUser` memanggil `POST /api/blocks` dengan body `{user_id}`.
- [ ] **Step 2:** `npx vitest run` → FAIL. **Step 3:** Implementasi. **Step 4:** `npx vitest run` PASS; `npm run build` sukses.
- [ ] **Step 5:** Commit `feat(web): block and unblock users` + push.

### Task 6: Web — Turnstile, halaman legal, persetujuan

**Files:**
- Create: `web/src/components/TurnstileWidget.tsx`, `web/src/components/LegalNote.tsx`, `web/src/pages/Terms.tsx`, `web/src/pages/Privacy.tsx`
- Modify: `web/src/App.tsx` (rute `/terms`, `/privacy`), `web/src/pages/Landing.tsx`, `web/src/pages/Info.tsx`, `web/src/lib/api.ts` (`getConfig`, `createGuest(token?)`), `web/src/i18n/{en,id}.json`
- Test: `web/src/components/TurnstileWidget.test.tsx` atau helper murni `web/src/lib/turnstile.test.ts`

**Interfaces:**
- Consumes: Task 3 (`/api/config`, body guest).
- Produces: `TurnstileWidget({siteKey, onToken(token|null)})` memuat `https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit` sekali, merender eksplisit, memanggil `onToken(null)` pada `expired-callback`/`error-callback`, dan bila script gagal dimuat dalam 10 s → menampilkan teks `landing.captchaLoadFailed`. Landing: site key kosong → tanpa widget, tombol guest aktif; ada → tombol aktif hanya dengan token; error `captcha_failed` → reset widget + toast. `LegalNote`: teks persetujuan implisit dengan link `/terms` dan `/privacy`, tampil di bawah tombol login/guest. Halaman Terms/Privacy sesuai spec §6; Info menautkan keduanya dan isi privasi lama dipindah ke Privacy.

- [ ] **Step 1: Tulis test** — helper `loadTurnstileScript()` hanya menyisipkan satu `<script>` meski dipanggil dua kali; timeout pemuatan → promise reject (Review Focus 5); i18n parity tetap lulus.
- [ ] **Step 2:** FAIL. **Step 3:** Implementasi. **Step 4:** `npx vitest run` PASS; `npm run build` sukses.
- [ ] **Step 5:** Commit `feat(web): turnstile, terms and privacy pages` + push.

### Task 7: E2E, CI, dokumentasi

**Files:**
- Create: `web/e2e/public.spec.ts`
- Modify: `.github/workflows/ci.yml` (env e2e: `TURNSTILE_SITE_KEY=1x00000000000000000000AA`, `TURNSTILE_SECRET_KEY=1x0000000000000000000000000000000AA`), `web/e2e/chat.spec.ts`/`revamp.spec.ts` bila alur guest berubah, `README.md` (env Turnstile, cara membuat widget di dashboard Cloudflare untuk `chat.rahmattomy.com`, header keamanan, blokir), `.env.example`

**Interfaces:**
- Consumes: semua task.

- [ ] **Step 1: Tulis E2E** — (a) A dan B login palsu, saling chat; A memblokir B dari header → label "Diblokir" di A, composer A diganti; B mengirim → pesan tidak muncul di A, B mendapat toast error umum (tanpa kata "blok"/"block"); A tidak muncul di pencarian B; A membuka blokir dari Settings → B mengirim lagi dan A menerima. (b) Landing: link `/terms` dan `/privacy` membuka halaman dengan judul yang benar. (c) Guest dengan test key Turnstile: widget muncul, tombol guest aktif setelah token, guest masuk. (d) Kumpulkan `page.on('console')` + `securitypolicyviolation` di landing, chat, settings → nol pelanggaran CSP. (e) Header: `response.headers()['content-security-policy']` ada pada halaman utama.
- [ ] **Step 2:** Jalankan stack lokal (postgres container + `go run ./cmd/server` di :8099 dengan `E2E_FAKE_LOGIN=1 COOKIE_SECURE=false DEVELOPER_EMAIL=dev@e2e.test STATIC_DIR=web/dist` + dua env Turnstile test key) → `E2E_BASE_URL=http://localhost:8099 npx playwright test` → semua PASS. Catatan: test key Turnstile memerlukan akses ke `challenges.cloudflare.com`; bila sandbox memblokirnya, jalankan (c) hanya di CI dan laporkan.
- [ ] **Step 3:** `go test -race ./...`, `cd web && npx vitest run`, actionlint pada `ci.yml` → semua bersih.
- [ ] **Step 4:** Commit `test(e2e): blocking, legal pages, turnstile, csp` + push.
