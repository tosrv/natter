# Desain: Widget Chat pmess (embeddable)

Tanggal: 2026-11-04 · Status: menunggu review

## 1. Tujuan

Pengunjung web portfolio (dan web lain milik developer) bisa membuka widget chat kecil di pojok halaman dan mengobrol dengan developer sebagai **tamu**, memakai backend pmess yang live di `https://chat.rahmattomy.com`. Developer membalas lewat aplikasi pmess seperti biasa.

**Kriteria sukses**
- Pemasangan cukup dua baris HTML di situs mana pun (framework apa pun).
- Pesan real-time dengan ✓/✓✓ dan indikator mengetik; tidak hilang/dobel saat jaringan putus.
- Refresh/pindah halaman tetap di percakapan yang sama selama sesi tamu aktif.
- Widget tidak pernah merusak tampilan atau skrip situs host.
- `widget.js` ≤ 40 KB gzip.

**Di luar cakupan (YAGNI)**
- Multi-tenant (orang lain memakai widget untuk menerima chat mereka sendiri), site key/API key, daftar origin per tenant. Jika dibutuhkan nanti → proyek & spec terpisah.
- Login Google di widget, dialog kontak tamu, notifikasi suara, kustomisasi posisi/teks.
- Publikasi ke npm/CDN pihak ketiga.

## 2. Keputusan

| # | Keputusan | Alasan |
|---|---|---|
| 1 | `<script>` + Web Component `<pmess-chat>` dengan Shadow DOM | Framework-agnostic; CSS terisolasi |
| 2 | Kode di folder `widget/` dalam repo pmess; disajikan server pmess di `/widget.js` | Satu PR & satu CI untuk perubahan kontrak API; tanpa CDN terpisah |
| 3 | Token tamu disimpan di `localStorage` situs host | Bertahan saat refresh; dampak XSS kecil (akun tamu, 30 menit, hanya bisa chat developer) |
| 4 | Fitur minimal (lihat §4) | Fokus pada inti |
| 5 | Preact + TypeScript, Vite library build → satu file | ~15–25 KB gzip; pola mirip React |
| 6 | Tanpa API key | Key di frontend selalu publik; pengamanan sebenarnya = `ALLOWED_ORIGINS` + Turnstile + rate limit |

## 3. Arsitektur

```
Situs host ──<script src="https://chat.rahmattomy.com/widget.js" defer>
   └─ <pmess-chat>  (Shadow DOM, Preact)
        ├─ REST  https://chat.rahmattomy.com/api/...   Authorization: Bearer <token>
        └─ WSS   wss://chat.rahmattomy.com/api/ws      Sec-WebSocket-Protocol: pmess.bearer, <token>
```

Pemasangan:

```html
<script src="https://chat.rahmattomy.com/widget.js" defer></script>
<pmess-chat></pmess-chat>
```

Atribut opsional: `server` (default: origin asal `widget.js`), `lang` (`en`/`id`, default dari `navigator.language`), `color` (warna utama CSS).

### 3.1 Perubahan backend

1. **Token di respons guest.** `POST /api/auth/guest` menambahkan `"token": "<session token>"` ke body **hanya jika** header `Origin` ada, berbeda dari origin pmess sendiri, dan termasuk `ALLOWED_ORIGINS`. Cookie tetap diset seperti sekarang. Request dari situs pmess sendiri atau tanpa Origin tidak menerima `token`.
2. **Auth WebSocket via subprotocol.** Middleware auth membaca token dari `Sec-WebSocket-Protocol` berbentuk `pmess.bearer, <token>` (hanya untuk `/api/ws`). Saat accept, server memilih subprotocol `pmess.bearer` (`AcceptOptions.Subprotocols`). Token tidak diletakkan di query string agar tidak masuk log akses. Urutan prioritas: header `Authorization` → subprotocol → cookie.
3. **Menyajikan `/widget.js`.** Dibuild di Dockerfile (stage Node, `widget/` → `widget.js`) dan disalin ke image. Header: `Content-Type: text/javascript`, `Cross-Origin-Resource-Policy: cross-origin`, `Cache-Control: public, max-age=300`, ETag. CSP/X-Frame-Options aplikasi tidak berubah (tidak ada iframe).
4. **CORS/Origin.** Tidak ada perubahan kode: CORS REST dan `OriginPatterns` WebSocket sudah memakai `ALLOWED_ORIGINS`. Operator menambahkan origin situs host ke env.

### 3.2 Langkah operator (didokumentasikan di `docs/operations*.md`)
- Tambah `https://<situs>` ke `ALLOWED_ORIGINS` di `.env` VPS, restart.
- Tambah hostname situs ke widget Turnstile di dashboard Cloudflare.
- Jika situs host punya CSP: izinkan `script-src https://chat.rahmattomy.com https://challenges.cloudflare.com`, `connect-src https://chat.rahmattomy.com wss://chat.rahmattomy.com`, `frame-src https://challenges.cloudflare.com`.

## 4. Widget: komponen

`widget/src/`:

| File | Tanggung jawab |
|---|---|
| `main.ts` | `customElements.define("pmess-chat")`, buat Shadow DOM, baca atribut, render Preact; guard agar tidak didefinisikan dua kali |
| `api.ts` | REST: `GET /config`, `POST /auth/guest`, `GET /conversations`, `GET /conversations/:id/messages?after_seq=`; Bearer; error → `ApiError{status, code}` |
| `socket.ts` | WebSocket dengan subprotocol, reconnect backoff (1s→30s, jitter), event `open/close/frame` |
| `store.ts` | State: pesan (dedup per `seq`/`client_msg_id`), outbox, `peerReadSeq`, typing, `ended`; logika murni, mudah diuji |
| `session.ts` | `localStorage` key `pmess:<server>` (try/catch; fallback memori bila diblokir) |
| `i18n.ts` | Kamus EN/ID tanpa library |
| `ui/` | `Launcher`, `Panel`, `Turnstile`, `MessageList`, `Composer`, `Ended`, banner koneksi |
| `styles.css` | Disuntik ke Shadow DOM; `--pm-color`; `prefers-color-scheme` |

Fitur: tombol bulat kanan-bawah → panel; Turnstile → sesi tamu; kirim/terima real-time, ✓/✓✓, mengetik; outbox + catch-up; status berakhir + "Mulai chat baru"; EN/ID; warna lewat atribut; terang/gelap mengikuti sistem.

## 5. Alur data

1. **Muat halaman:** hanya tombol; tidak ada request jaringan.
2. **Buka panel:**
   - Token ada → `GET /conversations`. Valid → muat pesan, sambung WS. 401 → hapus token, lanjut ke bawah.
   - Tidak ada token → `GET /config` (site key) → render Turnstile → `POST /auth/guest {turnstile_token}` → simpan `token` → sambung WS.
3. **Kirim:** buat `client_msg_id` (UUID), masuk outbox, kirim frame `send`. `ack` → keluar outbox, ✓. Frame `read` dengan seq ≥ pesan → ✓✓.
4. **Reconnect:** setelah `open`, `GET messages?after_seq=<terakhir>` (paging sampai habis), lalu kirim ulang outbox dengan `client_msg_id` sama.
5. **Berakhir:** frame `conversation_ended`, error `peer_ended`, atau 401 → hapus token, tutup WS, tampilkan "Percakapan berakhir" + tombol **Mulai chat baru** (kembali ke langkah Turnstile).

## 6. Penanganan error

| Kondisi | Perilaku |
|---|---|
| Offline / WS putus | Banner "Menyambungkan…", outbox ditahan, kirim ulang setelah tersambung (dedup oleh `UNIQUE(sender_id, client_msg_id)`) |
| `limit_reached` / 429 | Pesan "Sedang penuh, coba lagi nanti"; tanpa retry otomatis |
| Turnstile gagal / CORS ditolak / server tak terjangkau | "Chat tidak tersedia" + `console.warn` berisi detail |
| `localStorage` diblokir | Token di memori saja (sesi hilang saat refresh) |
| Error tak terduga | Ditangkap di dalam widget; tidak ada global selain elemen `<pmess-chat>` |

## 7. Pengujian

- **Go (`-race`, Postgres asli):**
  - Guest dengan Origin diizinkan → body berisi `token`; dari origin pmess/tanpa Origin/origin asing → tanpa `token`.
  - WS dengan `pmess.bearer, <token valid>` → tersambung, subprotocol `pmess.bearer` dipilih; token salah → 401.
  - `GET /widget.js` → 200, header CORP & content-type benar.
- **Vitest (`widget/`):** `session` (termasuk localStorage error), `store` (outbox/ack, dedup, read receipt, ended), `socket` (reconnect + catch-up dengan WebSocket palsu).
- **Playwright E2E:** `host.html` disajikan dari origin berbeda (port lain) yang memuat `widget.js`; tamu kirim → developer terima & balas → ✓✓; refresh → percakapan tetap; sesi diakhiri → "Mulai chat baru". Turnstile memakai test key Cloudflare.
- **Ukuran:** job CI gagal jika `widget.js` > 40 KB gzip.

**Verifikasi sendiri:**

```bash
go test -race ./...
cd widget && npm ci && npm test && npm run build && gzip -c dist/widget.js | wc -c
cd web && npx playwright test   # termasuk skenario widget
```

## 8. Dokumentasi
- `widget/README.md` (EN): pemasangan, atribut, CSP host, batasan.
- `docs/operations.md` / `.id.md`: langkah operator §3.2.
- README utama: bagian singkat "Embeddable widget".
