import { test, expect } from "@playwright/test";
import { startGuest } from "./guest";

// Unique per run so the suite can be re-run against the same database.
const run = Date.now().toString(36).slice(-7);

test("ended guest keeps its conversation visible to the developer", async ({ browser }) => {
  const dctx = await browser.newContext();
  const res = await dctx.request.post("/api/test/login", {
    data: { sub: "e2e-dev", email: "dev@e2e.test", username: "dev" },
  });
  expect(res.ok(), `fake login dev: ${res.status()}`).toBeTruthy();
  const dev = await dctx.newPage();
  await dev.goto("/chat");
  await expect(dev.getByRole("heading", { name: /Chat/ })).toBeVisible();

  const gctx = await browser.newContext();
  const g = await gctx.newPage();
  const contact = `contact-${run}@example.com`;
  await startGuest(g, { contact });
  await expect(g.locator("p.whitespace-pre-wrap", { hasText: contact })).toBeVisible();

  // Developer sees the guest's contact; open the conversation.
  await expect(dev.getByText(contact).first()).toBeVisible();

  // Guest ends the session from Settings.
  await g.goto("/settings");
  await g.getByRole("button", { name: "End session" }).click();
  await expect(g.getByRole("heading", { level: 1, name: "Conversations, made simple." })).toBeVisible();
  await expect(g.getByRole("button", { name: "Continue as guest" }).first()).toBeVisible();

  // Conversation is still there, marked ended, and cannot be replied to.
  await dev.reload();
  await dev.getByRole("button", { name: new RegExp(contact.replace(/[.]/g, "\\.")) }).first().click();
  await expect(dev.locator("p.whitespace-pre-wrap", { hasText: contact })).toBeVisible();
  await expect(dev.getByText("Session ended").first()).toBeVisible();
  await expect(dev.getByText("This guest has left. Reach out through the contact they shared, if any.")).toBeVisible();
  await expect(dev.getByPlaceholder("Write a message…")).toHaveCount(0);

  await dctx.close();
  await gctx.close();
});
