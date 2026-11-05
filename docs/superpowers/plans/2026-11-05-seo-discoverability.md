# SEO & AI Discoverability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Halaman publik (`/`, `/privacy`, `/terms`) diprerender dengan metadata per halaman, `robots.txt`/`sitemap.xml`/`llms.txt`/JSON-LD tersedia, dan route privat + API ber-`noindex` tanpa mengubah fitur.

**Architecture:** Server Go mengatur status dan header `X-Robots-Tag` per jenis path. Build web menjalankan SSR build Vite atas `entry-prerender.tsx` lalu skrip yang menulis HTML statis per route + sitemap; `main.tsx` meng-hydrate route publik dan tetap `createRoot` untuk lainnya.

**Tech Stack:** Go/Gin; React 18 (`react-dom/server`, `react-router-dom/server` StaticRouter), Vite 5 SSR build, Vitest, Playwright. Tidak ada dependensi baru.

**Spec:** `docs/superpowers/specs/2026-11-05-seo-discoverability-design.md`

## Global Constraints

- Tidak ada dependensi npm/Go baru; tidak ada layanan berbayar; arsitektur Go + React SPA tetap.
- Route publik: `/`, `/privacy`, `/terms`. Route SPA privat: `/chat`, `/settings`, `/onboarding`.
- Header persis: `X-Robots-Tag: noindex, nofollow` untuk route privat dan `/api/*`; `X-Robots-Tag: noindex` untuk 404.
- Metadata per halaman, `robots.txt`, JSON-LD, dan isi `llms.txt` persis spec §5 dan §7.
- Prerender bahasa EN; ID diterapkan setelah hydrate.
- Otorisasi API tidak diubah. Tidak ada perubahan fitur/perilaku chat.
- Setiap task: commit + push ke `feat/seo-discoverability`.

## Review Focus

1. Pengguna dengan bahasa tersimpan ID membuka `/` → tidak ada peringatan hydration, teks berganti ke ID (Task 3: Playwright `no hydration warnings with stored id`).
2. Pengguna yang sudah login membuka `/` → tetap diarahkan ke `/chat` setelah hydrate (Task 3: E2E redirect yang ada tetap hijau).
3. Path dengan garis miring akhir (`/privacy/`) dan query (`/?ref=x`) → tetap 200 dengan halaman yang benar (Task 1: `TestPublicRouteVariants`).
4. `HEAD /` dan `HEAD /chat` → header sama dengan GET (Task 1: `TestHeadRequests`).
5. Path traversal `/../go.mod` → tidak membocorkan file di luar `StaticDir` (Task 1: `TestNoTraversal`, perilaku lama dipertahankan).

---

### Task 1: Server Go — status & X-Robots-Tag

**Files:**
- Modify: `internal/api/router.go` (NoRoute + middleware grup `/api`)
- Test: `internal/api/seo_test.go`

**Interfaces:**
- Produces: `var privateSPARoutes = []string{"/chat", "/settings", "/onboarding"}`; `var publicRoutes = []string{"/", "/privacy", "/terms"}`. Lookup file: bila path menunjuk direktori yang berisi `index.html` → sajikan file itu.

- [ ] **Step 1: Test gagal** (StaticDir temp berisi `index.html`, `privacy/index.html`, `terms/index.html`, `robots.txt`, `sitemap.xml`, `llms.txt`):
  - `/` 200 tanpa `X-Robots-Tag`; `/privacy` dan `/terms` 200 berisi isi `privacy/index.html`/`terms/index.html`.
  - `/chat`, `/settings`, `/onboarding` 200 berisi `index.html` dengan `X-Robots-Tag: noindex, nofollow`.
  - `/nope` dan `/a/b` → 404, body `index.html`, `X-Robots-Tag: noindex`.
  - `/robots.txt` `text/plain`, `/sitemap.xml` `application/xml` atau `text/xml`, `/llms.txt` `text/plain`.
  - `/api/me` tanpa sesi → 401 dengan `X-Robots-Tag: noindex, nofollow`; tabel semua route grup authed (GET/POST/PUT/DELETE) → 401 tanpa sesi.
  - `TestPublicRouteVariants`: `/privacy/` → 200 privacy; `/?ref=x` → 200 index.
  - `TestHeadRequests`: HEAD `/` dan `/chat` → status & `X-Robots-Tag` sama dengan GET.
  - `TestNoTraversal`: `/../go.mod` → bukan isi file di luar StaticDir.
- [ ] **Step 2:** `go test -race ./internal/api -run 'SEO|Public|Head|Traversal'` → FAIL.
- [ ] **Step 3:** Implementasi; set content-type eksplisit untuk `.xml` dan `.txt` bila `c.File` tidak memberi tipe yang benar.
- [ ] **Step 4:** `go vet ./... && go test -race ./...` → PASS.
- [ ] **Step 5:** Commit `feat(server): noindex for private routes and API, real 404s` + push.

### Task 2: File statis — robots, llms, JSON-LD data, metadata

**Files:**
- Modify: `web/public/robots.txt` (persis spec §7)
- Create: `web/public/llms.txt` (spec §7), `web/src/seo/meta.ts`
- Test: `web/src/seo/meta.test.ts`

**Interfaces:**
- Produces: `type PublicRoute = "/" | "/privacy" | "/terms"`; `pageMeta: Record<PublicRoute, { title: string; description: string; canonical: string }>` (nilai persis spec §5); `jsonLd(): object` (objek persis spec §5); `headTags(route: PublicRoute): string` — string HTML berisi `<title>`, `meta description`, `link canonical`, `og:title`, `og:description`, `og:url`, dan untuk `/` blok `<script type="application/ld+json">`; semua nilai di-escape HTML (`&`, `<`, `>`, `"`); JSON-LD di-escape `<` → `<`.

- [ ] **Step 1: Test gagal:** `headTags("/privacy")` memuat `<link rel="canonical" href="https://chat.rahmattomy.com/privacy"/>` dan title "Privacy Policy — natter"; `headTags("/")` memuat tepat satu `application/ld+json` yang `JSON.parse` menghasilkan `@type === "SoftwareApplication"`, `offers.price === "0"`, tanpa `aggregateRating`; `headTags("/terms")` tidak memuat ld+json; nilai dengan `"` ter-escape.
- [ ] **Step 2:** `cd web && npx vitest run src/seo` → FAIL.
- [ ] **Step 3:** Implementasi; isi `llms.txt` hanya fakta dari spec §7 (fitur yang ada, privasi jujur, teknologi, link).
- [ ] **Step 4:** `npm run lint && npx vitest run` → PASS.
- [ ] **Step 5:** Commit `feat(web): robots policy, llms.txt and per-page SEO metadata` + push.

### Task 3: Prerender + hydrate

**Files:**
- Create: `web/src/entry-prerender.tsx`, `web/scripts/prerender.mjs`, `web/src/hooks/useDocumentTitle.ts`
- Modify: `web/index.html` (ganti blok title/description/canonical/og:title/og:description/og:url dengan penanda `<!--head-->`; tag lain tetap), `web/package.json` (`build` = `tsc -b && vite build && vite build --ssr src/entry-prerender.tsx --outDir dist-ssr && node scripts/prerender.mjs`), `web/src/main.tsx`, komponen yang mengakses `window`/`document`/`localStorage` saat render (ThemeToggle, LanguageToggle, Landing, LegalPage — pindahkan ke `useEffect` atau guard), `web/src/pages/Privacy.tsx`, `Terms.tsx`, `Landing.tsx` (pakai `useDocumentTitle`), `.github/workflows/ci.yml` dan `Dockerfile` bila perlu (build sudah lewat `npm run build`), `web/.gitignore` (`dist-ssr`)
- Test: `web/src/entry-prerender.test.tsx`, `web/e2e/seo.spec.ts`

**Interfaces:**
- Consumes: `PublicRoute`, `headTags`, `pageMeta` (Task 2).
- Produces: `render(route: PublicRoute): string` di `entry-prerender.tsx` (i18n instance `lng: "en"`, `StaticRouter location={route}`); `prerender.mjs` menulis `dist/index.html`, `dist/privacy/index.html`, `dist/terms/index.html`, `dist/sitemap.xml` (3 `<url>` dengan `<loc>` absolut dan `<lastmod>` YYYY-MM-DD), lalu menghapus `dist-ssr`. `useDocumentTitle(title: string): void`.

- [ ] **Step 1: Test gagal:**
  - Vitest: `render("/")` memuat `Conversations, made simple.` dan `Continue as guest`; `render("/privacy")` memuat judul Privacy Policy; `render` tidak melempar (tanpa `window`/`localStorage` — jalankan dengan environment `node`).
  - Playwright `seo.spec.ts` (server produksi lokal): konteks `javaScriptEnabled: false` → `/` punya h1 "Conversations, made simple." dan link "Sign in with Google"; `/privacy` punya h1 Privacy dan canonical `/privacy`; konteks dengan JS: tidak ada pesan console berisi "hydrat" di `/`, `/privacy`, `/terms` (EN) dan `no hydration warnings with stored id` (localStorage `natter.lang=id` → setelah load h1 "Ngobrol, jadi sederhana."); `/sitemap.xml` ter-parse dan berisi 3 URL; `/robots.txt` memuat `Sitemap: https://chat.rahmattomy.com/sitemap.xml`.
- [ ] **Step 2:** Jalankan → FAIL.
- [ ] **Step 3:** Implementasi. `main.tsx`: `const pre = root.hasChildNodes() && ["/", "/privacy", "/terms"].includes(location.pathname.replace(/\/$/, "") || "/")`; bila `pre` → `hydrateRoot` dengan i18n EN, lalu `i18n.changeLanguage(stored)` setelah render bila tersimpan ID; selain itu `createRoot`.
- [ ] **Step 4:** `npm run lint && npx vitest run && npm run build` → PASS; isi `dist/index.html` memuat h1 (grep); E2E penuh lokal (server seperti job CI e2e) → PASS.
- [ ] **Step 5:** Commit `feat(web): prerender public pages with per-page metadata and sitemap` + push.

### Task 4: Validasi & laporan

**Files:**
- Create: `docs/seo.md` (EN, ringkas: kebijakan crawler dan alasannya, cara prerender bekerja, cara memvalidasi)
- Modify: `docs/operations.md` & `docs/operations.id.md` (satu paragraf: daftarkan sitemap di Google Search Console/Bing Webmaster — langkah manual)

- [ ] **Step 1:** Build produksi + server Go lokal; `curl -sI` dan `curl -s` untuk `/`, `/privacy`, `/terms`, `/chat`, `/settings`, `/onboarding`, `/nope`, `/robots.txt`, `/sitemap.xml`, `/llms.txt`, `/og.png`, `/api/me`, `/api/conversations` — catat status, content-type, `X-Robots-Tag`, canonical, ada/tidaknya h1 di HTML awal.
- [ ] **Step 2:** Validasi sitemap dengan `xmllint --noout` (atau parser bila xmllint tidak ada); ekstrak JSON-LD dan `JSON.parse`; cek OG per halaman.
- [ ] **Step 3:** Tulis `docs/seo.md` dan paragraf operations; sertakan tabel hasil di laporan task (tidak di repo).
- [ ] **Step 4:** Commit `docs: SEO and crawler policy` + push.
