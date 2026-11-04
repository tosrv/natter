import { expect, type Page } from "@playwright/test";

// Opens the guest start dialog from the landing page, fills the name and
// (optional) contact, waits for Turnstile when the server has keys, and starts.
export async function startGuest(g: Page, opts: { name?: string; contact?: string } = {}) {
  await g.goto("/");
  await g.getByRole("button", { name: "Continue as guest" }).first().click();
  const dialog = g.getByRole("dialog");
  const nameInput = dialog.getByRole("textbox", { name: "Your name" });
  await expect(nameInput).toHaveValue(/^natter-/);
  if (opts.name !== undefined) await nameInput.fill(opts.name);
  if (opts.contact !== undefined) await dialog.getByRole("textbox", { name: /Contact \(optional\)/ }).fill(opts.contact);
  const start = dialog.getByRole("button", { name: "Start chat" });
  await expect(start).toBeEnabled({ timeout: 30_000 });
  await start.click();
  await expect(dialog).toBeHidden();
  await expect(g).toHaveURL(/\/chat/);
}
