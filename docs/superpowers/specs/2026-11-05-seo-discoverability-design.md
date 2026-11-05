# Desain: SEO & AI Discoverability

Tanggal: 2026-11-05 · Status: menunggu review

## 1. Tujuan

Halaman publik natter (`/`, `/privacy`, `/terms`) mudah ditemukan mesin pencari dan crawler AI; halaman privat tidak diindeks; arsitektur Go + React SPA tetap.

**Kriteria sukses**
- HTML awal (tanpa JavaScript) `/`, `/privacy`, `/terms` memuat h1, teks utama, dan link — bukan `<div id="root"></div>` kosong.
- Metadata (title, description, canonical, OG) benar **per halaman**.
- `/robots.txt`, `/sitemap.xml`, `/llms.txt` tersedia dengan content-type benar.
- JSON-LD `SoftwareApplication` valid di landing.
- Route privat dan API mengirim `X-Robots-Tag: noindex, nofollow`; path tak dikenal → 404 (bukan soft 404).
- API privat tetap 401 tanpa sesi.
- Chat/settings/onboarding tetap SPA; tidak ada perubahan fitur.

**Di luar cakupan:** Astro/Next.js; URL per bahasa (`/id/`); layanan SEO berbayar; perubahan fitur.

## 2. Keputusan

| # | Keputusan |
|---|---|
| 1 | Bot pencarian **dan** bot pelatihan diizinkan untuk halaman publik; `/api/` ditutup untuk semua |
| 2 | Prerender statis saat build dengan `react-dom/server` (tanpa dependensi baru); bukan Astro/Next.js |
| 3 | Prerender dalam bahasa EN + tema default; ID dan tema pilihan diterapkan di klien |

## 3. Temuan audit (sebelum perubahan)

Diuji pada build produksi lokal (Go + `web/dist`):

| URL | Status / type | Masalah |
|---|---|---|
| `/` | 200 html | `<div id="root"></div>` kosong |
| `/sitemap.xml`, `/llms.txt` | 200 **text/html** | tidak ada → soft 404 (index.html) |
| `/privacy`, `/terms` | 200 html | canonical/title/description = landing |
| `/chat`, `/settings`, `/onboarding` | 200 html | tanpa `noindex`; hanya `Disallow` di robots |
| `/nope` | 200 html | soft 404 |
| `/api/me`, `/api/conversations` | 401 json | ✅ |
| JSON-LD | — | tidak ada |

## 4. Prerender

- `web/src/entry-prerender.tsx` mengekspor `render(path: "/" | "/privacy" | "/terms"): string` — `renderToString` dari pohon yang sama dengan app (`StaticRouter` dari `react-router-dom/server`, i18n `lng: "en"`), tanpa efek samping browser.
- `web/scripts/prerender.mjs` (dijalankan oleh `npm run build` setelah `vite build` dan `vite build --ssr src/entry-prerender.tsx --outDir dist-ssr`): untuk tiap route, ambil `dist/index.html`, sisipkan HTML hasil render ke `<div id="root">`, ganti blok `<!--head-->` dengan metadata per halaman (§5), tulis `dist/index.html`, `dist/privacy/index.html`, `dist/terms/index.html`. Hapus `dist-ssr` setelahnya. Juga menulis `dist/sitemap.xml` dengan `lastmod` tanggal build.
- `main.tsx`: bila `#root` sudah berisi markup **dan** path ∈ {`/`, `/privacy`, `/terms`} → `hydrateRoot`; selain itu `createRoot` (SPA biasa).
- Bahasa: hydrate dengan EN; bila bahasa tersimpan ID, ganti ke ID **setelah** hydrate (efek di `main.tsx` setelah render pertama) agar tidak terjadi hydration mismatch. Tema tidak memengaruhi markup (kelas `dark` di `<html>` oleh `theme-init.js`).
- Komponen yang membaca `window`/`localStorage`/`document` saat render (ThemeToggle, LanguageToggle, Landing auth effect) harus aman di server: akses hanya di `useEffect` atau di-guard `typeof window`.
- Landing tetap mengalihkan user login ke `/chat` di klien (tidak berubah).

## 5. Metadata per halaman

| Route | title | description | canonical |
|---|---|---|---|
| `/` | natter — Conversations, made simple. | natter is a simple personal 1:1 messenger. Sign in with Google or continue as a guest — no phone number needed. | `https://chat.rahmattomy.com/` |
| `/privacy` | Privacy Policy — natter | How natter collects, uses and deletes your data. | `https://chat.rahmattomy.com/privacy` |
| `/terms` | Terms of Service — natter | The short terms for using natter. | `https://chat.rahmattomy.com/terms` |

OG (`og:title`, `og:description`, `og:url`) mengikuti tabel; `og:image` tetap `/og.png` untuk semua. Di klien, saat navigasi SPA antar halaman publik, `document.title` diperbarui (hook kecil `useDocumentTitle`).

**JSON-LD** (hanya `/`):

```json
{
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  "name": "natter",
  "url": "https://chat.rahmattomy.com/",
  "description": "A simple personal 1:1 real-time messenger. Natter is Matter.",
  "applicationCategory": "CommunicationApplication",
  "operatingSystem": "Web",
  "inLanguage": ["en", "id"],
  "isAccessibleForFree": true,
  "offers": { "@type": "Offer", "price": "0", "priceCurrency": "USD" },
  "license": "https://opensource.org/licenses/MIT",
  "author": { "@type": "Person", "name": "Rahmat Tomy" },
  "sameAs": ["https://github.com/tosrv/natter"]
}
```

Tanpa rating/ulasan. Blok `application/ld+json` tidak dieksekusi sehingga aman terhadap CSP.

## 6. Server Go (`internal/api/router.go`)

- Daftar route SPA privat: `/chat`, `/settings`, `/onboarding` → sajikan `index.html` dengan `X-Robots-Tag: noindex, nofollow`.
- Route publik: `/` → `dist/index.html`; `/privacy`, `/terms` → `dist/<route>/index.html` (sudah tertangani oleh lookup file + dukungan direktori: `p/index.html` bila `p` direktori).
- File statis yang ada → disajikan apa adanya (content-type dari ekstensi; `.txt` text/plain, `.xml` application/xml).
- Path lain → **404** dengan body `index.html` (SPA tetap bisa menampilkan redirect ke `/`) dan `X-Robots-Tag: noindex`.
- `/api/*` → semua respons diberi `X-Robots-Tag: noindex, nofollow` (middleware grup `/api`).
- Otorisasi API tidak diubah.

## 7. File statis

**`web/public/robots.txt`:**

```
# Search engines: index public pages.
User-agent: Googlebot
User-agent: Bingbot
Allow: /
Disallow: /api/

# AI search/answer engines that cite and link to sources.
User-agent: OAI-SearchBot
User-agent: PerplexityBot
Allow: /
Disallow: /api/

# AI training crawlers: public marketing and legal pages only (no user data is public).
User-agent: GPTBot
User-agent: ClaudeBot
Allow: /
Disallow: /api/

# Everyone else.
User-agent: *
Allow: /
Disallow: /api/

Sitemap: https://chat.rahmattomy.com/sitemap.xml
```

Route privat **tidak** di-`Disallow` agar crawler dapat membaca `X-Robots-Tag: noindex`; isinya hanya kerangka SPA, dan data dilindungi otorisasi API. robots.txt bukan kontrol akses.

**`dist/sitemap.xml`** (dibuat saat build): `/`, `/privacy`, `/terms` dengan `lastmod`.

**`web/public/llms.txt`** (format llmstxt.org): judul `# natter`, ringkasan satu paragraf (Natter is Matter; messenger 1:1 pribadi), bagian *Features* (hanya fitur yang ada), *Privacy* (tidak end-to-end encrypted, retensi 1 bulan, tamu berakhir setelah 30 menit), *Technology* (Go, PostgreSQL, WebSocket, React, open source MIT), *Links* (landing, privacy, terms, GitHub).

## 8. Validasi (hasil dilaporkan sebelum PR)

- Test Go: `X-Robots-Tag` pada `/chat`, `/settings`, `/onboarding`, `/api/me`; 404 + noindex untuk path tak dikenal; `/privacy` menyajikan `privacy/index.html`; content-type `robots.txt`, `sitemap.xml`, `llms.txt`; endpoint API privat 401 tanpa sesi (tabel semua route di grup authed).
- Vitest: `render("/")` memuat h1 "Conversations, made simple."; `render("/privacy")` memuat judul Privacy; skrip prerender menghasilkan canonical/title per halaman dan JSON-LD yang `JSON.parse`-able dengan field wajib; sitemap XML ter-parse (DOMParser) dan berisi 3 URL.
- Playwright: `javaScriptEnabled: false` → `/` memuat h1 dan tombol/link CTA; dengan JS: tidak ada error "hydration" di console di `/`, `/privacy`, `/terms` (EN dan ID tersimpan).
- `curl` semua endpoint pada build produksi (status, content-type, `X-Robots-Tag`, canonical).
- E2E, lint, unit test yang ada tetap hijau.
