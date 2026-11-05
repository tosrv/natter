import { test, expect, type Browser } from "@playwright/test";

test("landing is prerendered (no JS)", async ({ browser }) => {
  const ctx = await browser.newContext({ javaScriptEnabled: false });
  const page = await ctx.newPage();
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1, name: "Conversations, made simple." })).toBeVisible();
  await expect(page.getByRole("link", { name: /Sign in with Google/ }).first()).toBeVisible();
  await ctx.close();
});

test("privacy is prerendered with its own canonical (no JS)", async ({ browser }) => {
  const ctx = await browser.newContext({ javaScriptEnabled: false });
  const page = await ctx.newPage();
  await page.goto("/privacy");
  await expect(page.getByRole("heading", { level: 1, name: /Privacy/ })).toBeVisible();
  await expect(page.locator('link[rel="canonical"]')).toHaveAttribute("href", "https://chat.rahmattomy.com/privacy");
  await ctx.close();
});

async function hydrationMessages(browser: Browser, path: string, lang?: string) {
  const ctx = await browser.newContext();
  if (lang) await ctx.addInitScript((l) => localStorage.setItem("natter.lang", l), lang);
  const page = await ctx.newPage();
  const msgs: string[] = [];
  page.on("console", (m) => { if (/hydrat/i.test(m.text())) msgs.push(m.text()); });
  page.on("pageerror", (e) => { if (/hydrat|#418|#423|#425/i.test(e.message)) msgs.push(e.message); });
  await page.goto(path);
  return { ctx, page, msgs };
}

for (const path of ["/", "/privacy", "/terms"]) {
  test(`no hydration warnings on ${path}`, async ({ browser }) => {
    const { ctx, page, msgs } = await hydrationMessages(browser, path);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await page.waitForLoadState("networkidle");
    expect(msgs).toEqual([]);
    await ctx.close();
  });
}

test("no hydration warnings with stored id", async ({ browser }) => {
  const { ctx, page, msgs } = await hydrationMessages(browser, "/", "id");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Ngobrol, jadi sederhana.");
  await page.waitForLoadState("networkidle");
  expect(msgs).toEqual([]);
  await ctx.close();
});

test("sitemap.xml lists the three public URLs", async ({ request }) => {
  const res = await request.get("/sitemap.xml");
  expect(res.ok()).toBeTruthy();
  const xml = await res.text();
  expect(xml).toMatch(/^<\?xml/);
  const locs = [...xml.matchAll(/<loc>([^<]+)<\/loc>/g)].map((m) => m[1]);
  expect(locs).toEqual(["https://chat.rahmattomy.com/", "https://chat.rahmattomy.com/privacy", "https://chat.rahmattomy.com/terms"]);
  expect([...xml.matchAll(/<lastmod>\d{4}-\d{2}-\d{2}<\/lastmod>/g)].length).toBe(3);
});

test("robots.txt points at the sitemap", async ({ request }) => {
  const txt = await (await request.get("/robots.txt")).text();
  expect(txt).toContain("Sitemap: https://chat.rahmattomy.com/sitemap.xml");
});
