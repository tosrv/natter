import { test, expect, type Browser, type BrowserContext, type Page } from "@playwright/test";

const run = Date.now().toString(36).slice(-7);

async function login(browser: Browser, sub: string, email: string, username: string): Promise<{ ctx: BrowserContext; page: Page }> {
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

test("blocking: hides both ways, generic error for the blocked side, unblock restores", async ({ browser }) => {
  const aName = `ba_${run}`;
  const bName = `bb_${run}`;
  const A = await login(browser, `pub-a-${run}`, `pub-a-${run}@e2e.test`, aName);
  const B = await login(browser, `pub-b-${run}`, `pub-b-${run}@e2e.test`, bName);

  // A and B chat.
  await A.page.getByPlaceholder("Search username…").fill(bName);
  await A.page.getByRole("button", { name: new RegExp(`@${bName}`) }).click();
  await send(A.page, "hello before");
  await expect(B.page.getByText("hello before").first()).toBeVisible();
  await B.page.getByRole("button", { name: new RegExp(aName) }).click();
  await send(B.page, "reply before");
  await expect(A.page.getByText("reply before").first()).toBeVisible();

  // A blocks B from the header menu.
  await A.page.getByRole("button", { name: "Conversation menu" }).click();
  await A.page.getByRole("menuitem", { name: "Block" }).click();
  const dialog = A.page.getByRole("dialog");
  await expect(dialog).toContainText(`Block`);
  await dialog.getByRole("button", { name: "Block", exact: true }).click();
  await expect(A.page.getByText("Blocked", { exact: true }).first()).toBeVisible();
  await expect(A.page.getByText("You blocked this user")).toBeVisible();
  await expect(A.page.getByPlaceholder("Write a message…")).toHaveCount(0);

  // B is not told: no "Blocked" label for B, send fails with a generic error.
  await expect(B.page.getByText("Blocked", { exact: true })).toHaveCount(0);
  await send(B.page, "after block");
  const toast = B.page.locator("[data-sonner-toast]", { hasText: "Not found." });
  await expect(toast).toBeVisible();
  await expect(toast).not.toContainText(/block/i);
  await expect(A.page.getByText("after block")).toHaveCount(0);

  // A does not show up in B's search.
  await B.page.getByPlaceholder("Search username…").fill(aName);
  await expect(B.page.getByText("No users found.")).toBeVisible();

  // A unblocks from Settings; B can send again and A receives it.
  await A.page.getByRole("link", { name: "Settings" }).first().click();
  const row = A.page.locator("li", { hasText: `@${bName}` });
  await expect(row).toBeVisible();
  await row.getByRole("button", { name: "Unblock" }).click();
  await expect(A.page.locator("li", { hasText: `@${bName}` })).toHaveCount(0);

  await send(B.page, "after unblock");
  await A.page.goto("/chat");
  await A.page.getByRole("button", { name: new RegExp(bName) }).click();
  await expect(A.page.getByText("after unblock").first()).toBeVisible();
  await expect(A.page.getByText("Blocked", { exact: true })).toHaveCount(0);

  await A.ctx.close();
  await B.ctx.close();
});

test("landing links to terms and privacy", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("link", { name: "Terms of Service" }).first().click();
  await expect(page).toHaveURL(/\/terms$/);
  await expect(page.getByRole("heading", { level: 1, name: "Terms of Service" })).toBeVisible();
  await expect(page.getByText("Last updated: November 3, 2026")).toBeVisible();

  await page.goto("/");
  await page.getByRole("link", { name: "Privacy Policy" }).first().click();
  await expect(page).toHaveURL(/\/privacy$/);
  await expect(page.getByRole("heading", { level: 1, name: "Privacy Policy" })).toBeVisible();
  await expect(page.getByText("Last updated: November 3, 2026")).toBeVisible();
});

// Needs the server started with the Cloudflare test keys and access to challenges.cloudflare.com.
test("guest with Turnstile test key", async ({ browser }) => {
  test.skip(process.env.E2E_TURNSTILE !== "1", "set E2E_TURNSTILE=1 (server with Turnstile test keys, network access)");
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.goto("/");
  await page.getByRole("button", { name: "Continue as guest" }).first().click();
  // Turnstile renders its iframe inside a closed shadow root, so assert on the
  // effect instead: "Start chat" only enables once the widget issued a token.
  const dialog = page.getByRole("dialog");
  await expect(dialog.locator("[data-testid=turnstile]")).toBeAttached();
  const btn = dialog.getByRole("button", { name: "Start chat" });
  await expect(btn).toBeEnabled({ timeout: 30_000 });
  await btn.click();
  await expect(page).toHaveURL(/\/chat/);
  await ctx.close();
});

test("security headers and no CSP violations", async ({ browser }) => {
  const ctx = await browser.newContext();
  await ctx.addInitScript(() => {
    const w = window as unknown as { __cspViolations: string[] };
    w.__cspViolations = w.__cspViolations ?? [];
    document.addEventListener("securitypolicyviolation", (e) => {
      w.__cspViolations.push(`${e.violatedDirective} ${e.blockedURI}`);
    });
  });
  const page = await ctx.newPage();
  const consoleCsp: string[] = [];
  page.on("console", (m) => { if (m.text().includes("Content Security Policy")) consoleCsp.push(m.text()); });
  const violations = () => page.evaluate(() => (window as unknown as { __cspViolations: string[] }).__cspViolations ?? []);

  const res = await page.goto("/");
  const h = res!.headers();
  expect(h["content-security-policy"]).toBeTruthy();
  expect(h["x-content-type-options"]).toBe("nosniff");
  expect(h["x-frame-options"]).toBe("DENY");
  expect(h["referrer-policy"]).toBe("strict-origin-when-cross-origin");
  // Turnstile keeps talking to Cloudflare, so "networkidle" never settles.
  // Wait for concrete state instead: with Turnstile on, open the guest dialog;
  // "Start chat" enables only after Cloudflare's script and iframe loaded.
  await page.waitForLoadState("load");
  if (process.env.E2E_TURNSTILE === "1") {
    await page.getByRole("button", { name: "Continue as guest" }).first().click();
    await expect(page.getByRole("dialog").getByRole("button", { name: "Start chat" })).toBeEnabled({ timeout: 30_000 });
  }
  expect(await violations()).toEqual([]);

  const login = await ctx.request.post("/api/test/login", { data: { sub: `pub-csp-${run}`, email: `pub-csp-${run}@e2e.test`, username: `csp_${run}` } });
  expect(login.ok()).toBeTruthy();
  await page.goto("/chat");
  await expect(page.getByRole("heading", { name: /Chat/ })).toContainText(`csp_${run}`);
  await page.waitForLoadState("load");
  expect(await violations()).toEqual([]);

  await page.goto("/settings");
  await expect(page.getByText("Blocked users")).toBeVisible();
  await page.waitForLoadState("load");
  expect(await violations()).toEqual([]);

  expect(consoleCsp).toEqual([]);
  await ctx.close();
});
