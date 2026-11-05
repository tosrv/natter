# SEO and crawler policy

Public pages (`/`, `/privacy`, `/terms`) are indexable by search engines and AI crawlers. Everything private is not.

## Crawler policy and why

- `web/public/robots.txt` allows Googlebot, Bingbot, AI search bots (OAI-SearchBot, PerplexityBot) and AI training bots (GPTBot, ClaudeBot) on public pages, and disallows `/api/` for everyone. The public pages are marketing and legal text only; no user data is public, so there is nothing to protect from training crawlers.
- `robots.txt` is advisory. The real protection is server-side: every `/api/*` route needs a session (401 otherwise), and the server adds `X-Robots-Tag: noindex, nofollow` to `/api/*`, `/chat`, `/settings` and `/onboarding`.
- Unknown paths return a real 404 (with `X-Robots-Tag: noindex`), not a soft 200.
- `web/public/llms.txt` is a short Markdown summary for LLM tools. `sitemap.xml` is generated at build time and lists the three public URLs.

## How prerender works

`npm run build` in `web/` runs `vite build`, an SSR build of `src/entry-prerender.tsx`, then `scripts/prerender.mjs`:

1. Writes `dist/app.html`: a clean SPA shell (empty `#root`, no canonical, no JSON-LD). The server uses it for `/chat`, `/settings`, `/onboarding` and 404 bodies.
2. Renders each public route with `react-dom/server` (English, default theme) into `dist/index.html`, `dist/privacy/index.html` and `dist/terms/index.html`, with per-page title, description, canonical and Open Graph tags. `/` also gets `SoftwareApplication` JSON-LD (free, no rating).
3. Writes `dist/sitemap.xml`.

In the browser, `main.tsx` hydrates the prerendered public routes and mounts a normal SPA elsewhere. A stored Indonesian language preference is applied after hydration to avoid a mismatch.

## How to validate

Build, then serve with the Go server (`STATIC_DIR=web/dist`, copy `widget/dist/widget.js` into `web/dist`) and check:

```sh
curl -sI localhost:8080/chat            # X-Robots-Tag: noindex, nofollow
curl -s  localhost:8080/ | grep -c '<h1'   # >= 1 without JavaScript
curl -s  localhost:8080/ | grep canonical
curl -s  localhost:8080/sitemap.xml | xmllint --noout -
curl -s  localhost:8080/robots.txt
curl -si localhost:8080/nope | head -1  # 404
curl -si localhost:8080/api/me | head -1   # 401 + X-Robots-Tag
```

Also parse the `application/ld+json` block on `/` with `JSON.parse`. Go tests in `internal/api/seo_test.go` cover the server behaviour.
