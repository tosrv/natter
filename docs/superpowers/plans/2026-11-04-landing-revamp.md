# Landing Revamp Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Landing page baru (9 section, produk sebagai pusat visual) dan token visual netral + oranye + Geist untuk seluruh app, dengan tema Light/Dark/System, SEO/OG, dan performa yang lebih baik.

**Architecture:** Token shadcn di `web/src/index.css` diganti nilainya (nama tetap) sehingga seluruh app ikut berubah. Landing dipecah menjadi komponen per section di `web/src/pages/landing/`; logika auth/tamu yang ada dipertahankan. Screenshot produk dibuat otomatis oleh Playwright dari app dengan data demo, lalu dioptimalkan ke AVIF/WebP.

**Tech Stack:** React 18, Vite 5, Tailwind 3, shadcn/ui (Radix), react-i18next, Vitest, Playwright; baru: ESLint 9 (`typescript-eslint`, `eslint-plugin-react-hooks`, `eslint-plugin-jsx-a11y`), `sharp` (devDependency, hanya skrip optimasi gambar).

**Spec:** `docs/superpowers/specs/2026-11-04-landing-revamp-design.md`

## Global Constraints

- Nilai token persis seperti tabel spec §3; nama variabel CSS tidak berubah; tambah `--surface`.
- Font: Geist dari Google Fonts `family=Geist:wght@400;500;600&display=swap`; CSP (`internal/api/security.go`) **tidak** diubah.
- Tema: key `localStorage["natter.theme"]`, nilai `light|dark|system`; `public/theme-init.js` dimuat sinkron di `<head>` sebelum bundle.
- Teks persis tabel spec §6 (EN/ID); "Natter is Matter." tidak diterjemahkan; hanya fitur yang ada; tidak ada klaim keamanan selain yang di spec.
- Gambar di-host sendiri di `web/public/`; hero ≤ 200 KB total; `width`/`height` eksplisit.
- Perilaku auth tidak berubah: redirect user login, `getConfig` Turnstile, `GuestStartDialog`, `/api/auth/google/login`.
- Widget (`widget/`) tidak diubah.
- Setiap task: commit + push ke `feat/landing-revamp`; PR yang mengubah tampilan menyertakan screenshot (landing, chat, settings × desktop/HP × terang/gelap) dari `SCREENSHOTS=1 npx playwright test screenshots`.

## Review Focus

1. `localStorage` diblokir / nilai rusak → tema jatuh ke `system` tanpa error, tanpa kedipan (Task 2: test `theme-init falls back to system on bad value`).
2. User memilih **System** lalu OS berganti tema → app mengikuti tanpa reload (Task 2: `ThemeSwitch system follows matchMedia change`).
3. User yang sudah login membuka `/` → tetap diarahkan ke `/chat` atau `/onboarding` sebelum landing terasa (Task 4: E2E redirect yang ada tetap hijau).
4. Bahasa ID dipilih → semua teks landing berubah, termasuk `<html lang="id">` (Task 4: Vitest render ID + assertion `document.documentElement.lang`).
5. Gambar produk gagal dimuat / tema gelap → layout tidak bergeser (dimensi tetap) dan screenshot yang tampil sesuai tema (Task 5: E2E `dark theme shows dark product shot`).

---

### Task 1: ESLint + lazy routes

**Files:**
- Create: `web/eslint.config.js`
- Modify: `web/package.json` (devDeps + script `"lint": "eslint ."`), `web/src/App.tsx`, `.github/workflows/ci.yml` (job `web-test`: `npm run lint` sebelum vitest)
- Test: `web/src/App.test.tsx`

**Interfaces:**
- Produces: `npm run lint` (0 error); route `/chat`, `/settings`, `/onboarding`, `/privacy`, `/terms` lewat `React.lazy` dengan fallback `<p className="p-6 text-muted-foreground">{t("common.loading")}</p>`.

- [ ] **Step 1:** Test gagal: render `<App/>` di `/` (MemoryRouter tidak bisa karena App memakai BrowserRouter → set `window.history.pushState({}, "", "/")`) → landing tampil; di `/terms` → setelah `await findByRole("heading")` halaman Terms tampil (membuktikan Suspense bekerja).
- [ ] **Step 2:** `npx vitest run src/App.test.tsx` → FAIL (file baru belum lazy / test belum ada assertion lazy — pastikan gagal karena import).
- [ ] **Step 3:** Pasang ESLint (flat config: `@eslint/js` recommended, `typescript-eslint` recommended, `react-hooks` recommended, `jsx-a11y` recommended; abaikan `dist`, `e2e/test-results`); perbaiki semua error yang muncul **tanpa** mengubah perilaku (hapus komentar `eslint-disable` yang tak lagi relevan atau biarkan bila rule-nya ada). Ubah App ke lazy routes.
- [ ] **Step 4:** `npm run lint && npx vitest run && npm run build` → PASS; catat ukuran chunk landing vs sebelum di report.
- [ ] **Step 5:** Commit `chore(web): eslint and lazy routes` + push.

### Task 2: Token, font, tema 3-mode

**Files:**
- Modify: `web/src/index.css`, `web/tailwind.config.js` (font `Geist`, warna `surface`), `web/index.html` (font link, script `theme-init.js`), `web/src/components/ui/button.tsx` (tidak ada lagi `rounded-full` default — cek pemakaian), `web/src/pages/Settings.tsx` (pakai `ThemeSwitch`)
- Create: `web/public/theme-init.js`, `web/src/components/ThemeSwitch.tsx`
- Delete: `web/src/components/ThemeToggle.tsx` (setelah semua pemakai pindah)
- Test: `web/src/components/ThemeSwitch.test.tsx`, `web/src/lib/theme.test.ts`

**Interfaces:**
- Consumes: `getThemePref`, `setThemePref`, `applyTheme` dari `web/src/lib/theme.ts`.
- Produces: `ThemeSwitch({ className?: string })` — radiogroup 3 tombol (`role="radio"`, `aria-checked`, label i18n `theme.light|dark|system`, ikon Sun/Moon/Monitor), panah kiri/kanan berpindah pilihan. `theme-init.js`: membaca key, bila `dark` atau (`system`/tak valid + `matchMedia` gelap) → tambah kelas `dark` pada `<html>`; semua dalam `try/catch`.

- [ ] **Step 1:** Test gagal: klik "Dark" → `localStorage["natter.theme"]==="dark"` dan `<html>` punya `dark`; klik "System" dengan `matchMedia` palsu gelap → `dark`, lalu event `change` ke terang → kelas hilang (`ThemeSwitch system follows matchMedia change`); panah kanan dari Light → Dark terpilih; `theme-init falls back to system on bad value` (eksekusi isi file dengan storage berisi `"blue"` dan dengan storage melempar error → tidak throw, mengikuti `matchMedia`).
- [ ] **Step 2:** `npx vitest run` → FAIL.
- [ ] **Step 3:** Implementasi; ganti token sesuai spec §3; ganti semua `ThemeToggle` dengan `ThemeSwitch`; hitung kontras putih vs `--primary` light dan dark (tulis angkanya di report) dan terapkan aturan spec §3 bila < 4.5.
- [ ] **Step 4:** `npm run lint && npx vitest run && npm run build` → PASS; jalankan E2E penuh lokal (server seperti job CI e2e) → PASS; ambil screenshot sebelum/sesudah (landing lama, chat, settings) untuk PR.
- [ ] **Step 5:** Commit `feat(web): neutral tokens, Geist, three-way theme switch` + push.

### Task 3: Brand assets, metadata SEO

**Files:**
- Create: `web/public/brand/natter-wordmark.svg`, `web/public/brand/natter-mark.svg`, `web/public/favicon.svg`, `web/public/favicon-32.png`, `web/public/apple-touch-icon.png`, `web/public/icon-512.png`, `web/public/site.webmanifest`, `web/public/robots.txt`, `web/src/components/Wordmark.tsx`
- Modify: `web/index.html`, `web/src/i18n/index.ts` (set `document.documentElement.lang` saat bahasa berubah)
- Test: `web/src/components/Wordmark.test.tsx`, `web/src/i18n/index.test.ts`

**Interfaces:**
- Produces: `Wordmark({ className?: string })` — inline SVG (mark oranye `fill="hsl(var(--primary))"` + teks "natter" sebagai path `fill="currentColor"`), `role="img"`, `aria-label="natter"`. PNG ikon dibuat dari SVG via skrip `web/scripts/icons.mjs` (sharp).
- `index.html`: title, description, canonical, `og:*` (image `/og.png` — file dibuat di Task 5), `twitter:card`, dua `theme-color` (light `#FFFFFF`, dark `#09090B` dengan `media`), favicon, manifest.

- [ ] **Step 1:** Test gagal: `Wordmark` punya `aria-label="natter"`; ganti bahasa ke `id` → `document.documentElement.lang === "id"`.
- [ ] **Step 2:** `npx vitest run` → FAIL.
- [ ] **Step 3:** Implementasi. `robots.txt` persis spec §8.
- [ ] **Step 4:** `npm run lint && npx vitest run && npm run build` → PASS; `curl -s localhost:4173/ | grep -c 'og:title'` = 1 pada `vite preview`.
- [ ] **Step 5:** Commit `feat(web): wordmark, icons, SEO metadata` + push.

### Task 4: Landing sections + teks EN/ID

**Files:**
- Create: `web/src/pages/landing/{Nav,Hero,ProductIntro,Features,HowItWorks,Philosophy,Privacy,FinalCta,Footer}.tsx`
- Modify: `web/src/pages/Landing.tsx` (merangkai + logika auth yang ada), `web/src/i18n/en.json`, `web/src/i18n/id.json` (kunci `landing.*` baru persis spec §6; hapus yang lama tak terpakai; hapus FAQ), `web/e2e/public.spec.ts`, `web/e2e/revamp.spec.ts`, `web/e2e/guest.ts`
- Test: `web/src/pages/Landing.test.tsx`

**Interfaces:**
- Consumes: `ThemeSwitch` (Task 2), `Wordmark` (Task 3), `LanguageToggle`, `GuestStartDialog`, `Button`.
- Produces: `Hero` menerima `{ onGoogle: () => void; onGuest: () => void; guestDisabled: boolean; productShot: ReactNode }`; Task 5 mengisi `productShot` (sementara: placeholder `<div>` dengan rasio 16:10 dan border, tanpa teks palsu). Anchor id section: `product`, `how`, `privacy`. Skip link `href="#main"`, `<main id="main">`.

- [ ] **Step 1:** Test gagal: render EN → satu `h1` "Conversations, made simple.", tombol "Sign in with Google" dan "Continue as guest", teks "Natter is Matter.", "Less noise. More conversation.", "Not end-to-end encrypted." ada; render ID → "Ngobrol, jadi sederhana." dan "Lanjut sebagai tamu"; tidak ada teks "FAQ"/"Frequently"; klik "Continue as guest" saat `guestDisabled=false` membuka dialog tamu.
- [ ] **Step 2:** `npx vitest run src/pages/Landing.test.tsx` → FAIL.
- [ ] **Step 3:** Implementasi sesuai spec §5 (tata letak draf A); nav mobile memakai DropdownMenu; sesuaikan E2E dengan label baru (helper `guest.ts` memakai "Continue as guest").
- [ ] **Step 4:** `npm run lint && npx vitest run && npm run build` → PASS; E2E penuh lokal → PASS.
- [ ] **Step 5:** Commit `feat(web): new landing sections and copy` + push.

### Task 5: Screenshot produk + OG image

**Files:**
- Modify: `web/e2e/screenshots.spec.ts` (mode `PRODUCT_SHOTS=1`: seed data demo via fake login — 3 pengguna fiktif "Dimas Pratama", "Alya", "Raka", percakapan wajar dengan ✓ dan ✓✓, indikator mengetik; tangkap desktop 1440×900 crop area chat dan HP 390×844, terang & gelap, `deviceScaleFactor: 2`)
- Create: `web/scripts/optimize-images.mjs` (sharp → AVIF q50 + WebP q75 + PNG, lebar 1x/2x), `web/public/product/*`, `web/public/og.png` (1200×630, dirender Playwright dari `web/scripts/og.html` lalu dioptimalkan), `web/src/pages/landing/ProductShot.tsx`
- Modify: `Hero.tsx`, `Features.tsx`
- Test: `web/src/pages/landing/ProductShot.test.tsx`, E2E di `web/e2e/revamp.spec.ts`

**Interfaces:**
- Produces: `ProductShot({ variant: "desktop" | "mobile" | "receipts", priority?: boolean })` — dua `<picture>` (light dengan `dark:hidden`, dark dengan `hidden dark:block`), masing-masing `<source type="image/avif">`, `<source type="image/webp">`, `<img>` PNG dengan `width`, `height`, `alt` deskriptif, `loading={priority ? "eager" : "lazy"}`, `fetchpriority={priority ? "high" : undefined}`.

- [ ] **Step 1:** Test gagal: `ProductShot variant="desktop" priority` → `img` punya `width`/`height`, `loading="eager"`, `fetchpriority="high"`, dua picture (light/dark). E2E `dark theme shows dark product shot`: set tema Dark lewat `ThemeSwitch` → gambar dark terlihat, light tersembunyi; `boundingBox` hero tidak berubah sebelum/sesudah gambar load.
- [ ] **Step 2:** Jalankan → FAIL.
- [ ] **Step 3:** Buat screenshot dan aset; pasang di Hero (desktop di ≥ md, mobile di < md) dan Features (crop receipts/typing); total berat hero ≤ 200 KB (tulis angkanya di report).
- [ ] **Step 4:** `npm run lint && npx vitest run && npm run build` → PASS; E2E penuh → PASS.
- [ ] **Step 5:** Commit `feat(web): real product screenshots and OG image` + push.

### Task 6: Motion, aksesibilitas, verifikasi akhir

**Files:**
- Modify: komponen landing (motion), `web/src/index.css` (utility `reveal` di bawah `prefers-reduced-motion: no-preference`), `web/e2e/revamp.spec.ts`
- Create: `web/src/hooks/useReveal.ts`

**Interfaces:**
- Produces: `useReveal<T extends Element>(): React.RefObject<T>` — IntersectionObserver menambah kelas `is-visible` sekali; tanpa IntersectionObserver langsung visible.

- [ ] **Step 1:** Test gagal: Vitest `useReveal` (tanpa IntersectionObserver → elemen langsung `is-visible`); E2E: tema dipilih Dark → reload → masih Dark; keyboard: Tab dari awal mencapai skip link, lalu "Sign in", lalu ThemeSwitch, lalu "Sign in with Google", lalu "Continue as guest" dengan fokus terlihat (`toBeFocused`).
- [ ] **Step 2:** Jalankan → FAIL.
- [ ] **Step 3:** Implementasi motion (opacity/translate ≤ 8px, 200–300ms) hanya pada blok product shot dan judul section.
- [ ] **Step 4:** `npm run lint && npx vitest run && npm run build` → PASS; E2E penuh → PASS; Lighthouse lokal pada `vite preview` landing mobile: Accessibility, SEO, Best Practices ≥ 95 (angka di report); screenshot akhir semua halaman.
- [ ] **Step 5:** Commit `feat(web): subtle motion and accessibility pass` + push.
