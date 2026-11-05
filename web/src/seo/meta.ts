export type PublicRoute = "/" | "/privacy" | "/terms";

const ORIGIN = "https://chat.rahmattomy.com";

export const pageMeta: Record<PublicRoute, { title: string; description: string; canonical: string }> = {
  "/": {
    title: "natter — Conversations, made simple.",
    description:
      "natter is a simple personal 1:1 messenger. Sign in with Google or continue as a guest — no phone number needed.",
    canonical: `${ORIGIN}/`,
  },
  "/privacy": {
    title: "Privacy Policy — natter",
    description: "How natter collects, uses and deletes your data.",
    canonical: `${ORIGIN}/privacy`,
  },
  "/terms": {
    title: "Terms of Service — natter",
    description: "The short terms for using natter.",
    canonical: `${ORIGIN}/terms`,
  },
};

export function jsonLd(): object {
  return {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    name: "natter",
    url: `${ORIGIN}/`,
    description: "A simple personal 1:1 real-time messenger. Natter is Matter.",
    applicationCategory: "CommunicationApplication",
    operatingSystem: "Web",
    inLanguage: ["en", "id"],
    isAccessibleForFree: true,
    offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
    license: "https://opensource.org/licenses/MIT",
    author: { "@type": "Person", name: "Rahmat Tomy" },
    sameAs: ["https://github.com/tosrv/natter"],
  };
}

function esc(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

export function headTags(route: PublicRoute): string {
  const m = pageMeta[route];
  const tags = [
    `<title>${esc(m.title)}</title>`,
    `<meta name="description" content="${esc(m.description)}"/>`,
    `<link rel="canonical" href="${esc(m.canonical)}"/>`,
    `<meta property="og:title" content="${esc(m.title)}"/>`,
    `<meta property="og:description" content="${esc(m.description)}"/>`,
    `<meta property="og:url" content="${esc(m.canonical)}"/>`,
  ];
  if (route === "/") {
    const json = JSON.stringify(jsonLd()).replace(/</g, "\\u003c");
    tags.push(`<script type="application/ld+json">${json}</script>`);
  }
  return tags.join("\n");
}
