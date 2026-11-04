# Frontend Revamp Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Merombak landing, onboarding, chat, dan info ke arah visual "Friendly bubbles"; menambah halaman Settings, display name, dan hapus akun.

**Architecture:** Backend: migrasi `0002` menambah `users.display_name`; `store` → `chat` (validasi, aturan) → `api` (REST) + `realtime` (event `conversation_deleted`). Frontend: token tema CSS (terang/gelap) dibaca Tailwind/shadcn; logika chat dipindah ke hook `useChat`; tampilan dipecah ke komponen kecil; fungsi murni di `web/src/lib` diuji Vitest.

**Tech Stack:** Go 1.24 + Gin + pgx + golang-migrate; React 18 + Vite + TS + Tailwind 3 + shadcn/ui + Sonner + react-i18next; Vitest; Playwright.

**Spec:** `docs/superpowers/specs/2026-11-02-frontend-revamp-design.md`

## Global Constraints

- Display name: 1–40 karakter (rune) setelah trim dan spasi ganda → satu; hanya `\p{L}`, `\p{M}`, `\p{N}`, spasi, `.` `-` `'` `_`; **tanpa emoji**; tidak unik; perubahan tanpa batas; error `display_name_invalid` (400).
- Guest display name: `Guest XXXX` (XXXX = 4 hex huruf besar, akhiran username guest); tidak bisa diubah (`forbidden`).
- Fallback display name akun Google: klaim `name` userinfo bila valid, selain itu `User`.
- Hapus akun: `DELETE /api/me` → 204 + cookie `pm_session` dihapus; developer → `forbidden` (403); CASCADE menghapus sesi/percakapan/pesan; `user_slots.used - 1` di transaksi yang sama; setelah commit kirim `{type:"conversation_deleted", conversation_id}` ke tiap lawan bicara online lalu `Disconnect(user)`.
- Email tidak pernah dikirim ke pengguna lain; display name dan isi pesan selalu dirender sebagai teks.
- Status pesan: `sending` (ikon jam), `sent` (satu centang), `read` (dua centang bertumpuk, biru muda `#bfe9ff` di gelembung oranye). Tidak ada status "sampai di perangkat".
- Palet B: aksen `#e2552f`; terang: latar `#fff7f1`, teks `#2a1d17`, muted `#7a5f53`; font Nunito (Google Fonts) dengan fallback `system-ui, sans-serif`. Gelap: latar hangat gelap (mis. `#1c1512`), teks `#f3e7e1`, aksen tetap terbaca (mis. `#ff7a52`).
- Tema: `localStorage` key `natter.theme` = `light|dark|system` (try/catch), default `system`, kelas `.dark` pada `<html>`.
- Breakpoint dua kolom chat: lebar ≥ 768px; di bawahnya satu panel dengan tombol kembali.
- Semua teks UI baru di `en.json` dan `id.json` (test kesamaan key wajib lulus).
- Semua test Go dengan `-race`. Kerja di branch `feat/revamp`; commit + push per task (`git push -u origin feat/revamp`). Trailer commit:
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>` dan `Claude-Session: https://claude.ai/code/session_017NqdLCXr2x4X6KWjddprEM`.
- Di luar scope: foto profil, status sampai-di-perangkat, opsi mematikan read/typing, pagination riwayat lama.

## Review Focus

1. **Nama Google berisi emoji atau terlalu panjang** (mis. "Budi 🚀" atau 60 huruf) → login tetap berhasil dengan display name `User`, tidak 500. (Test di Task 2.)
2. **Display name hanya spasi atau setelah dirapikan kosong** (`"   "`) → `display_name_invalid`. (Test di Task 2.)
3. **Hapus akun ketika lawan bicara sedang membuka percakapan itu** → lawan bicara kembali ke daftar dengan toast, tidak ada error di console, dan kiriman berikutnya ke percakapan itu tidak terjadi. (Test di Task 6 dan E2E Task 9.)
4. **Hapus akun dipanggil dua kali / sesi lama dipakai lagi** → panggilan kedua 401, slot tidak berkurang dua kali. (Test di Task 3.)
5. **Display name dengan HTML** (`<img src=x onerror=alert(1)>` ditolak validasi karena `<` `>`; tetapi nama lama/aneh yang lolos tetap dirender sebagai teks) → tidak ada elemen HTML terbentuk. (Test di Task 2 untuk penolakan; E2E Task 9 untuk render pesan berisi HTML tetap teks.)

---

## File Structure

```
migrations/0002_display_name.up.sql / .down.sql
internal/store/users.go          User.DisplayName, scanUser, SetDisplayName, DeleteAccount
internal/store/search.go         UserBrief.DisplayName
internal/store/conversations.go  ConversationSummary.PeerDisplayName
internal/chat/displayname.go     ValidateDisplayName
internal/chat/service.go         LoginGoogle(name), SetDisplayName, DeleteAccount
internal/auth/google.go          fetchIdentity returns name
internal/api/handlers.go         me, setDisplayName, deleteMe, search, conversations
internal/realtime/hub.go         NotifyConversationDeleted
web/src/index.css                theme tokens
web/src/lib/theme.ts             theme storage + apply
web/src/lib/format.ts            statusOf, dayLabel, avatarColor, validateDisplayName
web/src/hooks/useChat.ts         chat state/socket logic (moved from Chat.tsx)
web/src/components/              Avatar, StatusTicks, MessageBubble, MessageList, ChatHeader,
                                 ConversationList, Composer, UserSearch, GuestContactDialog (restyled)
web/src/pages/                   Landing, Onboarding, Chat, Settings (new), Info
web/e2e/revamp.spec.ts
```

---

### Task 1: Store — display name dan hapus akun

**Files:**
- Create: `migrations/0002_display_name.up.sql`, `migrations/0002_display_name.down.sql`
- Modify: `internal/store/users.go`, `internal/store/search.go`, `internal/store/conversations.go`
- Test: `internal/store/displayname_test.go`

**Interfaces:**
- Produces:
  - `User.DisplayName string` (diisi oleh semua query user).
  - `UpsertGoogleUser(ctx, sub, email, displayName string, isDeveloper bool) (User, bool, error)` — `displayName` hanya dipakai saat INSERT (user lama tidak ditimpa).
  - `CreateGuest` mengisi `display_name = 'Guest ' || upper(<4 hex>)`.
  - `(*Store).SetDisplayName(ctx, userID uuid.UUID, name string) (User, error)` — tanpa validasi (pemanggil memvalidasi); `ErrNotFound` bila tidak ada.
  - `(*Store).DeleteAccount(ctx, userID uuid.UUID) (peers []struct{ConversationID, PeerID uuid.UUID}, err error)` — satu tx: ambil pasangan percakapan, `DELETE FROM users WHERE id=$1`, `used-1` hanya bila terhapus; `ErrNotFound` bila tidak ada. Definisikan tipe bernama `DeletedConversation{ConversationID, PeerID uuid.UUID}` dan kembalikan `[]DeletedConversation`.
  - `UserBrief{ID, Username, DisplayName}` (JSON `id, username, display_name`), `ConversationSummary.PeerDisplayName` (JSON `peer_display_name`).

- [ ] **Step 1: Tulis test** — `TestMigration0002Constraints` (setelah `Migrate`: insert user via SQL tanpa `display_name` gagal NOT NULL; `display_name` 41 karakter gagal CHECK; `CreateGuest` menghasilkan `Guest XXXX` yang cocok dengan akhiran username), `TestSetDisplayName` (set "Rahmat Tomy" → `GetUser` mengembalikannya), `TestDeleteAccount` (A–B dan A–C berpercakapan dengan pesan; `DeleteAccount(A)` → 2 `DeletedConversation` dengan peer B dan C, percakapan dan pesan hilang, sesi A hilang, `used` turun 1; panggilan kedua → `ErrNotFound`, `used` tidak turun lagi), `TestSearchAndSummaryIncludeDisplayName`.
  Backfill: migrasi menulis `UPDATE users SET display_name = CASE WHEN kind='guest' THEN 'Guest ' || upper(substr(username, 7)) ELSE coalesce(username, 'User') END` sebelum `SET NOT NULL`.
- [ ] **Step 2:** `go test -race ./internal/store/` → FAIL.
- [ ] **Step 3:** Implementasi. Perbarui semua pemanggil `UpsertGoogleUser` agar tetap terkompilasi (sementara kirim `""` → simpan `User`; Task 2 mengganti).
- [ ] **Step 4:** `go test -race ./...` → PASS.
- [ ] **Step 5:** Commit `feat(store): display name and account deletion` + push.

### Task 2: Chat service — validasi display name, aturan, hapus akun

**Files:**
- Create: `internal/chat/displayname.go`
- Modify: `internal/chat/service.go`, `internal/chat/errors.go`
- Test: `internal/chat/displayname_test.go`, `internal/chat/service_test.go`

**Interfaces:**
- Consumes: Task 1.
- Produces:
  - `chat.ValidateDisplayName(raw string) (string, error)` — kembalikan nama ternormalisasi atau `*Error{Code:"display_name_invalid", Status:400}`.
  - `(*Service).LoginGoogle(ctx, sub, email, name string) (store.User, error)` — `name` divalidasi; tidak valid → `"User"`.
  - `(*Service).SetDisplayName(ctx, u store.User, raw string) (store.User, error)` — guest → `forbidden`.
  - `(*Service).DeleteAccount(ctx, u store.User) ([]store.DeletedConversation, error)` — developer → `forbidden`.

- [ ] **Step 1: Tulis test** — `TestValidateDisplayName` tabel: `"Rahmat Tomy"`→OK, `"José"`→OK, `"  a   b "`→`"a b"`, `"O'Neil-Jr_2."`→OK, `"😀"`, `"a★b"`, `"<img>"`, `"   "`, 41×`"a"` → semua `display_name_invalid`; 40×`"é"` → OK. `TestLoginGoogleNameFallback` (Review Focus 1): name `"Budi 🚀"` → `DisplayName=="User"`; 60 huruf → `"User"`; `"Budi Santoso"` → tetap. `TestSetDisplayNameGuestForbidden`. `TestDeleteAccountDeveloperForbidden`, `TestDeleteAccountOK` (Google biasa → mengembalikan peers).
- [ ] **Step 2:** `go test -race ./internal/chat/` → FAIL.
- [ ] **Step 3:** Implementasi; regex `^[\p{L}\p{M}\p{N} .\-'_]+$` setelah normalisasi, panjang dihitung dengan `utf8.RuneCountInString`.
- [ ] **Step 4:** `go test -race ./...` → PASS.
- [ ] **Step 5:** Commit `feat(chat): display name rules and account deletion` + push.

### Task 3: Auth + REST API

**Files:**
- Modify: `internal/auth/google.go`, `internal/auth/fakelogin.go`, `internal/api/router.go`, `internal/api/handlers.go`
- Test: `internal/auth/auth_test.go`, `internal/api/api_test.go`

**Interfaces:**
- Consumes: Task 2.
- Produces:
  - `fetchIdentity` mengembalikan `(sub, email, name string, err error)`; klaim userinfo `name`.
  - Fake login menerima `display_name` opsional dan meneruskan sebagai `name`.
  - `api.Deps.Notifier interface{ NotifyConversationDeleted(peers []store.DeletedConversation, deletedUser uuid.UUID) }` (nil-safe).
  - `GET /me` → `{id, kind, username, display_name, is_developer, username_changes_left, email}` (email hanya untuk diri sendiri, seperti sekarang).
  - `PUT /api/me/display-name {display_name}` → 200 `/me`.
  - `DELETE /api/me` → 204, hapus cookie `pm_session` (MaxAge -1), panggil `Notifier` setelah sukses.
  - Search → `[{id, username, display_name}]`; conversations → tambah `peer_display_name`.

- [ ] **Step 1: Tulis test** — `TestCallbackStoresGoogleName` (userinfo `name:"Budi Santoso"` → `/me.display_name`), `TestSetDisplayNameAPI` (200; `"😀"` → 400 `display_name_invalid`), `TestDeleteMeAPI` (204, cookie dihapus, request berikutnya dengan token sama → 401, `Notifier` dipanggil dengan peers yang benar), `TestDeleteMeTwice` (Review Focus 4: panggilan kedua 401, `used` hanya turun sekali), `TestDeleteMeDeveloper` (403), `TestSearchIncludesDisplayNameNoEmail`.
- [ ] **Step 2:** `go test -race ./internal/auth/ ./internal/api/` → FAIL.
- [ ] **Step 3:** Implementasi.
- [ ] **Step 4:** `go test -race ./...` → PASS.
- [ ] **Step 5:** Commit `feat(api): display name endpoints and account deletion` + push.

### Task 4: Realtime — event `conversation_deleted` + wiring

**Files:**
- Modify: `internal/realtime/protocol.go`, `internal/realtime/hub.go`, `cmd/server/main.go`
- Test: `internal/realtime/hub_test.go`

**Interfaces:**
- Consumes: Task 3 (`api.Deps.Notifier`).
- Produces: `(*Hub).NotifyConversationDeleted(peers []store.DeletedConversation, deletedUser uuid.UUID)` — kirim frame `{type:"conversation_deleted", conversation_id}` ke tiap `PeerID` yang online (non-blocking, seperti `deliver`), lalu `Disconnect(deletedUser)`. `main.go` memasang hub sebagai `Notifier`.

- [ ] **Step 1:** `TestNotifyConversationDeleted` — B online menerima frame dengan `conversation_id` benar dalam < 1 s; koneksi A ditutup; C offline tidak menyebabkan error.
- [ ] **Step 2:** FAIL. **Step 3:** Implementasi. **Step 4:** `go test -race -count=3 ./internal/realtime/` lalu `go test -race ./...` → PASS; `go build ./...`.
- [ ] **Step 5:** Commit `feat(realtime): conversation_deleted event` + push.

### Task 5: Web — tema, font, API client, fungsi murni

**Files:**
- Create: `web/src/lib/theme.ts`, `web/src/lib/format.ts`, `web/src/lib/format.test.ts`, `web/src/lib/theme.test.ts`
- Modify: `web/index.html` (link Nunito), `web/src/index.css`, `web/tailwind.config.*`, `web/src/lib/api.ts`, `web/src/main.tsx`, `web/src/i18n/{en,id}.json`

**Interfaces:**
- Consumes: REST Task 3.
- Produces:
  - `theme.ts`: `type ThemePref = "light"|"dark"|"system"`; `getThemePref(): ThemePref`; `setThemePref(p: ThemePref): void`; `applyTheme(p: ThemePref): void` (toggle `.dark` di `document.documentElement`, ikut perubahan `prefers-color-scheme` saat `system`). Dipanggil di `main.tsx` sebelum render.
  - `format.ts`: `statusOf(msg: Message, myId: string, peerLastReadSeq: number, pendingIds: Set<string>): "sending"|"sent"|"read"|null` (null untuk pesan orang lain); `dayLabel(d: Date, now: Date, lang: "en"|"id"): string` ("Today"/"Hari ini", "Yesterday"/"Kemarin", selain itu `Intl.DateTimeFormat(lang, {day:"numeric", month:"long", year:"numeric"})`); `avatarColor(id: string): string` (hash → salah satu 8 warna tetap); `validateDisplayName(s: string): {ok:true; value:string} | {ok:false}` (aturan sama dengan Global Constraints).
  - `api.ts`: `Me.display_name`, `UserBrief.display_name`, `ConversationSummary.peer_display_name`; `setDisplayName(name): Promise<Me>`; `deleteAccount(): Promise<void>`.
  - Token CSS shadcn (`--background`, `--foreground`, `--primary`, `--muted`, `--card`, `--border`, dst.) diisi palet B untuk `:root` dan `.dark`.

- [ ] **Step 1: Tulis test** — `format.test.ts`: `statusOf` (pending → `sending`; seq 5, peer read 4 → `sent`; peer read 5 → `read`; pesan orang lain → `null`); `dayLabel` (hari sama → "Hari ini"/"Today", kemarin, tanggal lain dalam id → mengandung nama bulan Indonesia); `avatarColor` deterministik dan selalu dari 8 warna; `validateDisplayName` (kasus sama dengan `TestValidateDisplayName` Task 2). `theme.test.ts`: `setThemePref("dark")` lalu `getThemePref()` → `"dark"`; storage yang melempar error → `getThemePref()` = `"system"`.
- [ ] **Step 2:** `cd web && npx vitest run` → FAIL.
- [ ] **Step 3:** Implementasi.
- [ ] **Step 4:** `npx vitest run` PASS; `npm run build` sukses.
- [ ] **Step 5:** Commit `feat(web): theme tokens, api fields and format helpers` + push.

### Task 6: Web — hook `useChat` dan komponen chat

**Files:**
- Create: `web/src/hooks/useChat.ts`, `web/src/components/{Avatar,StatusTicks,MessageBubble,MessageList,ChatHeader}.tsx`
- Modify: `web/src/pages/Chat.tsx`, `web/src/components/{ConversationList,Composer,UserSearch,GuestContactDialog}.tsx`
- Delete: `web/src/components/MessageView.tsx`
- Test: `web/src/hooks/useChat.test.ts` (fungsi reducer murni yang diekspor dari hook)

**Interfaces:**
- Consumes: Task 5 (`statusOf`, `dayLabel`, `avatarColor`, api), socket/outbox/paging yang ada.
- Produces:
  - `useChat(): { me, convs, activeId, setActiveId, messages(convId), pendingIds, typing(convId), online, bootError, retryBoot, send(convId, body), openPeer(peerId), markRead }` — semua perilaku MVP dipertahankan (catch-up berhalaman `fetchMissing`, retry `rate_limited` dengan backoff 10/20/40 s, layar retry saat boot gagal, toast pesan baru, read hanya saat terlihat). Tambahan: frame `conversation_deleted` → `removeConversation(state, convId)` (ekspor murni) membuang dari daftar dan cache; bila aktif → `activeId=null` + toast `chat.conversationDeleted`.
  - `StatusTicks({status})`: SVG — jam (`sending`), satu centang (`sent`), dua centang bertumpuk warna `#bfe9ff` (`read`); `aria-label` dari i18n.
  - `Avatar({id, name})`: lingkaran huruf awal, warna `avatarColor(id)`.
  - `MessageList`: pemisah tanggal via `dayLabel`, gelembung milik sendiri di kanan (oranye), jam `HH:mm`, `whitespace-pre-wrap`, teks React (tanpa `dangerouslySetInnerHTML`).
  - Layout ≥ 768px dua kolom; < 768px satu panel dengan tombol kembali; banner "Menyambung ulang…" saat `!online`; header menampilkan display name + "sedang mengetik…"; pencarian menampilkan display name + `@username`; ikon Settings di header daftar → `/settings`.

- [ ] **Step 1: Tulis test** — `useChat.test.ts`: `removeConversation` membuang percakapan dan pesannya serta mengosongkan `activeId` bila sama (Review Focus 3); konversi yang lain tidak tersentuh.
- [ ] **Step 2:** `npx vitest run` → FAIL.
- [ ] **Step 3:** Implementasi; pindahkan logika dari `Chat.tsx` tanpa mengubah perilaku.
- [ ] **Step 4:** `npx vitest run` PASS (termasuk test lama socket/outbox/paging); `npm run build` sukses.
- [ ] **Step 5:** Commit `feat(web): chat redesign with useChat hook` + push.

### Task 7: Web — Landing, Onboarding, Info

**Files:**
- Modify: `web/src/pages/{Landing,Onboarding,Info}.tsx`, `web/src/i18n/{en,id}.json`

**Interfaces:**
- Consumes: Task 5.
- Produces:
  - Landing: judul + ilustrasi dua gelembung chat, tombol "Masuk dengan Google" (link `/api/auth/google/login`) dan "Chat sebagai guest", link Info, toggle bahasa dan tema; `?error=` ditampilkan sebagai kartu terjemahan lalu dihapus dari URL (`history.replaceState`).
  - Onboarding: field display name (prefill `me.display_name` bila bukan `User`) dan username, validasi langsung (`validateDisplayName` + regex username), simpan via `setDisplayName` lalu `setUsername`; tampilkan error server per field.
  - Info: gaya baru, isi tetap.

- [ ] **Step 1:** Implementasi.
- [ ] **Step 2:** `npx vitest run` PASS (kesamaan key i18n); `npm run build` sukses.
- [ ] **Step 3:** Commit `feat(web): landing, onboarding and info redesign` + push.

### Task 8: Web — halaman Settings

**Files:**
- Create: `web/src/pages/Settings.tsx`, `web/src/components/DeleteAccountDialog.tsx`
- Modify: `web/src/App.tsx` (rute `/settings`), `web/src/i18n/{en,id}.json`

**Interfaces:**
- Consumes: Task 5 (`setDisplayName`, `deleteAccount`, `setUsername`, theme), Task 6 (tautan dari chat).
- Produces: `/settings` — kartu Profil (display name editable; username editable dengan teks "sisa N perubahan", nonaktif bila 0), Bahasa (EN/ID), Tema (Terang/Gelap/Ikut sistem, segmented control), Logout, Zona berbahaya ("Hapus akun" → `DeleteAccountDialog`: tombol hapus aktif hanya bila input sama persis dengan username; sukses → `/`). Guest: hanya Bahasa, Tema, dan "Akhiri sesi" (memanggil `deleteAccount`). Developer: tombol hapus tersembunyi dengan keterangan bahwa akun developer tidak bisa dihapus. Tombol kembali ke `/chat`.

- [ ] **Step 1:** Implementasi.
- [ ] **Step 2:** `npx vitest run` PASS; `npm run build` sukses.
- [ ] **Step 3:** Commit `feat(web): settings page` + push.

### Task 9: E2E dan screenshot

**Files:**
- Create: `web/e2e/revamp.spec.ts`, `docs/screenshots/` (PNG terang/gelap, desktop 1280×800 dan HP 390×844, untuk landing, chat, settings)
- Modify: `web/e2e/chat.spec.ts` bila selector lama berubah

**Interfaces:**
- Consumes: semua task; fake login `POST /api/test/login {sub,email,username?,display_name?}`.

- [ ] **Step 1: Tulis E2E** — (a) user baru lewat fake login tanpa username → onboarding, isi display name "Budi Santoso" + username unik → chat; (b) A cari B lewat awalan username → hasil menampilkan display name dan `@username`; (c) A kirim `<b>hi</b>` → B melihat teks literal, header A di B menampilkan display name; A melihat satu centang lalu dua centang bertumpuk (`aria-label` dibaca) setelah B membuka; (d) Settings: ganti tema ke Gelap → `<html>` punya kelas `dark` dan bertahan setelah reload; ganti bahasa; ubah display name → tampil di sisi lawan setelah reload; (e) A hapus akun saat B membuka percakapan A–B → A kembali ke landing, B kembali ke daftar dengan toast dan percakapan hilang (Review Focus 3); (f) guest: dialog kontak muncul, Settings hanya Bahasa/Tema/Akhiri sesi.
- [ ] **Step 2:** `E2E_FAKE_LOGIN=1 COOKIE_SECURE=false DEVELOPER_EMAIL=dev@e2e.test POSTGRES_PASSWORD=test docker compose up --build -d` (dengan `docker-compose.dev.yml` bila perlu CA), lalu `cd web && E2E_BASE_URL=http://localhost:8080 npx playwright test` → semua PASS.
- [ ] **Step 3:** Ambil screenshot dengan Playwright ke `docs/screenshots/`; `go test -race ./...` → PASS; `docker compose down -v`.
- [ ] **Step 4:** Commit `test(e2e): revamp flows and screenshots` + push.
