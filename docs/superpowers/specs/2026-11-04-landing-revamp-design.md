# Desain: Revamp Landing Page + Token Visual Seluruh App

Tanggal: 2026-11-04 · Status: menunggu review
Acuan visual: Superdesign draf A v3 (`8819e522-0be6-44f0-bbb2-12d7a2eb7025`, proyek "natter landing").

## 1. Tujuan

Landing page baru yang utility-first, minimalis, dan berfokus pada produk, dengan bahasa desain yang sama di seluruh app (landing, onboarding, chat, settings, legal, dialog).

- Tagline: **Natter is Matter.** (tidak diterjemahkan)
- Headline: **Conversations, made simple.**
- Filosofi: **Less noise. More conversation.**

**Kriteria sukses**
- Produk (UI chat asli) jadi pusat visual hero.
- Light/Dark/System bisa dipilih di landing dan tersimpan; tanpa kedipan tema saat load.
- Teks hanya menyebut fitur yang ada; tidak ada klaim keamanan yang tidak didukung.
- Landing terasa cepat: JS chat/settings tidak dimuat di `/`.
- Lighthouse (landing, mobile): Accessibility ≥ 95, SEO ≥ 95, Best Practices ≥ 95.
- Semua test (Go, Vitest web/widget, E2E) dan lint hijau.

**Di luar cakupan:** staging permanen (review lewat screenshot otomatis di setiap PR + build lokal); restyle widget (`widget/` tetap); fitur baru; SSR/prerender; halaman per-bahasa untuk SEO.

## 2. Keputusan

| # | Keputusan |
|---|---|
| 1 | Token baru diterapkan ke **seluruh app** |
| 2 | Tanpa staging permanen; tiap PR melampirkan screenshot desktop+HP × terang+gelap |
| 3 | Wordmark dibuat sendiri: "natter" Geist semibold + titik oranye; SVG + favicon |
| 4 | Tagline tetap EN; sisa teks diterjemahkan ke ID (lihat §6) |
| 5 | Aksen tetap **oranye natter**; latar terang **netral** (bukan krem) |

## 3. Token visual

Di `web/src/index.css` (format HSL shadcn, nama variabel tidak berubah sehingga komponen lama ikut berganti):

| Token | Light | Dark |
|---|---|---|
| `--background` | `0 0% 100%` (#FFFFFF) | `240 10% 4%` (#09090B) |
| `--foreground` | `240 10% 4%` (#09090B) | `0 0% 98%` (#FAFAFA) |
| `--card` / `--popover` | `0 0% 100%` | `240 6% 10%` (#18181B) |
| `--muted` / `--secondary` / `--accent` | `240 5% 96%` (#F4F4F5) | `240 4% 16%` (#27272A) |
| `--muted-foreground` | `240 4% 46%` (#71717A) | `240 5% 65%` (#A1A1AA) |
| `--border` / `--input` | `240 6% 90%` (#E4E4E7) | `240 4% 16%` (#27272A) |
| `--primary` / `--ring` | `13 76% 54%` (#E25C2F) | `14 72% 50%` (#DC5A2B) |
| `--primary-foreground` | `0 0% 100%` | `0 0% 100%` |
| `--destructive` | `0 72% 48%` | `0 80% 62%` |
| `--surface` (baru, latar section halus) | `0 0% 98%` (#FAFAFA) | `240 6% 7%` (#111113) |

Kontras teks utama/latar dan teks putih di atas oranye harus ≥ 4.5:1 (dicek; bila oranye+putih < 4.5 untuk teks kecil, tombol memakai font-weight 600 dan ukuran ≥ 14px, atau oranye digelapkan ke `13 76% 48%` khusus untuk tombol — diputuskan saat implementasi dengan angka kontras di PR).

- **Font:** Geist Sans dari Google Fonts (`wght@400;500;600`), ganti Nunito di `index.html` dan `tailwind.config.js`. CSP tidak berubah.
- **Radius:** `--radius: 0.5rem`; frame produk 12–16px. Tombol CTA tidak lagi `rounded-full` (8px).
- **Ikon:** lucide, stroke 1.5.
- **Motion:** opacity/translate ≤ 8px, 200–300ms ease-out, hanya saat elemen masuk viewport; semua di bawah `@media (prefers-reduced-motion: no-preference)`.

## 4. Tema

- Pref tersimpan di `localStorage["natter.theme"]` (sudah ada di `lib/theme.ts`).
- **Script inline kecil di `<head>`** menerapkan kelas `dark` sebelum render (tanpa kedipan). CSP `script-src 'self'` tidak mengizinkan inline → script ditaruh sebagai file `public/theme-init.js` yang dimuat sinkron (tanpa `async/defer`) sebelum bundle.
- Komponen baru `ThemeSwitch`: tiga pilihan Light/Dark/System (radio group, ikon Sun/Moon/Monitor, keyboard-navigable, `aria-checked`). Menggantikan `ThemeToggle` dua-mode di landing dan dipakai juga di Settings.

## 5. Struktur landing

`web/src/pages/landing/` (satu komponen per section); `pages/Landing.tsx` hanya merangkai + logika auth yang ada (redirect user login, config Turnstile, `GuestStartDialog`) tanpa perubahan perilaku.

1. **Nav** (sticky, latar background + border bawah saat scroll): wordmark · Product · How it works · Privacy · `ThemeSwitch` · EN/ID · tombol **Sign in** (Google). Mobile: link disembunyikan di menu (Radix DropdownMenu), wordmark + Sign in tetap terlihat.
2. **Hero**: headline, subteks, CTA **Sign in with Google** (primary) + **Continue as guest** (outline), catatan kecil soal tamu; di bawahnya **screenshot asli** UI chat dua-panel dalam frame netral (desktop), screenshot HP di layar kecil.
3. **Product introduction**: judul "Natter is Matter." + paragraf singkat + kutipan filosofi.
4. **Core features**: daftar teks (bukan grid kartu) — Username search, Real-time messages, Sent & read receipts, Typing indicator, Your controls (block, delete chat, delete account) — berdampingan dengan crop screenshot (receipts/typing).
5. **How it works**: 3 langkah bernomor dalam satu baris (desktop) / tumpuk (mobile).
6. **Brand philosophy**: pernyataan tipografis besar "Less noise. More conversation." di latar `--surface`.
7. **Privacy & trust**: daftar jujur + link Privacy Policy dan GitHub.
8. **Final CTA**: "Ready to start?" + dua tombol.
9. **Footer**: wordmark, "Natter is Matter.", Terms · Privacy · GitHub, © tahun.

## 6. Teks (EN / ID)

| Kunci | EN | ID |
|---|---|---|
| hero.title | Conversations, made simple. | Ngobrol, jadi sederhana. |
| hero.sub | A personal 1:1 messenger. Sign in with Google, or continue as a guest. No phone number needed. | Messenger pribadi 1:1. Masuk dengan Google, atau lanjut sebagai tamu. Tanpa nomor HP. |
| hero.google | Sign in with Google | Masuk dengan Google |
| hero.guest | Continue as guest | Lanjut sebagai tamu |
| hero.guestNote | Guest sessions end after 30 minutes of inactivity. | Sesi tamu berakhir setelah 30 menit tidak aktif. |
| intro.title | Natter is Matter. | Natter is Matter. |
| intro.body | natter removes the friction from starting a conversation. No phone numbers, no complex onboarding, no clutter. Pick a username and start talking. | natter menghapus hambatan untuk memulai obrolan. Tanpa nomor HP, tanpa pendaftaran rumit, tanpa kerumitan. Pilih username dan mulai ngobrol. |
| features.title | What's inside | Yang ada di dalamnya |
| f.search | Username search — Find people by their handle. | Cari username — Temukan orang lewat username-nya. |
| f.realtime | Real-time messages — Delivered instantly over a live connection. | Pesan real-time — Terkirim seketika lewat koneksi langsung. |
| f.receipts | Sent & read receipts — Know when a message arrives and is read. | Tanda terkirim & dibaca — Tahu kapan pesan sampai dan dibaca. |
| f.typing | Typing indicator — See when someone is replying. | Indikator mengetik — Lihat saat lawan bicara sedang membalas. |
| f.controls | Your controls — Block people, delete chats, or delete your account. | Kendali di tanganmu — Blokir orang, hapus chat, atau hapus akun. |
| how.title | How it works | Cara kerjanya |
| how.1 | Sign in — With Google, or as a guest (name + optional contact). | Masuk — Dengan Google, atau sebagai tamu (nama + kontak opsional). |
| how.2 | Pick a username — Choose your handle and display name. | Pilih username — Tentukan username dan nama tampilan. |
| how.3 | Chat — Search someone and send your first message. | Ngobrol — Cari seseorang dan kirim pesan pertamamu. |
| philosophy | Less noise. More conversation. | Lebih sedikit bising. Lebih banyak obrolan. |
| privacy.title | Privacy & trust | Privasi & kepercayaan |
| privacy.sub | Honest about how your data is handled. | Jujur soal bagaimana datamu dikelola. |
| p.e2e | Not end-to-end encrypted. Server administrators can read messages — don't share sensitive information. | Tidak terenkripsi end-to-end. Admin server dapat membaca pesan — jangan bagikan informasi sensitif. |
| p.retention | Conversations are deleted after 1 month without activity. | Percakapan dihapus setelah 1 bulan tanpa aktivitas. |
| p.guests | Guest accounts end after 30 minutes of inactivity; their conversation is kept up to 1 month. | Akun tamu berakhir setelah 30 menit tidak aktif; percakapannya disimpan hingga 1 bulan. |
| p.oss | Open source under the MIT license. | Open source dengan lisensi MIT. |
| cta.title | Ready to start? | Siap mulai? |
| nav.signIn | Sign in | Masuk |

Kunci `landing.*` lama yang tidak dipakai dihapus. FAQ dihapus dari landing (isinya tercakup di Privacy & trust dan halaman Privacy Policy).

## 7. Tampilan produk & aset

- **Screenshot asli** dibuat otomatis oleh Playwright (`web/e2e/screenshots.spec.ts`, diperluas) dari app yang berjalan dengan **data demo** (2–3 pengguna fiktif, percakapan wajar, ada ✓ dan ✓✓, indikator mengetik) — **setelah** token baru diterapkan.
  - Varian: desktop dua-panel (1440×900 crop) dan HP (390×844), masing-masing terang & gelap.
  - Disimpan sebagai AVIF + WebP + PNG fallback di `web/public/product/`, dengan `<picture>`, `srcset` (1x/2x), `width`/`height` eksplisit; hero `fetchpriority="high"`, sisanya `loading="lazy"`.
  - Tema screenshot mengikuti tema aktif (`<picture>` dengan `media="(prefers-color-scheme: dark)"` tidak cukup karena pilihan manual → dua `<img>` dengan kelas `dark:hidden` / `hidden dark:block`).
- **Wordmark** `web/public/brand/natter-wordmark.svg` (teks dikonversi ke path) dan `natter-mark.svg` (titik/gelembung oranye).
- **Favicon/ikon**: `favicon.svg`, `favicon-32.png`, `apple-touch-icon.png` (180), `icon-512.png`, `site.webmanifest`.
- **OG image** `web/public/og.png` 1200×630: wordmark, headline, crop screenshot terang.
- Total berat gambar di atas lipatan pertama (hero) ≤ 200 KB.

## 8. SEO, performa, aksesibilitas

- `index.html`: `<html lang>` diperbarui saat bahasa berubah; `<title>natter — Conversations, made simple.</title>`; `meta description`; canonical `https://chat.rahmattomy.com/`; `og:title/description/image/url/type`; `twitter:card=summary_large_image`; `theme-color` (light/dark); link favicon & manifest.
- `public/robots.txt` (izinkan `/`, larang `/api/`, `/chat`, `/settings`, `/onboarding`).
- Route `/chat`, `/settings`, `/onboarding`, `/privacy`, `/terms` dimuat lewat `React.lazy` + `Suspense`.
- Aksesibilitas: satu `<h1>`, landmark (`header`, `main`, `footer`, `nav`), skip link "Skip to content", fokus terlihat (`ring`), semua kontrol bisa dipakai keyboard, `alt` deskriptif pada screenshot, kontras AA.

## 9. Kualitas & review

- **ESLint** baru: `@eslint/js`, `typescript-eslint`, `eslint-plugin-react-hooks`, `eslint-plugin-jsx-a11y` (rekomendasi); script `npm run lint`; job CI `web-test` menjalankannya.
- **Vitest:** `ThemeSwitch` (pilih → simpan pref + kelas `dark`; System mengikuti `matchMedia`); render landing EN/ID memuat headline & dua CTA.
- **E2E:** sesuaikan `public.spec`, `revamp.spec`, helper `guest.ts` dengan label baru; tambah: theme switch tersimpan setelah reload; navigasi keyboard mencapai Sign in dan Continue as guest.
- **Screenshot review:** setiap PR menyertakan landing, chat, settings (desktop+HP × terang+gelap) sebelum/sesudah.
- **Lighthouse** dijalankan lokal pada build produksi untuk landing (angka di PR).

## 10. Risiko

- Token dipakai bersama → regresi visual di chat/settings/dialog; mitigasi: screenshot sebelum/sesudah per halaman.
- Kontras oranye+putih pada teks kecil (lihat §3).
- Screenshot produk bisa basi saat UI chat berubah → skrip otomatis, dibuat ulang bila perlu.
- E2E bergantung pada label → diperbarui di fase yang sama.
- Font eksternal (Google Fonts) memengaruhi LCP → `preconnect` + `display=swap`, hanya 3 berat.
