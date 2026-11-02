import { test, expect, type Browser, type Page } from "@playwright/test";
import path from "node:path";

// Generates docs/screenshots/*.png. Skipped unless SCREENSHOTS=1:
//   SCREENSHOTS=1 E2E_BASE_URL=http://localhost:8099 npx playwright test screenshots
test.skip(!process.env.SCREENSHOTS, "set SCREENSHOTS=1 to regenerate screenshots");

const OUT = path.resolve(process.cwd(), "../docs/screenshots"); // run from web/
const run = Date.now().toString(36).slice(-6);
const VIEWPORTS = { desktop: { width: 1280, height: 800 }, mobile: { width: 390, height: 844 } } as const;
const THEMES = ["light", "dark"] as const;

async function user(browser: Browser, sub: string, username: string, display_name: string) {
  const ctx = await browser.newContext();
  const res = await ctx.request.post("/api/test/login", { data: { sub, email: `${sub}@e2e.test`, username, display_name } });
  expect(res.ok()).toBeTruthy();
  return ctx;
}

async function send(p: Page, body: string) {
  await p.getByPlaceholder("Write a message…").fill(body);
  await p.getByRole("button", { name: "Send", exact: true }).click();
  await expect(p.locator("p.whitespace-pre-wrap", { hasText: body }).last()).toBeVisible();
}

test("screenshots", async ({ browser }) => {
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
      await ctx.addInitScript((t) => { try { localStorage.setItem("pmess.theme", t); localStorage.setItem("pmess.lang", "en"); } catch { /* */ } }, theme);
      const p = await ctx.newPage();
      const shot = (name: string) => p.screenshot({ path: path.join(OUT, `${name}-${theme}-${vp}.png`) });

      const anon = await browser.newContext({ viewport: size });
      await anon.addInitScript((t) => { try { localStorage.setItem("pmess.theme", t); localStorage.setItem("pmess.lang", "en"); } catch { /* */ } }, theme);
      const lp = await anon.newPage();
      await lp.goto("/");
      await expect(lp.getByRole("button", { name: "Continue as guest" })).toBeVisible();
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
