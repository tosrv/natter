import { describe, expect, it } from "vitest";
import { headTags, jsonLd, pageMeta } from "./meta";

function ldBlocks(html: string): string[] {
  return [...html.matchAll(/<script type="application\/ld\+json">([\s\S]*?)<\/script>/g)].map((m) => m[1]);
}

describe("seo meta", () => {
  it("privacy has canonical and title", () => {
    const h = headTags("/privacy");
    expect(h).toContain('<link rel="canonical" href="https://chat.rahmattomy.com/privacy"/>');
    expect(h).toContain("<title>Privacy Policy — natter</title>");
    expect(h).toContain('property="og:url"');
    expect(h).toContain('property="og:title"');
    expect(h).toContain('property="og:description"');
    expect(h).toContain('name="description"');
  });

  it("landing has exactly one SoftwareApplication JSON-LD", () => {
    const blocks = ldBlocks(headTags("/"));
    expect(blocks).toHaveLength(1);
    const data = JSON.parse(blocks[0]);
    expect(data["@type"]).toBe("SoftwareApplication");
    expect(data.offers.price).toBe("0");
    expect(data.aggregateRating).toBeUndefined();
    expect(jsonLd()).toMatchObject({ name: "natter" });
  });

  it("terms has no JSON-LD", () => {
    expect(headTags("/terms")).not.toContain("ld+json");
  });

  it("escapes HTML in values", () => {
    const orig = pageMeta["/terms"].description;
    pageMeta["/terms"].description = 'a "b" <c> & d';
    try {
      const h = headTags("/terms");
      expect(h).toContain("a &quot;b&quot; &lt;c&gt; &amp; d");
      expect(h).not.toContain('"b"');
    } finally {
      pageMeta["/terms"].description = orig;
    }
  });
});
