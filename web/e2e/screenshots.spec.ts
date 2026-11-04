import { test, expect, type Browser, type Page } from "@playwright/test";
import path from "node:path";

// Generates docs/screenshots/*.png. Skipped unless SCREENSHOTS=1:
//   SCREENSHOTS=1 E2E_BASE_URL=http://localhost:8099 npx playwright test screenshots

const OUT = path.resolve(process.cwd(), "../docs/screenshots"); // run from web/
const run = Date.now().toString(36).slice(-6);
const VIEWPORTS = { desktop: { width: 1280, height: 800 }, mobile: { width: 390, height: 844 } } as const;
const THEMES = ["light", "dark"] as const;

async function user(browser: Browser, sub: string, username: string, display_name: string) {
  const ctx = await browser.newContext();
  const res = await ctx.request.post("/api/test/login", { data: { sub, email: `${sub}@e2e.test`, username, display_name } });
  expect(res.ok()).toBeTruthy();
  const me = await (await ctx.request.get("/api/me")).json();
  expect(me.username, "username taken? run PRODUCT_SHOTS against a fresh DB").toBe(username);
  return ctx;
}

async function send(p: Page, body: string) {
  await p.getByPlaceholder("Write a message…").fill(body);
  await p.getByRole("button", { name: "Send", exact: true }).click();
  await expect(p.locator("p.whitespace-pre-wrap", { hasText: body }).last()).toBeVisible();
}

test("screenshots", async ({ browser }) => {
  test.skip(!process.env.SCREENSHOTS, "set SCREENSHOTS=1 to regenerate screenshots");
  test.setTimeout(180_000);
  const rina = `rina_${run}`, dimas = `dimas_${run}`;
  const rctx = await user(browser, `shot-r-${run}`, rina, "Rina Putri");
  const dctx = await user(browser, `shot-d-${run}`, dimas, "Dimas Pratama");
  const r = await rctx.newPage(), d = await dctx.newPage();
  await r.goto("/chat"); await d.goto("/chat");

  await r.getByPlaceholder("Search username…").fill(dimas);
  await r.getByRole("button", { name: new RegExp(`@${dimas}`) }).click();
  await send(r, "Hi Dimas! Are we still on for coffee tomorrow?");
  await d.getByRole("button", { name: new RegExp(`@${rina}`) }).click();
  await send(d, "Yes! 10am at the usual place?");
  await expect(r.getByRole("img", { name: "Read", exact: true }).first()).toBeVisible();
  await d.close(); // Dimas leaves: later messages stay "sent", the first one is read.
  await send(r, "Perfect, see you there ☕");
  await send(r, "I'll bring the book I mentioned.");

  for (const theme of THEMES) {
    for (const [vp, size] of Object.entries(VIEWPORTS)) {
      const ctx = await browser.newContext({ viewport: size, storageState: await rctx.storageState() });
      await ctx.addInitScript((t) => { try { localStorage.setItem("natter.theme", t); localStorage.setItem("natter.lang", "en"); } catch { /* */ } }, theme);
      const p = await ctx.newPage();
      const shot = (name: string) => p.screenshot({ path: path.join(OUT, `${name}-${theme}-${vp}.png`) });

      const anon = await browser.newContext({ viewport: size });
      await anon.addInitScript((t) => { try { localStorage.setItem("natter.theme", t); localStorage.setItem("natter.lang", "en"); } catch { /* */ } }, theme);
      const lp = await anon.newPage();
      await lp.goto("/");
      await expect(lp.getByRole("heading", { level: 1, name: "Conversations, made simple." })).toBeVisible();
      await expect(lp.getByRole("button", { name: "Continue as guest" }).first()).toBeVisible();
      await lp.waitForTimeout(300);
      await lp.screenshot({ path: path.join(OUT, `landing-${theme}-${vp}.png`) });
      await anon.close();

      await p.goto("/chat");
      await p.getByRole("button", { name: new RegExp(`@${dimas}`) }).click();
      await expect(p.locator("p.whitespace-pre-wrap", { hasText: "book" })).toBeVisible();
      await p.waitForTimeout(300);
      await shot("chat");

      await p.goto("/settings");
      await expect(p.getByRole("heading", { name: "Settings" })).toBeVisible();
      await p.waitForTimeout(300);
      await shot("settings");
      await ctx.close();
    }
  }
  await rctx.close(); await dctx.close();
});

// Landing product shots (raw PNGs → web/shots-raw, then `node scripts/optimize-images.mjs`):
//   PRODUCT_SHOTS=1 E2E_BASE_URL=http://localhost:8099 npx playwright test screenshots -g product
test.describe("product", () => {
  test.skip(!process.env.PRODUCT_SHOTS, "set PRODUCT_SHOTS=1 to regenerate landing product shots");
  test("product shots", async ({ browser }) => {
    test.setTimeout(240_000);
    const RAW = path.resolve(process.cwd(), "shots-raw");
    const dimas = "dimas", alya = "alya", raka = "raka"; // clean names: run against a fresh DB
    const dctx = await user(browser, `ps-d-${run}`, dimas, "Dimas Pratama");
    const actx = await user(browser, `ps-a-${run}`, alya, "Alya");
    const kctx = await user(browser, `ps-k-${run}`, raka, "Raka");
    const d = await dctx.newPage(), a = await actx.newPage(), k = await kctx.newPage();
    await d.goto("/chat"); await a.goto("/chat"); await k.goto("/chat");

    // Raka ↔ Dimas: an older, short thread for the conversation list.
    await k.getByPlaceholder("Search username…").fill(dimas);
    await k.getByRole("button", { name: new RegExp(`@${dimas}`) }).click();
    await send(k, "Slides for Thursday are in the shared folder.");
    await d.getByRole("button", { name: new RegExp(`@${raka}`) }).click();
    await send(d, "Thanks Raka, I'll review them tonight.");
    await k.close();

    // Alya ↔ Dimas: the featured thread.
    await d.getByPlaceholder("Search username…").fill(alya);
    await d.getByRole("button", { name: new RegExp(`@${alya}`) }).click();
    await send(d, "Morning! Did the train get you in on time?");
    await a.getByRole("button", { name: new RegExp(`@${dimas}`) }).click();
    await send(a, "Just barely. Grabbing a coffee before the 9:30 standup.");
    await send(a, "Want one?");
    await send(d, "Yes please, oat latte if they have it.");
    await send(d, "I'll save us seats near the window.");
    await expect(d.getByRole("img", { name: "Read", exact: true }).last()).toBeVisible();
    await a.goto("/settings"); // Alya steps away: the next message stays at a single tick.
    await send(d, "Also, the demo moved to 2pm.");
    await expect(d.getByRole("img", { name: "Sent", exact: true }).last()).toBeVisible();
    const convId: string = await a.evaluate(async () => {
      const r = await fetch("/api/conversations"); const cs = await r.json(); return cs[0].id;
    });
    // Keep Alya "typing" via a raw socket so the indicator is visible in every capture.
    await a.evaluate((id) => {
      const ws = new WebSocket(`${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/api/ws`);
      ws.onopen = () => { const f = () => ws.send(JSON.stringify({ type: "typing", conversation_id: id })); f(); setInterval(f, 2000); };
    }, convId);
    await d.close();

    const SIZES = { desktop: { width: 1440, height: 760 }, mobile: { width: 390, height: 844 } } as const;
    for (const theme of THEMES) {
      for (const [vp, size] of Object.entries(SIZES)) {
        const ctx = await browser.newContext({ viewport: size, deviceScaleFactor: 2, storageState: await dctx.storageState() });
        await ctx.addInitScript((t) => { try { localStorage.setItem("natter.theme", t); localStorage.setItem("natter.lang", "en"); } catch { /* */ } }, theme);
        const p = await ctx.newPage();
        await p.goto("/chat");
        await p.getByRole("button", { name: new RegExp(`Alya.*@${alya}`) }).click();
        await expect(p.locator("p.whitespace-pre-wrap", { hasText: "2pm" })).toBeVisible();
        await expect(p.getByText("typing", { exact: false }).first()).toBeVisible({ timeout: 10_000 });
        expect(await p.evaluate(async () => { await document.fonts.ready; return document.fonts.check('16px "Geist"'); })).toBe(true);
        await p.waitForTimeout(300);
        await p.screenshot({ path: path.join(RAW, `${vp}-${theme}.png`) });
        await ctx.close();
      }
    }
    await actx.close(); await kctx.close();
  });
});
