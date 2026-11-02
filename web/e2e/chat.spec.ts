import { test, expect, type Browser, type BrowserContext, type Page } from "@playwright/test";

// Unique per run so the suite can be re-run against the same database.
const run = Date.now().toString(36).slice(-7);
const DEV_EMAIL = "dev@e2e.test";

async function fakeLogin(browser: Browser, sub: string, email: string, username: string): Promise<{ ctx: BrowserContext; page: Page }> {
  const ctx = await browser.newContext();
  const res = await ctx.request.post("/api/test/login", { data: { sub, email, username } });
  expect(res.ok(), `fake login ${email}: ${res.status()} ${await res.text()}`).toBeTruthy();
  const page = await ctx.newPage();
  await page.goto("/chat");
  await expect(page.getByRole("heading", { name: /Chat/ })).toContainText(username);
  return { ctx, page };
}

test("two users chat in realtime, guest contacts the developer", async ({ browser }) => {
  // Developer first, so the guest flow has someone to talk to.
  const dev = await fakeLogin(browser, "e2e-dev", DEV_EMAIL, "dev");

  const aName = `a_${run}`;
  const bName = `b_${run}`;
  const A = await fakeLogin(browser, `e2e-a-${run}`, `a-${run}@e2e.test`, aName);
  const B = await fakeLogin(browser, `e2e-b-${run}`, `b-${run}@e2e.test`, bName);

  // A searches B and opens the conversation.
  await A.page.getByPlaceholder("Search username…").fill(bName);
  await A.page.getByRole("button", { name: new RegExp(`@${bName}`) }).click();

  // A sends HTML-looking text; B sees it literally, without reload, plus a toast.
  const html = "<b>hi</b>";
  await A.page.getByPlaceholder("Write a message…").fill(html);
  await A.page.getByRole("button", { name: "Send", exact: true }).click();
  await expect(A.page.getByRole("img", { name: "Sent", exact: true })).toBeVisible();

  await expect(B.page.getByText(/New message from/)).toBeVisible();
  await expect(B.page.getByText(html, { exact: true }).first()).toBeVisible();
  await expect(B.page.locator("b", { hasText: "hi" })).toHaveCount(0);

  // B opens the conversation → A sees the double check (read).
  await B.page.getByRole("button", { name: new RegExp(aName) }).click();
  await expect(B.page.locator("p.whitespace-pre-wrap", { hasText: html })).toBeVisible();
  await expect(A.page.getByRole("img", { name: "Read", exact: true })).toBeVisible();

  // Typing indicator.
  await B.page.getByPlaceholder("Write a message…").pressSequentially("hel");
  await expect(A.page.getByText("typing…")).toBeVisible();

  // Language switch (now in Settings) changes the chat UI text.
  await A.page.getByRole("link", { name: "Settings" }).first().click();
  await A.page.getByRole("button", { name: "ID", exact: true }).click();
  await A.page.goBack();
  await A.page.getByRole("button", { name: new RegExp(bName) }).click();
  await expect(A.page.getByRole("heading", { name: /Obrolan/ })).toBeVisible();
  await expect(A.page.getByPlaceholder("Tulis pesan…")).toBeVisible();

  // Guest: contact dialog appears, answer reaches the developer; no search box.
  const gctx = await browser.newContext();
  const g = await gctx.newPage();
  await g.goto("/");
  await g.getByRole("button", { name: "Continue as guest" }).click();
  const dialog = g.getByRole("dialog");
  await expect(dialog).toContainText("How can the developer reach you?");
  const contact = `contact-${run}@example.com`;
  await dialog.getByRole("textbox").fill(contact);
  await dialog.getByRole("button", { name: "Send" }).click();
  await expect(dialog).toBeHidden();
  await expect(g.locator("p.whitespace-pre-wrap", { hasText: contact })).toBeVisible();
  await expect(g.getByPlaceholder("Search username…")).toHaveCount(0);

  await expect(dev.page.getByText(contact).first()).toBeVisible();

  for (const c of [dev.ctx, A.ctx, B.ctx, gctx]) await c.close();
});
