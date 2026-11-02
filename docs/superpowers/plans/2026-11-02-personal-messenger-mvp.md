# Personal Messenger MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Web chat 1:1 real-time (Google + guest → developer) dengan backend Go yang bisa dipakai klien lain.

**Architecture:** Satu proses Go + Gin. `store` (pgx, SQL manual) di bawah `chat` (aturan bisnis); `api` (REST) dan `realtime` (hub WebSocket) memanggil `chat`. `jobs` menjalankan pembersih guest dan retensi. React SPA di `web/` di-build dan disajikan oleh server yang sama.

**Tech Stack:** Go 1.23, Gin, `github.com/coder/websocket`, `github.com/jackc/pgx/v5`, `golang-migrate`, `golang.org/x/oauth2`, `testcontainers-go` (postgres module), PostgreSQL 16; React 18 + Vite + TypeScript + Tailwind + shadcn/ui + Sonner + react-i18next; Vitest; Playwright.

**Spec:** `docs/superpowers/specs/2026-11-02-personal-messenger-design.md`

## Global Constraints

- Batas pengguna: `user_slots.used` ≤ **300**; error `user_limit_reached`.
- Guest dihapus **30 menit** setelah `last_activity_at` percakapannya (atau `created_at` bila belum ada pesan); pembersih tiap **1 menit**; retensi percakapan **1 bulan**, harian pukul **03:00** waktu server.
- Pesan: `body` 1–**2000** karakter (dihitung rune, setelah trim spasi tidak boleh kosong); frame WS maks **8 KB**.
- Rate limit: **20 pesan / 10 detik** per user dan per IP; guest **5 / jam / IP**; search **30 / menit** per user.
- Username: `^[a-z0-9_]{3,20}$`, unik case-insensitive, maks **3** perubahan setelah pilihan pertama; search awalan min **2** karakter, maks **10** hasil.
- WS: ping tiap **25 s**, tutup bila tanpa pong **60 s**; buffer keluar **64**; satu koneksi per user. Klien: typing ≤ 1 / **3 s**, hilang setelah **5 s**; backoff 1, 2, 4 … maks **30 s**.
- Sesi: token 32 byte acak, DB simpan SHA-256; cookie `pm_session` `HttpOnly; Secure(COOKIE_SECURE); SameSite=Lax`; Google 30 hari; juga terima `Authorization: Bearer`.
- Bukan anggota percakapan → **404**. Format error `{"error":{"code","message"}}`.
- Email tidak pernah dikirim ke pengguna lain. Isi pesan tidak pernah dirender sebagai HTML.
- Env: `DATABASE_URL, GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, OAUTH_REDIRECT_URL, DEVELOPER_EMAIL, ALLOWED_ORIGINS, COOKIE_SECURE, LISTEN_ADDR(default :8080), TRUSTED_PROXY(default 127.0.0.1)`.
- Semua test Go dijalankan dengan `-race`. Commit + push setelah setiap task (`git push origin main`).
- Di luar scope (PRD §5) tidak dibangun. CI→prod (kriteria 18) tidak termasuk plan ini.

## Review Focus

1. **Guest masuk sebelum developer pernah login** → developer belum punya baris `users`; `POST /auth/guest` harus menolak dengan `developer_unavailable` tanpa memakan slot. (Test di Task 6.)
2. **Akun Google yang sudah terdaftar login lagi saat slot penuh** → harus berhasil dan tidak menambah `used`. (Test di Task 2.)
3. **Pesan emoji/multibyte dan spasi saja** → 2000 emoji diterima, 2001 ditolak; `"   "` ditolak `message_empty`. (Test di Task 6.)
4. **Developer membalas guest yang baru saja dihapus pembersih** → `send` mengembalikan `not_found` (bukan 500), koneksi WS guest ditutup. (Test di Task 6 dan Task 10.)
5. **Username beda huruf besar-kecil** (`Rahmat` vs `rahmat`) → dinormalisasi ke huruf kecil, yang kedua `username_taken`. (Test di Task 5.)

---

## File Structure

```
go.mod
cmd/server/main.go               wiring, graceful shutdown
cmd/loadtest/main.go             50 klien WS, cetak p50/p95/max
internal/config/config.go        load env
internal/store/
  store.go                       Store{pool}, New, WithTx, error sentinel
  migrate.go                     embed + jalankan migrations
  users.go  sessions.go  conversations.go  messages.go  search.go  cleanup.go
  testdb_test.go                 helper testcontainers (shared)
migrations/0001_init.up.sql / .down.sql
internal/chat/service.go         aturan bisnis; errors.go (kode error)
internal/ratelimit/limiter.go
internal/auth/                   session.go (middleware), google.go, guest.go
internal/api/                    router.go, handlers_*.go, errors.go
internal/realtime/               hub.go, conn.go, protocol.go
internal/jobs/jobs.go
web/                             Vite React app (src/lib, src/components, src/pages, src/i18n)
Dockerfile, docker-compose.yml, deploy/nginx.conf.example, README.md
```

---

### Task 1: Scaffold, config, skema DB, helper test

**Files:**
- Create: `go.mod`, `internal/config/config.go`, `migrations/0001_init.up.sql`, `migrations/0001_init.down.sql`, `internal/store/store.go`, `internal/store/migrate.go`, `internal/store/testdb_test.go`, `docker-compose.yml` (service `postgres` saja dulu), `.gitignore`
- Test: `internal/store/migrate_test.go`, `internal/config/config_test.go`

**Interfaces:**
- Produces:
  - `config.Load() (config.Config, error)` — field: `DatabaseURL, GoogleClientID, GoogleClientSecret, OAuthRedirectURL, DeveloperEmail, AllowedOrigins []string, CookieSecure bool, ListenAddr, TrustedProxy string`. Error bila `DATABASE_URL` atau `DEVELOPER_EMAIL` kosong.
  - `store.New(ctx, url string) (*store.Store, error)`, `(*Store).Close()`, `(*Store).Migrate(ctx) error`
  - `(*Store).WithTx(ctx, fn func(pgx.Tx) error) error`
  - Sentinel: `store.ErrNotFound, store.ErrSlotsFull, store.ErrUsernameTaken, store.ErrUsernameChangeLimit, store.ErrDuplicate`
  - Test helper `newTestStore(t *testing.T) *Store` (container Postgres 16 dibagi per paket, DB baru per test via `CREATE DATABASE`).

- [ ] **Step 1: Tulis test**
  - `TestLoadRequiresDeveloperEmail`: env tanpa `DEVELOPER_EMAIL` → error.
  - `TestMigrateCreatesSchema`: setelah `Migrate`, `SELECT used FROM user_slots WHERE id=1` = 0; `UPDATE user_slots SET used=301` → error check violation; insert message body 2001 karakter → error.
- [ ] **Step 2:** `go test -race ./internal/...` → FAIL (paket belum ada).
- [ ] **Step 3: Implementasi.** Skema persis spec §4, ditambah: `INSERT INTO user_slots VALUES (1,0)`; `conversations.created_at`; index `conversations(last_activity_at)`; `users(kind)`. Migrasi di-embed (`embed.FS` + `golang-migrate` source `iofs`). `docker-compose.yml`: `postgres:16`, volume `pgdata`, port `5432`.
- [ ] **Step 4:** `go test -race ./internal/...` → PASS.
- [ ] **Step 5:** Commit `feat: scaffold module, config and initial schema` + push.

### Task 2: Store — users dan slot

**Files:**
- Create: `internal/store/users.go`
- Test: `internal/store/users_test.go`

**Interfaces:**
- Consumes: Task 1.
- Produces:
  - `type User struct{ ID uuid.UUID; Kind string; GoogleSub *string; Email *string; Username *string; UsernameChanges int; IsDeveloper bool; CreatedAt time.Time }`
  - `(*Store).UpsertGoogleUser(ctx, sub, email string, isDeveloper bool) (User, bool /*created*/, error)` — user lama: update email/is_developer, tanpa slot; user baru: ambil slot dalam tx (`ErrSlotsFull`).
  - `(*Store).CreateGuest(ctx, developerID uuid.UUID) (User, uuid.UUID /*conversationID*/, error)` — tx: slot +1, insert guest `guest-` + 4 hex acak, buat percakapan + 2 member dengan developer.
  - `(*Store).DeleteGuest(ctx, id uuid.UUID) error` — tx: `DELETE ... WHERE kind='guest'`, bila terhapus slot −1.
  - `(*Store).GetUser(ctx, id) (User, error)`, `(*Store).GetDeveloper(ctx) (User, error)` (`ErrNotFound` bila belum ada).

- [ ] **Step 1: Tulis test**
  - `TestSlotRace`: set `used=299`, dua goroutine `UpsertGoogleUser` sub berbeda serentak → tepat 1 sukses, 1 `ErrSlotsFull`, `used=300`.
  - `TestExistingUserLoginWhenFull` (Review Focus 2): buat user, set `used=300`, upsert sub yang sama → sukses, `created=false`, `used` tetap 300.
  - `TestDeleteGuestFreesSlot`: buat guest → `used=1`; hapus → `used=0`, percakapan dan pesannya hilang; `DeleteGuest` pada user Google tidak menghapus apa pun.
- [ ] **Step 2:** `go test -race -run 'TestSlot|TestExisting|TestDeleteGuest' ./internal/store/` → FAIL.
- [ ] **Step 3:** Implementasi seperti Interfaces. Pemetaan check violation slot (`pgconn.PgError.Code == "23514"`) → `ErrSlotsFull`.
- [ ] **Step 4:** Test yang sama → PASS.
- [ ] **Step 5:** Commit `feat(store): users and race-safe slot counter` + push.

### Task 3: Store — sesi

**Files:**
- Create: `internal/store/sessions.go`
- Test: `internal/store/sessions_test.go`

**Interfaces:**
- Produces:
  - `(*Store).CreateSession(ctx, userID uuid.UUID, ttl time.Duration) (token string, error)` — 32 byte `crypto/rand`, base64url; simpan SHA-256.
  - `(*Store).UserBySession(ctx, token string) (User, error)` — `ErrNotFound` bila tidak ada/kedaluwarsa.
  - `(*Store).DeleteSession(ctx, token string) error`

- [ ] **Step 1:** `TestSessionRoundTrip` (buat → ambil user yang sama; DB tidak berisi token mentah), `TestSessionExpired` (ttl negatif → `ErrNotFound`), `TestSessionGoneAfterGuestDeleted`.
- [ ] **Step 2:** Jalankan → FAIL. **Step 3:** Implementasi. **Step 4:** PASS.
- [ ] **Step 5:** Commit `feat(store): hashed sessions` + push.

### Task 4: Store — percakapan dan pesan (inti konkurensi)

**Files:**
- Create: `internal/store/conversations.go`, `internal/store/messages.go`
- Test: `internal/store/messages_test.go`

**Interfaces:**
- Produces:
  - `type Message struct{ ID, ConversationID, SenderID uuid.UUID; Seq int64; Body string; CreatedAt time.Time; ClientMsgID uuid.UUID }` (JSON tag snake_case; `ClientMsgID` dengan `json:"client_msg_id,omitempty"`)
  - `(*Store).GetOrCreateConversation(ctx, a, b uuid.UUID) (uuid.UUID, error)` — urutkan a<b, `ON CONFLICT DO NOTHING` lalu select; insert 2 member.
  - `(*Store).IsMember(ctx, convID, userID uuid.UUID) (bool, error)`
  - `(*Store).Peer(ctx, convID, userID uuid.UUID) (uuid.UUID, error)`
  - `(*Store).SendMessage(ctx, convID, senderID, clientMsgID uuid.UUID, body string) (Message, bool /*duplicate*/, error)` — tx: cek dulu `(sender_id, client_msg_id)`; bila ada → kembalikan pesan lama, `duplicate=true`. Bila tidak: `UPDATE conversations SET last_seq=last_seq+1, last_activity_at=now() WHERE id=$1 RETURNING last_seq` (0 baris → `ErrNotFound`), insert. Bila insert kena unique `(sender_id, client_msg_id)` (race resend) → rollback, select pesan lama, `duplicate=true`.
  - `(*Store).ListMessages(ctx, convID uuid.UUID, afterSeq, beforeSeq int64, limit int) ([]Message, error)` — afterSeq>0: seq>after ASC; beforeSeq>0: seq<before, ambil terbaru lalu kembalikan ASC; limit dibatasi 1..100.
  - `(*Store).MarkRead(ctx, convID, userID uuid.UUID, seq int64) (int64, error)` — `last_read_seq = GREATEST(last_read_seq, LEAST($seq, last_seq))`, kembalikan nilai baru.
  - `type ConversationSummary struct{ ID uuid.UUID; PeerID uuid.UUID; PeerUsername string; PeerKind string; LastMessage *Message; Unread int64; PeerLastReadSeq int64; LastActivityAt time.Time }`
  - `(*Store).ListConversations(ctx, userID uuid.UUID) ([]ConversationSummary, error)` — urut `last_activity_at DESC`; `Unread = last_seq - my last_read_seq`.

- [ ] **Step 1: Tulis test**
  - `TestConcurrentSend` (kriteria 11): 50 user, 25 percakapan berpasangan + 1 percakapan bersama tempat 50 goroutine masing-masing mengirim 20 pesan serentak. Assert per percakapan: `count = expected`, seq = `1..N` tanpa celah, tidak ada duplikat `client_msg_id`.
  - `TestResend` (kriteria 12): `SendMessage` dua kali dengan `client_msg_id` sama (juga serentak dari 2 goroutine) → 1 baris, keduanya mengembalikan `ID` sama, satu `duplicate=true`; `ListMessages(after=N-1)` hanya mengembalikan pesan ke-N.
  - `TestGetOrCreateConversationConcurrent`: 10 goroutine untuk pasangan yang sama (urutan a/b acak) → satu ID.
  - `TestMarkReadMonotonic`: mark 5 lalu 3 → tetap 5; mark 999 → dipotong ke `last_seq`.
- [ ] **Step 2:** `go test -race -run 'TestConcurrentSend|TestResend|TestGetOrCreate|TestMarkRead' ./internal/store/` → FAIL.
- [ ] **Step 3:** Implementasi sesuai Interfaces.
- [ ] **Step 4:** Test yang sama → PASS; jalankan 3 kali (`-count=3`) untuk memastikan stabil.
- [ ] **Step 5:** Commit `feat(store): ordered idempotent messages` + push.

### Task 5: Store — username dan pencarian

**Files:**
- Create: `internal/store/search.go` (berisi juga `SetUsername`)
- Test: `internal/store/search_test.go`

**Interfaces:**
- Produces:
  - `(*Store).SetUsername(ctx, userID uuid.UUID, username string) (User, error)` — username sudah dinormalisasi oleh pemanggil. Bila `username` saat ini NULL: set tanpa menambah counter. Bila sama dengan yang sekarang: no-op. Selain itu `username_changes+1`; check violation → `ErrUsernameChangeLimit`; unique violation → `ErrUsernameTaken`.
  - `type UserBrief struct{ ID uuid.UUID; Username string }`
  - `(*Store).SearchUsers(ctx, prefix string, excludeID uuid.UUID, limit int) ([]UserBrief, error)` — `kind='google' AND username IS NOT NULL AND lower(username) LIKE prefix||'%'` (escape `%`/`_`), urut username, maks `limit`.

- [ ] **Step 1:** `TestUsernameChangeLimit` (pilih pertama + 3 perubahan sukses, ke-4 `ErrUsernameChangeLimit`), `TestUsernameTaken` (Review Focus 5: dua user serentak set `rahmat` → satu `ErrUsernameTaken`), `TestSearchPrefix` (`ra` menemukan `rahmat`, `rani_99`, tidak menemukan guest, diri sendiri, atau `bara`; `_` di prefix diperlakukan literal; maks 10).
- [ ] **Step 2:** FAIL. **Step 3:** Implementasi. **Step 4:** PASS.
- [ ] **Step 5:** Commit `feat(store): usernames and prefix search` + push.

### Task 6: Chat service — aturan bisnis

**Files:**
- Create: `internal/chat/service.go`, `internal/chat/errors.go`
- Test: `internal/chat/service_test.go` (pakai Postgres asli; salin helper `newTestStore` sebagai `internal/store/storetest/storetest.go` exported `storetest.New(t) *store.Store` — pindahkan helper Task 1 ke sana)

**Interfaces:**
- Consumes: Task 2–5.
- Produces:
  - `type Error struct{ Code string; Status int }` + kode: `user_limit_reached(403)`, `developer_unavailable(503)`, `not_found(404)`, `forbidden(403)`, `message_empty(400)`, `message_too_long(400)`, `username_invalid(400)`, `username_taken(409)`, `username_change_limit(409)`, `username_required(403)`, `query_too_short(400)`, `rate_limited(429)`, `unauthorized(401)`.
  - `chat.New(st *store.Store, developerEmail string) *Service`
  - `(*Service).LoginGoogle(ctx, sub, email string) (store.User, error)` — `isDeveloper = strings.EqualFold(email, developerEmail)`.
  - `(*Service).CreateGuest(ctx) (store.User, uuid.UUID, error)` — `GetDeveloper` gagal → `developer_unavailable` (sebelum menyentuh slot).
  - `(*Service).SetUsername(ctx, u store.User, raw string) (store.User, error)` — lowercase + trim, validasi regex; guest → `forbidden`.
  - `(*Service).Search(ctx, u store.User, q string) ([]store.UserBrief, error)` — guest → `forbidden`; `len<2` → `query_too_short`; limit 10.
  - `(*Service).OpenConversation(ctx, u store.User, peerID uuid.UUID) (uuid.UUID, error)` — guest → `forbidden`; peer harus Google dan punya username, bukan diri sendiri, kalau tidak `not_found`.
  - `(*Service).Send(ctx, u store.User, convID, clientMsgID uuid.UUID, body string) (store.Message, uuid.UUID /*peerID*/, bool /*dup*/, error)` — validasi body (trim kosong → `message_empty`; `utf8.RuneCountInString(body) > 2000` → `message_too_long`); bukan member → `not_found`.
  - `(*Service).Messages(ctx, u, convID, afterSeq, beforeSeq int64, limit int) ([]store.Message, error)`, `(*Service).MarkRead(ctx, u, convID, seq) (int64, uuid.UUID /*peerID*/, error)`, `(*Service).Conversations(ctx, u) ([]store.ConversationSummary, error)`, `(*Service).PeerOf(ctx, u, convID) (uuid.UUID, error)` (untuk typing).
  - Setiap method selain `LoginGoogle`/`CreateGuest`/`SetUsername`: user Google tanpa username → `username_required`.

- [ ] **Step 1: Tulis test**
  - `TestGuestBeforeDeveloper` (Review Focus 1): tanpa developer → `developer_unavailable`, `used` tetap 0.
  - `TestGuestOnlyDeveloper`: guest `OpenConversation` ke user lain → `forbidden`; `Search` → `forbidden`; `Send` ke percakapan dengan developer → OK.
  - `TestAuthzNotMember` (kriteria 16): user C `Messages` / `Send` / `MarkRead` pada percakapan A–B → `not_found`.
  - `TestBodyValidation` (Review Focus 3, kriteria 17): 2000 × "😀" OK, 2001 → `message_too_long`, `"  \n "` → `message_empty`, `<script>alert(1)</script>` tersimpan apa adanya.
  - `TestSendToDeletedGuest` (Review Focus 4): developer kirim ke percakapan guest yang sudah `DeleteGuest` → `not_found`.
  - `TestUsernameRequired`: user Google baru tanpa username → `Conversations` → `username_required`.
- [ ] **Step 2:** `go test -race ./internal/chat/` → FAIL.
- [ ] **Step 3:** Implementasi.
- [ ] **Step 4:** PASS.
- [ ] **Step 5:** Commit `feat(chat): business rules service` + push.

### Task 7: Rate limiter

**Files:**
- Create: `internal/ratelimit/limiter.go`
- Test: `internal/ratelimit/limiter_test.go`

**Interfaces:**
- Produces: `ratelimit.New(limit int, window time.Duration, now func() time.Time) *Limiter`; `(*Limiter).Allow(key string) bool` (token bucket, aman konkuren dengan mutex); `(*Limiter).Sweep()` menghapus bucket penuh (dipanggil tiap menit oleh `jobs`).

- [ ] **Step 1:** `TestAllowBurstThenRefill` (20/10s: 20 lolos, ke-21 ditolak, setelah jam palsu maju 10 s lolos lagi), `TestKeysIndependent`, `TestConcurrentAllow` (100 goroutine × 1 Allow, tepat 20 true).
- [ ] **Step 2:** FAIL. **Step 3:** Implementasi. **Step 4:** PASS.
- [ ] **Step 5:** Commit `feat: in-memory token bucket limiter` + push.

### Task 8: Auth — middleware sesi, Google OAuth, guest

**Files:**
- Create: `internal/auth/session.go`, `internal/auth/google.go`, `internal/auth/guest.go`
- Test: `internal/auth/auth_test.go`

**Interfaces:**
- Consumes: Task 3, 6, 7.
- Produces:
  - `auth.New(st *store.Store, svc *chat.Service, cfg config.Config, guestLimiter *ratelimit.Limiter) *Auth`
  - `(*Auth).Middleware() gin.HandlerFunc` — token dari cookie `pm_session` atau `Authorization: Bearer`; set `c.Set("user", store.User)`; gagal → 401 `unauthorized`. `auth.CurrentUser(c *gin.Context) store.User`.
  - Handler: `GoogleLogin` (redirect + cookie `pm_oauth_state` 10 menit), `GoogleCallback` (cek state, tukar code, ambil `sub`/`email` dari userinfo `https://openidconnect.googleapis.com/v1/userinfo`, `LoginGoogle`, buat sesi 30 hari, redirect `/`; `user_limit_reached` → redirect `/?error=user_limit_reached`), `Guest` (POST; limiter key `guest:`+IP; sesi TTL 24 jam — penghapusan sebenarnya oleh job), `Logout` (hapus sesi; guest → `DeleteGuest`).
  - `auth.ClientIP(c *gin.Context) string` — `X-Real-IP` hanya bila `RemoteAddr` = `TrustedProxy`, selain itu `RemoteAddr`.
  - Endpoint userinfo diinjeksi lewat field `Auth.UserInfoURL` agar bisa di-test.

- [ ] **Step 1:** `TestMiddlewareCookieAndBearer`, `TestMiddlewareRejectsMissing` (401 format error spec), `TestCallbackBadState` (400), `TestCallbackCreatesSession` (OAuth token + userinfo palsu via `httptest.Server`; email = `DEVELOPER_EMAIL` → `is_developer`), `TestGuestRateLimit` (IP sama ke-6 → 429), `TestGuestLogoutDeletesAccount`, `TestClientIPIgnoresSpoofedHeader`.
- [ ] **Step 2:** FAIL. **Step 3:** Implementasi. **Step 4:** PASS.
- [ ] **Step 5:** Commit `feat(auth): sessions, google oauth, guest` + push.

### Task 9: REST API

**Files:**
- Create: `internal/api/router.go`, `internal/api/errors.go`, `internal/api/handlers.go`
- Test: `internal/api/api_test.go`

**Interfaces:**
- Consumes: Task 6–8.
- Produces:
  - `api.NewRouter(deps api.Deps) *gin.Engine`, `type Deps struct{ Auth *auth.Auth; Chat *chat.Service; SearchLimiter *ratelimit.Limiter; WS gin.HandlerFunc; StaticDir string }`.
  - Rute persis spec §6 di bawah `/api`. `GET /me` → `{id, kind, username, is_developer, username_changes_left}` (email hanya untuk diri sendiri). `GET /users/search?q=` → `[{id, username}]`. `POST /conversations {peer_id}` → `{id}`. `GET /conversations` → array `ConversationSummary` (snake_case). `GET /conversations/:id/messages` → `{messages:[...]}`.
  - `api.WriteError(c, err)` memetakan `*chat.Error` → status + JSON; error lain → 500 `internal` (detail hanya di log).
  - `StaticDir` disajikan untuk path non-`/api`; fallback `index.html` (SPA).
  - CORS: hanya origin di `ALLOWED_ORIGINS`, `Allow-Credentials: true`.

- [ ] **Step 1:** `TestAPIAuthzReturns404` (kriteria 16 lewat HTTP), `TestSearchHidesEmail` (respons tidak mengandung `@`), `TestSearchRateLimit` (ke-31 → 429), `TestMessagesPagination` (`after_seq`, `before_seq`, limit 500 → 100), `TestErrorFormat`.
- [ ] **Step 2:** FAIL. **Step 3:** Implementasi. **Step 4:** PASS.
- [ ] **Step 5:** Commit `feat(api): REST endpoints` + push.

### Task 10: Realtime — hub WebSocket

**Files:**
- Create: `internal/realtime/protocol.go`, `internal/realtime/hub.go`, `internal/realtime/conn.go`
- Test: `internal/realtime/hub_test.go`

**Interfaces:**
- Consumes: Task 6, 7, 8.
- Produces:
  - Frame persis spec §5 (`send, ack, message, typing, read, error`); `type Frame struct{ Type string; ConversationID *uuid.UUID; ClientMsgID *uuid.UUID; Body string; Seq int64; UserID *uuid.UUID; Message *store.Message; Code, Message string }` dengan JSON snake_case (field kosong `omitempty`).
  - `realtime.NewHub(svc *chat.Service, msgLimiter *ratelimit.Limiter, allowedOrigins []string) *Hub`
  - `(*Hub).Handler() gin.HandlerFunc` — butuh `auth.Middleware` sebelumnya; `websocket.Accept` dengan `OriginPatterns`; `SetReadLimit(8192)`.
  - `(*Hub).Disconnect(userID uuid.UUID)` — dipakai `jobs` saat guest dihapus.
  - Perilaku: map `user_id→*conn` dengan `sync.RWMutex`; koneksi baru menutup lama; tiap conn goroutine baca + tulis, `out chan []byte` buffer 64; enqueue non-blocking, penuh → tutup conn; ping 25 s / timeout 60 s. `send`: limiter key `msg:u:`+userID dan `msg:ip:`+IP (gagal → `error rate_limited` dengan `client_msg_id`), `svc.Send`, `ack` ke pengirim, `message` ke peer **hanya bila bukan duplikat**. `typing`: `PeerOf` lalu teruskan. `read`: `MarkRead` lalu kirim `read` ke peer. Error service → frame `error` dengan `code`; koneksi tetap terbuka.

- [ ] **Step 1: Tulis test** (server `httptest` + `coder/websocket` client)
  - `TestDeliverOnlineUnder1s` (kriteria 2): A kirim, B menerima `message` < 1 s, A menerima `ack`.
  - `TestDuplicateNotRedelivered`: kirim ulang `client_msg_id` sama → A dapat `ack` ID sama, B hanya menerima 1 `message`.
  - `TestTypingAndRead` (kriteria 9).
  - `TestSecondConnectionReplacesFirst`.
  - `TestSlowConsumerDropped`: klien B tidak membaca; A kirim 200 pesan → koneksi B ditutup, semua 200 tetap ada di DB.
  - `TestOversizeFrameClosed` (> 8 KB) dan `TestRateLimitedFrame` (pesan ke-21 dalam 10 s → `error rate_limited`).
  - `TestDisconnectDeletedGuest` (Review Focus 4).
- [ ] **Step 2:** `go test -race ./internal/realtime/` → FAIL.
- [ ] **Step 3:** Implementasi.
- [ ] **Step 4:** PASS (`-count=3`).
- [ ] **Step 5:** Commit `feat(realtime): websocket hub` + push.

### Task 11: Jobs + main server

**Files:**
- Create: `internal/store/cleanup.go`, `internal/jobs/jobs.go`, `cmd/server/main.go`
- Test: `internal/store/cleanup_test.go`, `internal/jobs/jobs_test.go`

**Interfaces:**
- Produces:
  - `(*Store).ExpiredGuests(ctx, olderThan time.Time) ([]uuid.UUID, error)` — guest yang `COALESCE(conv.last_activity_at, user.created_at) < olderThan`.
  - `(*Store).DeleteInactiveConversations(ctx, olderThan time.Time) (int64, error)`
  - `jobs.Run(ctx, st *store.Store, hub interface{ Disconnect(uuid.UUID) }, limiters []*ratelimit.Limiter, now func() time.Time)` — ticker 1 menit: hapus guest > 30 menit (`DeleteGuest` + `Disconnect`), `Sweep` limiter; timer ke 03:00 berikutnya: hapus percakapan > 1 bulan (`now.AddDate(0,-1,0)`).
  - `jobs.NextDailyRun(now time.Time, hour int) time.Time`
  - `main`: `config.Load` → `store.New` → `Migrate` → rakit service/limiter/auth/hub/router → `jobs.Run` → `http.Server` di `ListenAddr`, shutdown halus pada SIGTERM.

- [ ] **Step 1:** `TestExpiredGuests` (guest 31 menit lalu tanpa pesan → kena; guest dengan pesan 10 menit lalu → tidak; developer membalas → jam guest ter-reset), `TestRetention` (kriteria 8: percakapan `last_activity_at` mundur 32 hari → terhapus beserta pesannya; 20 hari → tetap), `TestNextDailyRun` (02:59 → 03:00 hari sama; 03:00 → besok 03:00).
- [ ] **Step 2:** FAIL. **Step 3:** Implementasi. **Step 4:** PASS; `go build ./...` sukses.
- [ ] **Step 5:** Commit `feat: cleanup jobs and server entrypoint` + push.

### Task 12: Load test

**Files:**
- Create: `cmd/loadtest/main.go`

**Interfaces:**
- Consumes: server berjalan + endpoint `POST /api/auth/guest` tidak dipakai (rate limit); sebagai gantinya flag `-dsn` untuk membuat 50 user + sesi langsung lewat `store`, lalu konek WS dengan Bearer.
- Produces: CLI `go run ./cmd/loadtest -url ws://localhost:8080/api/ws -dsn $DATABASE_URL -clients 50 -messages 20` mencetak `sent, received, missing, duplicates, p50, p95, max`, exit code 1 bila `missing>0 || duplicates>0 || p95>=1s`. Batasi laju per klien di bawah rate limit (1 pesan / 600 ms).

- [ ] **Step 1:** Jalankan `docker compose up -d postgres` + server lokal, lalu loadtest → harapkan output `missing=0 duplicates=0` dan `p95 < 1s`.
- [ ] **Step 2:** Commit `feat: websocket load test tool` + push.

### Task 13: Frontend — fondasi, auth, onboarding, i18n

**Files:**
- Create: `web/` (Vite React TS, Tailwind, shadcn: button, input, dialog, scroll-area, sonner, dropdown-menu), `web/src/lib/api.ts`, `web/src/i18n/{index.ts,en.json,id.json}`, `web/src/pages/{Landing.tsx,Onboarding.tsx,Info.tsx}`, `web/src/App.tsx`, `web/src/components/LanguageToggle.tsx`
- Test: `web/src/i18n/i18n.test.ts`, `web/src/lib/api.test.ts` (Vitest)

**Interfaces:**
- Consumes: REST Task 9.
- Produces:
  - `api.ts`: `request<T>(method, path, body?) : Promise<T>` (`credentials:'include'`, lempar `ApiError{code,status}`); `getMe, setUsername, search, openConversation, listConversations, listMessages, createGuest, logout`.
  - `t('errors.<code>')` untuk setiap kode di Task 6.
  - Rute: `/` (Landing: tombol "Login with Google" → `/api/auth/google/login`, "Continue as guest"; tampilkan `?error=`), `/onboarding` (pilih username bila `me.username==null`), `/info` (teks privasi spec §8), `/chat` (Task 14). Pilihan bahasa disimpan di `localStorage` (try/catch), default dari `navigator.language`.

- [ ] **Step 1:** `i18n.test.ts`: `en.json` dan `id.json` punya kumpulan key identik dan semua kode error. `api.test.ts`: respons 403 `{error:{code:'user_limit_reached'}}` → `ApiError.code === 'user_limit_reached'`.
- [ ] **Step 2:** `cd web && npx vitest run` → FAIL.
- [ ] **Step 3:** Implementasi. Vite dev proxy `/api` → `http://localhost:8080` (termasuk `ws: true`).
- [ ] **Step 4:** `npx vitest run` PASS; `npm run build` sukses.
- [ ] **Step 5:** Commit `feat(web): scaffold, auth screens, i18n` + push.

### Task 14: Frontend — chat real-time

**Files:**
- Create: `web/src/lib/socket.ts`, `web/src/lib/outbox.ts`, `web/src/pages/Chat.tsx`, `web/src/components/{ConversationList.tsx,MessageView.tsx,Composer.tsx,UserSearch.tsx,GuestContactDialog.tsx}`
- Test: `web/src/lib/socket.test.ts`, `web/src/lib/outbox.test.ts`

**Interfaces:**
- Consumes: frame Task 10, REST Task 9, Task 13.
- Produces:
  - `socket.ts`: `class ChatSocket { connect(); send(frame); on(type, cb); onReconnect(cb) }` — backoff 1,2,4…30 s (fungsi murni `backoffDelay(attempt): number`), saat reconnect: kirim ulang outbox dan panggil `onReconnect`.
  - `outbox.ts`: `class Outbox { add(convId, body): ClientMsg; ack(clientMsgId, message); pending(): ClientMsg[] }` — `client_msg_id = crypto.randomUUID()`.
  - Chat: daftar percakapan (unread badge), pencarian username (debounce 300 ms, min 2 karakter), tampilan pesan terurut `seq` dan di-dedup per `id`, status per pesan sendiri: mengirim → ✓ (ack) → ✓✓ (peer `last_read_seq >= seq`); "typing…" 5 s; kirim `read` saat percakapan terbuka dan `document.visibilityState==='visible'`; toast Sonner untuk `message` dari percakapan yang tidak sedang dibuka; setelah reconnect: `listMessages(after_seq = seq terakhir)` + `listConversations`. Guest: langsung buka percakapan developer, `GuestContactDialog` sekali setelah `createGuest` (jawaban → pesan biasa, tombol Skip). Isi pesan dirender sebagai teks React biasa; `whitespace-pre-wrap`. Indikator karakter, tombol Send nonaktif > 2000. 401 dari API/WS → kembali ke `/`.

- [ ] **Step 1:** `backoffDelay` → `[1000,2000,4000,8000,16000,30000,30000]`; Outbox: setelah `ack` tidak lagi di `pending()`, `add` dua kali menghasilkan ID berbeda; merge pesan dengan `id` sama tidak menduplikasi (`mergeMessages(list, incoming)` di `outbox.ts`).
- [ ] **Step 2:** `npx vitest run` → FAIL.
- [ ] **Step 3:** Implementasi.
- [ ] **Step 4:** `npx vitest run` PASS; `npm run build` sukses.
- [ ] **Step 5:** Commit `feat(web): realtime chat UI` + push.

### Task 15: Docker, Nginx, E2E, README

**Files:**
- Create: `Dockerfile`, `deploy/nginx.conf.example`, `web/playwright.config.ts`, `web/e2e/chat.spec.ts`, `README.md`
- Modify: `docker-compose.yml` (tambah service `app`), `internal/auth/google.go` (hanya bila `E2E_FAKE_LOGIN=1`: `POST /api/test/login {sub,email}` membuat sesi — **tidak didaftarkan** bila env tidak di-set; test memastikan 404 tanpa env)

**Interfaces:**
- Consumes: semua task.
- Produces: `docker compose up --build` menyajikan aplikasi di `:8080`; `README.md` berisi setup Google OAuth, env, perintah verifikasi dari tabel spec §10, dan config Nginx.

- [ ] **Step 1:** Dockerfile multi-stage (`node:20` build web → `golang:1.23` build `CGO_ENABLED=0` → `gcr.io/distroless/static`), static dir `/app/web`.
- [ ] **Step 2:** `TestFakeLoginDisabledByDefault` (Go) → tulis, FAIL, implementasi, PASS.
- [ ] **Step 3:** E2E Playwright (dua browser context): A & B login palsu, set username, A cari `b`, kirim `<b>hi</b>` → B melihat teks literal tanpa refresh + toast; B membuka → A melihat ✓✓; typing tampil; toggle bahasa mengubah teks; guest masuk → dialog kontak muncul → isi → developer menerima pesan; guest tidak punya kotak pencarian.
- [ ] **Step 4:** `E2E_FAKE_LOGIN=1 DEVELOPER_EMAIL=dev@e2e.test docker compose up --build -d` lalu `cd web && E2E_BASE_URL=http://localhost:8080 npx playwright test` → PASS; `go test -race ./...` → PASS.
- [ ] **Step 5:** Commit `chore: docker, nginx example, e2e, readme` + push.
