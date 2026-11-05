// Post-build: renders the public routes into dist/ with per-page <head> tags and writes sitemap.xml.
// Runs after `vite build --ssr src/entry-prerender.tsx --outDir dist-ssr`; removes dist-ssr when done.
import { mkdirSync, readFileSync, rmSync, writeFileSync, readdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const web = join(dirname(fileURLToPath(import.meta.url)), "..");
const dist = join(web, "dist");
const ssrDir = join(web, "dist-ssr");
const entry = readdirSync(ssrDir).find((f) => /^entry-prerender\.(m?js)$/.test(f));
if (!entry) throw new Error("dist-ssr/entry-prerender.js not found");
const { render, headTags, pageMeta } = await import(pathToFileURL(join(ssrDir, entry)).href);

const template = readFileSync(join(dist, "index.html"), "utf8");
if (!template.includes("<!--head-->") || !template.includes('<div id="root"></div>')) {
  throw new Error("dist/index.html is missing <!--head--> or <div id=\"root\"></div>");
}

// Clean SPA shell for private routes and 404s: empty root, neutral head (no canonical, og:url or JSON-LD).
const esc = (v) => v.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
writeFileSync(join(dist, "app.html"), template.replace("<!--head-->", () =>
  `<title>natter</title>\n<meta name="description" content="${esc(pageMeta["/"].description)}"/>`));
console.log("wrote dist/app.html");

const routes = [["/", "index.html"], ["/privacy", "privacy/index.html"], ["/terms", "terms/index.html"]];
for (const [route, file] of routes) {
  const html = template
    .replace("<!--head-->", () => headTags(route))
    .replace('<div id="root"></div>', () => `<div id="root">${render(route)}</div>`);
  if (!/<h1[\s>]/.test(html)) throw new Error(`prerendered ${route} has no <h1>`);
  mkdirSync(dirname(join(dist, file)), { recursive: true });
  writeFileSync(join(dist, file), html);
  console.log(`prerendered ${route} -> dist/${file}`);
}

const lastmod = new Date().toISOString().slice(0, 10);
const urls = routes.map(([route]) => `  <url>\n    <loc>${pageMeta[route].canonical}</loc>\n    <lastmod>${lastmod}</lastmod>\n  </url>`);
writeFileSync(join(dist, "sitemap.xml"),
  `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${urls.join("\n")}\n</urlset>\n`);
console.log("wrote dist/sitemap.xml");

rmSync(ssrDir, { recursive: true, force: true });
