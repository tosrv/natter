# Guest Name + Contact Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tamu wajib mengisi nama tampilan (default `natter-xxxx`) dan boleh mengisi kontak, dalam satu form, di web dan widget.

**Architecture:** `POST /api/auth/guest` menerima `display_name` wajib (divalidasi setelah Turnstile, sebelum rate limit/slot) dan menyimpannya saat akun tamu dibuat. Web dan widget menampilkan satu form (nama → kontak opsional → Turnstile → Mulai chat); kontak dikirim klien sebagai pesan pertama lewat outbox.

**Tech Stack:** Go/Gin/pgx; React + shadcn (web); Preact (widget); Vitest; Playwright.

**Spec:** `docs/superpowers/specs/2026-11-04-guest-name-contact-design.md`

## Global Constraints

- Body guest: `{"turnstile_token": string, "display_name": string}`; nama divalidasi `chat.ValidateDisplayName` (1–40 rune, `^[\p{L}\p{M}\p{N} .\-'_]+$`, bukan "User").
- Nama tidak valid → 400 `display_name_invalid`, tidak memakai token rate limit maupun slot akun.
- Username tamu tetap `guest-<suffix>` buatan server; kontak tidak diproses server.
- Nama acak: `natter-` + 4 karakter `[a-z0-9]` dari `crypto.getRandomValues`, pola `^natter-[a-z0-9]{4}$`, dibuat ulang setiap form dibuka.
- Tombol "Mulai chat" nonaktif bila nama (trim) kosong atau > 40 rune, atau kontak > batas panjang pesan (`MAX_BODY` web; nilai yang sama di widget), atau token Turnstile belum ada (saat Turnstile aktif).
- Teks UI (EN/ID): "Your name"/"Nama kamu"; "Contact (optional) — email, WhatsApp…"/"Kontak (opsional) — email, WhatsApp…"; "Start chat"/"Mulai chat"; "Use 1–40 letters, numbers, spaces or . - ' _"/"Pakai 1–40 huruf, angka, spasi, atau . - ' _".
- Widget tetap ≤ 40960 B gzip.
- Setiap task: commit + push ke `feat/guest-name-contact`.

## Review Focus

1. Kontak hanya spasi → tidak dikirim sebagai pesan (Task 2 & 3: test `whitespace contact is not sent`).
2. Nama dengan spasi berlebih (`"  Budi   S  "`) → tersimpan dinormalisasi `"Budi S"` (Task 1: `TestGuestDisplayNameNormalized`).
3. `display_name_invalid` setelah Turnstile terpakai → token Turnstile direset sehingga tamu bisa mencoba lagi (Task 2 & 3: test `invalid name resets turnstile`).
4. Kontak tidak hilang bila socket belum tersambung saat sesi dibuat → masuk outbox dan terkirim setelah tersambung (Task 3: `contact queued before socket open is sent after open`).
5. Body tanpa `display_name` saat Turnstile nonaktif (dev/E2E) → 400, bukan tamu tanpa nama (Task 1: `TestGuestMissingNameNoTurnstile`).

---

### Task 1: Backend — `display_name` wajib pada guest

**Files:**
- Modify: `internal/store/users.go` (`CreateGuest`), `internal/chat/service.go` (`CreateGuest`), `internal/auth/guest.go`
- Modify (call sites): semua `CreateGuest(` di `*_test.go` (store, chat, api, realtime, jobs) — beri nama tetap, mis. `"Guest Test"`
- Test: `internal/auth/auth_test.go`, `internal/chat/guests_test.go`, `internal/store/guests_test.go`

**Interfaces:**
- Produces: `func (s *Store) CreateGuest(ctx context.Context, developerID uuid.UUID, displayName string) (User, uuid.UUID, error)`; `func (s *Service) CreateGuest(ctx context.Context, displayName string) (store.User, uuid.UUID, error)` — memanggil `ValidateDisplayName` dan mengembalikan `ErrDisplayNameInvalid` bila gagal. Handler mengirim 400 `display_name_invalid` lewat `WriteError` yang ada.

- [ ] **Step 1: Test gagal:**
  - store: `CreateGuest(ctx, dev.ID, "Budi")` → `*g.DisplayName == "Budi"` (sesuaikan dengan tipe field yang ada), username berawalan `guest-`.
  - chat: `CreateGuest(ctx, "  ")` → `ErrDisplayNameInvalid`; tidak ada user baru dan `user_slots.used` tidak bertambah.
  - auth: `TestGuestDisplayNameStored` (body `{"display_name":"Budi"}` → 201, `/api/me` mengembalikan display name `Budi`); `TestGuestDisplayNameNormalized` (`"  Budi   S  "` → `"Budi S"`); `TestGuestDisplayNameInvalid` (`"x<y"` dan string 41 rune → 400 `display_name_invalid`); `TestGuestInvalidNameNoRateLimit` (6× nama invalid dari IP sama lalu 1× valid → 201, dengan limiter 5/jam); `TestGuestMissingNameNoTurnstile` (Turnstile nonaktif, body kosong → 400 `display_name_invalid`).
- [ ] **Step 2:** `go test -race ./internal/...` → FAIL (compile/assert).
- [ ] **Step 3:** Implementasi. Di `Guest`: baca body (batas 4 KiB) selalu, bukan hanya saat Turnstile aktif; urutan Turnstile → `ValidateDisplayName` → limiter → `svc.CreateGuest`. Perbarui semua call site test.
- [ ] **Step 4:** `go vet ./... && go test -race ./...` → PASS.
- [ ] **Step 5:** Commit `feat(guest): require display name on guest creation` + push.

### Task 2: Web — `GuestStartDialog`

**Files:**
- Create: `web/src/lib/guestName.ts`, `web/src/components/GuestStartDialog.tsx`
- Modify: `web/src/lib/api.ts` (`createGuest`), `web/src/pages/Landing.tsx`, `web/src/hooks/useChat.ts`, `web/src/pages/Chat.tsx`, `web/src/i18n/en.json`, `web/src/i18n/id.json`
- Delete: `web/src/components/GuestContactDialog.tsx` (+ test bila ada)
- Test: `web/src/lib/guestName.test.ts`, `web/src/components/GuestStartDialog.test.tsx`

**Interfaces:**
- Consumes: backend Task 1.
- Produces: `randomGuestName(): string`; `createGuest(displayName: string, turnstileToken?: string): Promise<Guest>` mengirim `{display_name, turnstile_token?}`; `GuestStartDialog({ open, onClose, siteKey }: { open: boolean; onClose: () => void; siteKey: string })` — memuat nama acak saat dibuka, merender Turnstile (komponen yang dipakai Landing sekarang, dipindah), memanggil `createGuest`, lalu `nav("/chat", { state: { guestContact } })` dengan `guestContact` = kontak trim (atau tidak diset bila kosong). `useChat`: bila `location.state.guestContact` ada dan user tamu, kirim sebagai pesan ke `c[0].id` lewat `send`, lalu `nav(".", { replace: true, state: null })`.

- [ ] **Step 1: Test gagal:**
  - `randomGuestName()` 100× cocok `/^natter-[a-z0-9]{4}$/`.
  - Dialog: terbuka dengan input nama berisi pola tersebut; mengosongkan nama → tombol "Start chat" disabled; submit memanggil `createGuest("Budi", token)` (mock api); `whitespace contact is not sent` (kontak `"   "` → navigate tanpa `guestContact`); `invalid name resets turnstile` (mock reject `ApiError` `display_name_invalid` → pesan error nama tampil dan callback reset Turnstile dipanggil).
- [ ] **Step 2:** `cd web && npx vitest run` → FAIL.
- [ ] **Step 3:** Implementasi; kedua tombol tamu di Landing membuka dialog; hapus `GuestContactDialog`, `contactOpen`/`setContactOpen`, `newGuest`, dan kunci i18n `chat.contactTitle/contactHint/contactPlaceholder/skip` bila tak dipakai lagi.
- [ ] **Step 4:** `npx vitest run && npx tsc --noEmit && npm run build` → PASS.
- [ ] **Step 5:** Commit `feat(web): guest start dialog with name and optional contact` + push.

### Task 3: Widget — form nama + kontak

**Files:**
- Create: `widget/src/guestName.ts`, `widget/src/ui/GuestForm.tsx`
- Modify: `widget/src/api.ts`, `widget/src/controller.ts`, `widget/src/ui/Panel.tsx`, `widget/src/ui/Turnstile.tsx` (tanpa auto-submit: `onToken` hanya menyimpan token), `widget/src/i18n.ts`, `widget/src/styles.css`
- Test: `widget/src/guestName.test.ts`, `widget/src/controller.test.ts`, `widget/src/ui/GuestForm.test.tsx`

**Interfaces:**
- Consumes: backend Task 1.
- Produces: `randomGuestName(): string` (salinan logika web; paket terpisah); `api.createGuest(turnstileToken: string, displayName: string)`; `controller.startGuest(turnstileToken: string, displayName: string, contact?: string): Promise<void>` — kontak trim non-kosong di-`queue` ke store sebagai pesan pertama (dikirim saat socket open seperti outbox biasa). `ViewState` menambah `nameError?: boolean`; `display_name_invalid` → tetap `phase: "turnstile"`, `nameError: true`, dan Turnstile direset.

- [ ] **Step 1: Test gagal:**
  - `randomGuestName()` pola `/^natter-[a-z0-9]{4}$/`.
  - controller: `startGuest("tk","Budi","wa 0812")` → `createGuest("tk","Budi")` dipanggil dan outbox berisi body `"wa 0812"`; `whitespace contact is not sent`; `contact queued before socket open is sent after open` (socket palsu belum open → setelah open, frame `send` dengan body kontak terkirim tepat sekali); `invalid name resets turnstile` (reject `display_name_invalid` → `phase "turnstile"`, `nameError true`).
  - GuestForm: input nama terisi pola acak; nama kosong → tombol disabled; tanpa token Turnstile → tombol disabled; klik → `startGuest(token, nama, kontak)`.
- [ ] **Step 2:** `cd widget && npm test` → FAIL.
- [ ] **Step 3:** Implementasi; "Start a new chat" kembali ke form dengan nama acak baru.
- [ ] **Step 4:** `npm test && npx tsc --noEmit && npm run build && npm run size` → PASS, ukuran tercetak ≤ 40960.
- [ ] **Step 5:** Commit `feat(widget): guest form with name and optional contact` + push.

### Task 4: E2E

**Files:**
- Modify: spec di `web/e2e/` yang membuat tamu (`chat.spec.ts`, `guest-retention.spec.ts`, `public.spec.ts`, `revamp.spec.ts`, `screenshots.spec.ts`, `widget.spec.ts`) — alur tamu lewat dialog/form baru.

- [ ] **Step 1:** Ubah helper pembuatan tamu: klik tombol tamu → (opsional) isi nama → Turnstile (test key bila `E2E_TURNSTILE`) → "Start chat". Tambah assertion baru:
  - web: tamu isi nama `"E2E Guest"` dan kontak `"e2e@example.com"` → developer melihat `"E2E Guest"` di daftar percakapan dan pesan `"e2e@example.com"`.
  - widget: sama di halaman host.
- [ ] **Step 2:** Jalankan lokal seperti CI (server :8099, `ALLOWED_ORIGINS` termasuk :8081): `cd web && E2E_BASE_URL=http://localhost:8099 npx playwright test` → semua PASS (kecuali yang memang di-skip).
- [ ] **Step 3:** Commit `test(e2e): guest name and contact flow` + push.
