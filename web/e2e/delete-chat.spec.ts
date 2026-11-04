import { test, expect, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { startGuest } from "./guest";

// Unique per run so the suite can be re-run against the same database.
const run = Date.now().toString(36).slice(-7);

async function fakeLogin(browser: Browser, sub: string, email: string, username: string): Promise<{ ctx: BrowserContext; page: Page }> {
  const ctx = await browser.newContext();
  const res = await ctx.request.post("/api/test/login", { data: { sub, email, username } });
  expect(res.ok(), `fake login ${email}: ${res.status()} ${await res.text()}`).toBeTruthy();
  const page = await ctx.newPage();
  await page.goto("/chat");
  await expect(page.getByRole("heading", { name: /Chat/ })).toContainText(username);
  return { ctx, page };
}

async function send(page: Page, text: string) {
  await page.getByPlaceholder("Write a message…").fill(text);
  await page.getByRole("button", { name: "Send", exact: true }).click();
}

async function deleteActiveChat(page: Page) {
  await page.getByRole("button", { name: "Conversation menu" }).click();
  await page.getByRole("menuitem", { name: "Delete chat" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Delete this chat?")).toBeVisible();
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();
  await expect(dialog).toBeHidden();
}

const msg = (page: Page, text: string) => page.locator("p.whitespace-pre-wrap", { hasText: text });

test("delete chat: removed for the deleter only, new message revives it with only new history", async ({ browser }) => {
  const aName = `da_${run}`;
  const bName = `db_${run}`;
  const A = await fakeLogin(browser, `e2e-da-${run}`, `da-${run}@e2e.test`, aName);
  const B = await fakeLogin(browser, `e2e-db-${run}`, `db-${run}@e2e.test`, bName);

  const oldA = `old-from-a-${run}`;
  const oldB = `old-from-b-${run}`;
  const fresh = `fresh-from-b-${run}`;

  // A opens B via search, both exchange a message.
  await A.page.getByPlaceholder("Search username…").fill(bName);
  await A.page.getByRole("button", { name: new RegExp(`@${bName}`) }).click();
  await send(A.page, oldA);
  await expect(A.page.getByRole("img", { name: "Sent", exact: true })).toBeVisible();
  await B.page.getByRole("button", { name: new RegExp(aName) }).click();
  await expect(msg(B.page, oldA)).toBeVisible();
  await send(B.page, oldB);
  await expect(msg(A.page, oldB)).toBeVisible();

  // A deletes the chat: it leaves A's list.
  await deleteActiveChat(A.page);
  await expect(A.page.getByRole("button", { name: new RegExp(bName) })).toHaveCount(0);
  await expect(msg(A.page, oldA)).toHaveCount(0);

  // B keeps the history.
  await expect(msg(B.page, oldA)).toBeVisible();
  await expect(msg(B.page, oldB)).toBeVisible();

  // B writes again: the chat comes back for A with only the new message.
  await send(B.page, fresh);
  const conv = A.page.getByRole("button", { name: new RegExp(bName) });
  await expect(conv).toBeVisible();
  await conv.click();
  await expect(msg(A.page, fresh)).toBeVisible();
  await expect(msg(A.page, oldA)).toHaveCount(0);
  await expect(msg(A.page, oldB)).toHaveCount(0);

  // Still gone after a reload (history is server-side, not just cached).
  await A.page.reload();
  await A.page.getByRole("button", { name: new RegExp(bName) }).click();
  await expect(msg(A.page, fresh)).toBeVisible();
  await expect(msg(A.page, oldA)).toHaveCount(0);

  for (const c of [A.ctx, B.ctx]) await c.close();
});

test("delete chat as guest clears history and the composer keeps working", async ({ browser }) => {
  const dev = await fakeLogin(browser, "e2e-dev", "dev@e2e.test", "dev");
  const gctx = await browser.newContext();
  const g = await gctx.newPage();
  const guestName = `Del Guest ${run}`;
  const contact = `del-${run}@example.com`;
  await startGuest(g, { name: guestName, contact });
  await expect(msg(g, contact)).toBeVisible();

  await deleteActiveChat(g);
  await expect(msg(g, contact)).toHaveCount(0);

  const after = `after-delete-${run}`;
  await send(g, after);
  await expect(msg(g, after)).toBeVisible();
  await expect(g.getByRole("img", { name: "Sent", exact: true })).toBeVisible();
  const conv = dev.page.getByRole("button", { name: new RegExp(guestName) });
  await conv.click();
  await expect(msg(dev.page, after)).toBeVisible();

  for (const c of [dev.ctx, gctx]) await c.close();
});
