import { test, expect, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { startGuest } from "./guest";

// Unique per run so the suite can be re-run against the same database.
const run = Date.now().toString(36).slice(-7);

type Login = { sub: string; email: string; username?: string; display_name?: string };

async function login(browser: Browser, data: Login): Promise<{ ctx: BrowserContext; page: Page }> {
  const ctx = await browser.newContext();
  const res = await ctx.request.post("/api/test/login", { data });
  expect(res.ok(), `fake login ${data.email}: ${res.status()} ${await res.text()}`).toBeTruthy();
  return { ctx, page: await ctx.newPage() };
}

const msgText = (p: Page, body: string) => p.locator("p.whitespace-pre-wrap", { hasText: body });

test("revamp: onboarding, search, ticks, settings, delete account", async ({ browser }) => {
  // Developer must exist so guest flows elsewhere work.
  const dev = await login(browser, { sub: "e2e-dev", email: "dev@e2e.test", username: "dev" });
  await dev.ctx.close();

  // (a) New user without username → onboarding → chat.
  const aUser = `budi_${run}`;
  const A = await login(browser, { sub: `rv-a-${run}`, email: `rv-a-${run}@e2e.test` });
  await A.page.goto("/chat");
  await expect(A.page).toHaveURL(/\/onboarding$/);
  await A.page.getByLabel("Display name").fill("Budi Santoso");
  await A.page.getByLabel("Choose a username").fill(aUser);
  await A.page.getByRole("button", { name: "Continue" }).click();
  await expect(A.page).toHaveURL(/\/chat$/);
  await expect(A.page.getByRole("heading", { name: /Chat/ })).toContainText(aUser);

  const bUser = `sari_${run}`;
  const B = await login(browser, { sub: `rv-b-${run}`, email: `rv-b-${run}@e2e.test`, username: bUser, display_name: "Sari Dewi" });
  await B.page.goto("/chat");
  await expect(B.page.getByRole("heading", { name: /Chat/ })).toContainText(bUser);

  // (b) A searches B by username prefix → display name and @username.
  await A.page.getByPlaceholder("Search username…").fill(bUser.slice(0, bUser.length - 2));
  const hit = A.page.getByRole("button", { name: new RegExp(`Sari Dewi.*@${bUser}`) });
  await expect(hit).toBeVisible();
  await hit.click();

  // (c) A sends HTML-looking text; B sees it literally; ticks go Sent → Read.
  const html = "<b>hi</b>";
  await A.page.getByPlaceholder("Write a message…").fill(html);
  await A.page.getByRole("button", { name: "Send", exact: true }).click();
  await expect(A.page.getByRole("img", { name: "Sent", exact: true })).toBeVisible();

  await expect(B.page.getByText(html, { exact: true }).first()).toBeVisible();
  await expect(B.page.locator("b", { hasText: "hi" })).toHaveCount(0);
  await B.page.getByRole("button", { name: new RegExp(`Budi Santoso.*@${aUser}`) }).click();
  await expect(msgText(B.page, html)).toBeVisible();
  await expect(B.page.locator("p.truncate.font-semibold", { hasText: "Budi Santoso" })).toBeVisible();
  await expect(A.page.getByRole("img", { name: "Read", exact: true })).toBeVisible();

  // (d) Settings: dark theme persists; language switch; display name change.
  await A.page.getByRole("link", { name: "Settings" }).click();
  await expect(A.page).toHaveURL(/\/settings$/);
  await A.page.getByRole("radio", { name: "Dark", exact: true }).click();
  await expect(A.page.getByRole("radio", { name: "Dark", exact: true })).toHaveAttribute("aria-checked", "true");
  await expect(A.page.locator("html")).toHaveClass(/\bdark\b/);
  await A.page.reload();
  await expect(A.page.locator("html")).toHaveClass(/\bdark\b/);

  await A.page.getByRole("button", { name: "ID", exact: true }).click();
  await expect(A.page.getByRole("heading", { name: "Pengaturan" })).toBeVisible();
  await A.page.getByRole("button", { name: "EN", exact: true }).click();
  await expect(A.page.getByRole("heading", { name: "Settings" })).toBeVisible();

  const display = A.page.getByLabel("Display name");
  await display.fill("Budi S.");
  await display.press("Enter");
  await expect(A.page.getByText("Saved")).toBeVisible();

  await B.page.reload();
  await B.page.getByRole("button", { name: new RegExp(`Budi S\\..*@${aUser}`) }).click();
  await expect(msgText(B.page, html)).toBeVisible();

  // (e) A deletes the account while B has the A–B conversation open.
  await A.page.getByRole("button", { name: "Delete account" }).click();
  const dlg = A.page.getByRole("dialog");
  const confirm = dlg.getByRole("button", { name: "Delete permanently" });
  await dlg.getByRole("textbox").fill(aUser.toUpperCase());
  await expect(confirm).toBeDisabled();
  await dlg.getByRole("textbox").fill(aUser);
  await confirm.click();
  await expect(A.page).toHaveURL(/\/$/);
  await expect(A.page.getByRole("heading", { level: 1, name: "Conversations, made simple." })).toBeVisible();
  await expect(A.page.getByRole("button", { name: "Continue as guest" }).first()).toBeVisible();

  await expect(B.page.getByText("This conversation was deleted.")).toBeVisible();
  await expect(B.page.getByText("Select a conversation.")).toBeVisible();
  await expect(B.page.getByRole("button", { name: new RegExp(`@${aUser}`) })).toHaveCount(0);

  await A.ctx.close();
  await B.ctx.close();
});

test("revamp: guest starts from the dialog and sees limited settings", async ({ browser }) => {
  const dev = await login(browser, { sub: "e2e-dev", email: "dev@e2e.test", username: "dev" });
  await dev.ctx.close();

  const ctx = await browser.newContext();
  const g = await ctx.newPage();
  await startGuest(g); // default random name, no contact

  await g.goto("/settings");
  await expect(g.getByRole("heading", { name: "Settings" })).toBeVisible();
  const sections = g.locator("section h2");
  await expect(sections).toHaveText(["Language", "Theme", "Danger zone"]);
  await expect(g.getByRole("button", { name: "End session" })).toBeVisible();
  await expect(g.getByLabel("Display name")).toHaveCount(0);
  await expect(g.getByRole("button", { name: "Log out" })).toHaveCount(0);
  await expect(g.getByRole("button", { name: "Delete account" })).toHaveCount(0);

  await g.getByRole("button", { name: "End session" }).click();
  await expect(g.getByRole("heading", { level: 1, name: "Conversations, made simple." })).toBeVisible();
  await expect(g.getByRole("button", { name: "Continue as guest" }).first()).toBeVisible();
  await ctx.close();
});

test("dark theme shows dark product shot", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.addInitScript(() => { try { localStorage.setItem("natter.lang", "en"); localStorage.removeItem("natter.theme"); } catch { /* */ } });
  await page.goto("/");
  const hero = page.locator("#top");
  const before = await hero.boundingBox();
  await page.evaluate(() => Promise.all(Array.from(document.querySelectorAll<HTMLImageElement>("#top img[loading=eager]")).filter((i) => !i.complete).map((i) => new Promise((r) => { i.onload = i.onerror = r; }))));
  expect(await hero.boundingBox()).toEqual(before);
  await page.getByRole("radio", { name: "Dark" }).first().click();
  await expect(page.locator("html")).toHaveClass(/dark/);
  await expect(page.locator('#top picture:has(source[srcset*="desktop-dark"]) img')).toBeVisible();
  await expect(page.locator('#top picture:has(source[srcset*="desktop-light"]) img')).toBeHidden();
});
