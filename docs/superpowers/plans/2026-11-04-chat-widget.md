# Chat Widget Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Widget `<natter-chat>` (satu file `widget.js`) yang bisa dipasang di situs lain agar pengunjung chat dengan developer sebagai tamu lewat backend natter.

**Architecture:** Backend Go ditambah dua jalur auth lintas origin (token di respons guest untuk origin yang diizinkan; token WebSocket via subprotocol) dan menyajikan `/widget.js`. Widget: Preact + TypeScript di `widget/`, Vite library build ke satu IIFE, Shadow DOM, token di `localStorage`.

**Tech Stack:** Go 1.24/Gin/coder/websocket; Preact 10, TypeScript, Vite 5, Vitest; Playwright (sudah ada di `web/`).

**Spec:** `docs/superpowers/specs/2026-11-04-chat-widget-design.md`

## Global Constraints

- `widget.js` ≤ 40 KB gzip (40960 byte); satu file IIFE, tanpa dependensi runtime selain Preact yang ter-bundle.
- Tidak ada global selain custom element `natter-chat`; semua CSS di dalam Shadow DOM.
- Subprotocol: tepat `natter.bearer`; klien mengirim `["natter.bearer", token]`.
- Key localStorage: `natter:<server origin>`.
- Token di body guest hanya jika `Origin` ∈ `ALLOWED_ORIGINS` **dan** ≠ origin request (`scheme://Host`).
- Tanpa iframe; CSP / X-Frame-Options aplikasi tidak diubah.
- Header `/widget.js`: `Cross-Origin-Resource-Policy: cross-origin`, `Cache-Control: public, max-age=300`.
- Atribut: `server`, `lang` (`en`|`id`), `color`.
- Reconnect backoff 1s→30s dengan jitter.
- Teks UI EN/ID; dokumentasi operasional EN + ID, spec/plan Bahasa Indonesia.
- Setiap task: commit + push ke `feat/chat-widget`.

## Review Focus

1. Token sesi tamu ditolak/kedaluwarsa saat panel dibuka dari localStorage lama → widget harus jatuh ke Turnstile, bukan macet di "loading" (Task 5 test `stale token → turnstile`).
2. Pesan dikirim saat offline lalu reconnect → tepat satu salinan tampil dan di server (Task 4 test `resend after reconnect keeps one copy`).
3. `localStorage` melempar error (Safari private / diblokir) → widget tetap berfungsi dengan memori (Task 3 test `falls back to memory when storage throws`).
4. Script dimuat dua kali di halaman → tidak error `customElements.define` (Task 5 test `double load is harmless`).
5. Origin asing memanggil `POST /auth/guest` → tidak menerima token di body (Task 1 test `TestGuestTokenForeignOrigin`).

---

### Task 1: Backend — token di respons guest untuk origin yang diizinkan

**Files:**
- Modify: `internal/auth/guest.go`
- Create: `internal/auth/origin.go`
- Test: `internal/auth/auth_test.go`

**Interfaces:**
- Produces: `func embedOrigin(c *gin.Context, allowed []string) bool` — true jika header `Origin` tidak kosong, ada di `allowed` (persis, tanpa trailing slash), dan tidak sama dengan `scheme://c.Request.Host` (scheme `https` jika `cfg.CookieSecure` atau `X-Forwarded-Proto: https`, selain itu `http`). Respons guest: `{"id","username","conversation_id","token"?}`.

- [ ] **Step 1: Tulis test gagal** di `auth_test.go` memakai helper guest yang ada, dengan `cfg.AllowedOrigins = []string{"https://port.example"}`:
  - `TestGuestTokenAllowedOrigin`: header `Origin: https://port.example` → 201, `body["token"]` non-kosong dan `UserBySession(token)` mengembalikan user tamu yang sama; cookie `pm_session` tetap diset.
  - `TestGuestTokenSameOrigin`: `Origin` = origin request sendiri (dan dimasukkan juga ke allowed) → tidak ada key `token`.
  - `TestGuestTokenForeignOrigin`: `Origin: https://evil.example` → tidak ada key `token`.
  - `TestGuestTokenNoOrigin`: tanpa Origin → tidak ada key `token`.
- [ ] **Step 2:** `go test -race ./internal/auth -run TestGuestToken` → FAIL (`token` tidak ada).
- [ ] **Step 3:** Implementasi `embedOrigin` di `origin.go`; di `Guest`, setelah `startSession`, ambil token sesi yang baru dibuat (ubah `startSession` agar mengembalikan `(tok string, ok bool)` dan sesuaikan pemanggil lain) dan tambahkan `"token"` ke `gin.H` bila `embedOrigin(c, a.cfg.AllowedOrigins)`.
- [ ] **Step 4:** `go test -race ./internal/auth` → PASS.
- [ ] **Step 5:** Commit `feat(auth): return guest token to allowed embed origins` + push.

### Task 2: Backend — auth WebSocket via subprotocol + sajikan `/widget.js`

**Files:**
- Modify: `internal/auth/session.go` (`token()`), `internal/realtime/conn.go` (AcceptOptions), `internal/api/router.go` (header `/widget.js`)
- Test: `internal/auth/auth_test.go`, `internal/realtime/hub_test.go`, `internal/api/router_test.go` (atau file test api yang ada)

**Interfaces:**
- Produces: konstanta `auth.WSSubprotocol = "natter.bearer"`. `token(c)` urutan: `Authorization: Bearer` → `Sec-WebSocket-Protocol` (hanya jika path `/api/ws`, nilai di-split koma+trim, elemen pertama `natter.bearer`, elemen kedua = token) → cookie.

- [ ] **Step 1: Test gagal:**
  - `TestMiddlewareSubprotocolToken` (auth): request `GET /api/ws` dengan header `Sec-WebSocket-Protocol: natter.bearer, <tok>` → lolos middleware; path lain dengan header yang sama → 401.
  - `TestWSSubprotocolAuth` (realtime): dial memakai `websocket.DialOptions{Subprotocols: []string{"natter.bearer", tok}}` tanpa cookie, dengan Origin yang diizinkan → tersambung dan `ws.Subprotocol() == "natter.bearer"`; token salah → dial gagal dengan status 401.
  - `TestWidgetJSHeaders` (api): `StaticDir` temp berisi `widget.js` → `GET /widget.js` 200, `Cross-Origin-Resource-Policy: cross-origin`, `Cache-Control: public, max-age=300`, Content-Type diawali `text/javascript`.
- [ ] **Step 2:** Jalankan ketiganya → FAIL.
- [ ] **Step 3:** Implementasi: perluas `token()`; `AcceptOptions{OriginPatterns: h.origins, Subprotocols: []string{auth.WSSubprotocol}}`; di NoRoute (atau route eksplisit `GET /widget.js` sebelum NoRoute) set header di atas lalu `c.File`. Pastikan `SecurityHeaders` tidak menimpa CORP (tidak menyetel CORP, jadi aman).
- [ ] **Step 4:** `go test -race ./...` → PASS.
- [ ] **Step 5:** Commit `feat: websocket subprotocol auth and widget.js serving` + push.

### Task 3: Widget scaffold + `session`, `api`, `i18n`

**Files:**
- Create: `widget/package.json`, `widget/tsconfig.json`, `widget/vite.config.ts`, `widget/src/session.ts`, `widget/src/api.ts`, `widget/src/i18n.ts`, `widget/src/types.ts`
- Test: `widget/src/session.test.ts`, `widget/src/api.test.ts`

**Interfaces:**
- Produces:
  - `types.ts`: `Message {id:number; conversation_id:number; sender_id:number; seq:number; body:string; client_msg_id:string; created_at:string}` (salin bentuk dari `web/src/lib/api.ts`), `Conversation` (field yang dipakai: `id, peer_id, peer_last_read_seq, peer_ended`).
  - `session.ts`: `createSession(server: string, storage?: Storage): { get(): string|null; set(t: string): void; clear(): void }`.
  - `api.ts`: `class ApiError extends Error {status:number; code:string}`; `createApi(server: string, getToken: () => string|null)` → `{ config(): Promise<{turnstile_site_key:string}>; createGuest(turnstileToken: string): Promise<{token:string; conversation_id:number; id:number}>; conversations(): Promise<Conversation[]>; messagesAfter(convId:number, afterSeq:number, limit?:number): Promise<Message[]> }`. Semua fetch `credentials: "omit"`, header `Authorization: Bearer` bila token ada; error body `{error:{code}}` → `ApiError`.
  - `i18n.ts`: `t(lang: "en"|"id", key: Key): string`, `pickLang(attr?: string|null): "en"|"id"`.
- npm scripts: `test` = `vitest run`, `build` = `vite build` (lib, format `iife`, entry `src/main.ts`, `fileName: () => "widget.js"`, CSS di-inline via `?inline` import), `size` = script node yang gagal bila gzip `dist/widget.js` > 40960.

- [ ] **Step 1: Test gagal:**
  - `session`: set/get/clear memakai key `natter:https://chat.example`; `falls back to memory when storage throws` (storage palsu yang `throw` di getItem/setItem → set lalu get tetap mengembalikan token).
  - `api`: `createGuest` mengirim JSON `{turnstile_token}` ke `${server}/api/auth/guest` dengan `credentials:"omit"`; `conversations` mengirim `Authorization: Bearer abc`; respons 401 `{error:{code:"unauthorized"}}` → reject `ApiError` status 401 code `unauthorized`.
- [ ] **Step 2:** `cd widget && npm install && npm test` → FAIL.
- [ ] **Step 3:** Implementasi.
- [ ] **Step 4:** `npm test` → PASS.
- [ ] **Step 5:** Commit (termasuk `package-lock.json`) + push.

### Task 4: Widget `store` + `socket`

**Files:**
- Create: `widget/src/store.ts`, `widget/src/socket.ts`
- Test: `widget/src/store.test.ts`, `widget/src/socket.test.ts`

**Interfaces:**
- Consumes: `Message`, `createApi().messagesAfter` (Task 3).
- Produces:
  - `store.ts` (murni, tanpa I/O): `type State {messages: Message[]; outbox: {client_msg_id:string; body:string}[]; peerReadSeq:number; peerTyping:boolean; ended:boolean; online:boolean}`; `initial(): State`; `reduce(s: State, e: Event): State` dengan `Event` = `{t:"queue"; client_msg_id; body}` | `{t:"ack"|"message"; message: Message}` | `{t:"history"; messages: Message[]}` | `{t:"read"; seq:number}` | `{t:"typing"; on:boolean}` | `{t:"ended"}` | `{t:"online"; on:boolean}`; `lastSeq(s): number`. Dedup per `client_msg_id` dan `seq`; `ack`/`message` dengan `client_msg_id` di outbox menghapusnya; pesan diurut per `seq`.
  - `socket.ts`: `createSocket(opts:{url:string; token:()=>string|null; onFrame:(f:any)=>void; onOpen:()=>void; onClose:(code:number)=>void; WS?: typeof WebSocket; random?: ()=>number}) → {send(frame:object):boolean; close():void}`. Membuka `new WS(url, ["natter.bearer", token])`; reconnect delay `min(30000, 1000*2^n) * (0.5 + random()/2)`, reset setelah open; berhenti reconnect setelah `close()` atau close code 4401/1008 (auth) — laporkan ke `onClose`.
- Frame server (lihat `web/src/lib/socket.ts`): `ack`, `message`, `typing`, `read`, `error{code}`, `conversation_ended`. Frame kirim: `{type:"send", conversation_id, client_msg_id, body}`, `{type:"typing", conversation_id}`, `{type:"read", conversation_id, seq}` — salin nama field persis dari `web/src/lib/socket.ts`.

- [ ] **Step 1: Test gagal:**
  - store: queue lalu ack → outbox kosong, satu pesan; `resend after reconnect keeps one copy` (queue, history berisi pesan yang sama, ack duplikat → tetap 1 pesan); `message` urutan seq 3 lalu 2 → terurut; `read` seq 5 → `peerReadSeq` 5 dan tidak turun bila datang seq 4; `ended` → `ended:true`.
  - socket (FakeWebSocket + `vi.useFakeTimers`): protokol yang dikirim `["natter.bearer","tok"]`; close tak sengaja → reconnect setelah ≤1000ms, lalu ≤2000ms; setelah `close()` tidak reconnect; `send` saat belum open → `false`.
- [ ] **Step 2:** `npm test` → FAIL.
- [ ] **Step 3:** Implementasi.
- [ ] **Step 4:** `npm test` → PASS.
- [ ] **Step 5:** Commit + push.

### Task 5: Widget UI + Web Component + build

**Files:**
- Create: `widget/src/main.ts`, `widget/src/controller.ts`, `widget/src/ui/{App,Launcher,Panel,Turnstile,MessageList,Composer,Ended}.tsx`, `widget/src/styles.css`, `widget/index.html` (halaman dev)
- Test: `widget/src/controller.test.ts`, `widget/src/main.test.ts`

**Interfaces:**
- Consumes: Task 3 & 4.
- Produces:
  - `controller.ts`: `createController(deps:{api; session; socketFactory}) → {state(): ViewState; subscribe(cb):()=>void; open():Promise<void>; startGuest(turnstileToken:string):Promise<void>; send(body:string):void; typing():void; restart():void}`. `ViewState = {phase:"idle"|"loading"|"turnstile"|"chat"|"ended"|"full"|"unavailable"; chat: State}`. Alur sesuai spec §5: `open()` → token? `conversations()` → `chat`, connect, `messagesAfter(lastSeq)` loop sampai < limit; 401 → `session.clear()` → `turnstile`. Setelah socket open: catch-up lalu kirim ulang outbox. `conversation_ended`/error `peer_ended`/401 → clear token, `ended`. `limit_reached`/429 → `full`. Error lain (fetch gagal/CORS/`captcha_failed`) → `unavailable` + `console.warn("[natter]", err)`. Kirim frame `read` untuk pesan masuk terbaru saat panel terbuka.
  - `main.ts`: `if (!customElements.get("natter-chat")) customElements.define("natter-chat", NatterChat)`; `server` default = origin dari `document.currentScript.src` (ditangkap saat script dievaluasi), fallback `location.origin`; `attachShadow({mode:"open"})`, suntik `<style>` dari CSS inline, `--pm-color` dari atribut `color`; render `App`.
  - Turnstile: muat `https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit` sekali, `turnstile.render(container, {sitekey, callback})` pada elemen di dalam shadow root.
  - Teks UI dari `i18n.ts` (kunci minimal: `open, close, title, placeholder, send, connecting, ended, newChat, full, unavailable, typing`).

- [ ] **Step 1: Test gagal:**
  - controller (api/session/socket palsu): `stale token → turnstile` (conversations reject 401 → phase `turnstile`, session dikosongkan); `valid token → chat` dan socket dibuat; `startGuest` menyimpan token lalu `chat`; frame `conversation_ended` → `ended` dan token dihapus; createGuest reject code `limit_reached` → `full`; fetch TypeError → `unavailable`; `restart()` dari `ended` → `turnstile`.
  - main (jsdom): `double load is harmless` (import modul dua kali via `vi.resetModules` → tidak throw); elemen punya shadowRoot berisi tombol launcher dengan `aria-label` "Open chat" saat `lang="en"`.
- [ ] **Step 2:** `npm test` → FAIL.
- [ ] **Step 3:** Implementasi.
- [ ] **Step 4:** `npm test && npm run build && npm run size` → PASS, ukuran tercetak ≤ 40960.
- [ ] **Step 5:** Commit + push.

### Task 6: Dockerfile, CI, E2E lintas origin

**Files:**
- Modify: `Dockerfile` (stage `widget` → salin `widget/dist/widget.js` ke `/app/web/widget.js`), `.github/workflows/ci.yml` (job `widget-test`: `npm ci`, `npm test`, `npm run build`, `npm run size`; job e2e: tambah `http://localhost:8081` ke `ALLOWED_ORIGINS` server dan sajikan `web/e2e/fixtures/host.html` di port 8081 via `npx http-server`/`python3 -m http.server`), `docker-compose*.yml` bila e2e memakai compose
- Create: `web/e2e/fixtures/host.html`, `web/e2e/widget.spec.ts`

**Interfaces:**
- Consumes: `/widget.js` (Task 2/5); `host.html` memuat `<script src="http://localhost:8080/widget.js" defer></script><natter-chat lang="en"></natter-chat>`.
- Env E2E: `E2E_HOST_URL` default `http://localhost:8081/host.html`.

- [ ] **Step 1: Tulis `widget.spec.ts`** (pola login developer palsu dan Turnstile test key mengikuti `guest-retention.spec.ts`):
  - tamu di host.html buka panel → Turnstile (test key) → kirim "hi from widget" → developer (konteks browser lain di app natter) melihat pesan dan membalas → tamu melihat balasan; setelah developer membuka percakapan, pesan tamu bertanda ✓✓.
  - reload host.html → buka panel → pesan lama masih tampil tanpa Turnstile.
  - developer/servis mengakhiri sesi tamu (pakai mekanisme yang sama dengan `guest-retention.spec.ts`) → widget menampilkan "Start new chat".
- [ ] **Step 2:** Jalankan lokal (server dengan `E2E_FAKE_LOGIN=1`, `ALLOWED_ORIGINS=http://localhost:8080,http://localhost:8081`, host di 8081): `cd web && npx playwright test widget` → PASS.
- [ ] **Step 3:** Update Dockerfile & CI; `docker build .` sukses dan `docker run` → `curl -I localhost:8080/widget.js` menunjukkan header CORP.
- [ ] **Step 4:** Commit + push; pastikan semua job CI hijau di PR.

### Task 7: Dokumentasi

**Files:**
- Create: `widget/README.md`
- Modify: `README.md`, `README.id.md` (bagian singkat "Embeddable widget"), `docs/operations.md`, `docs/operations.id.md` (langkah operator spec §3.2)

- [ ] **Step 1:** `widget/README.md`: pemasangan dua baris, atribut `server/lang/color`, CSP host persis seperti spec §3.2, batasan (tamu saja, token di localStorage, tanpa multi-tenant), cara dev (`npm run dev`, `npm test`, `npm run build`, `npm run size`).
- [ ] **Step 2:** operations: tambah origin ke `ALLOWED_ORIGINS` + restart, tambah hostname di Turnstile, cara cek `curl -I https://chat.rahmattomy.com/widget.js`.
- [ ] **Step 3:** Commit + push.
