import { test, expect, type Page } from "@playwright/test";
import http from "node:http";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

// Cross-origin embed test: the spec serves a tiny host page on HOST_PORT
// (default 8081, a different origin from the pmess server) that loads
// ${E2E_BASE_URL}/widget.js. The server must list the host origin in
// ALLOWED_ORIGINS (e.g. ALLOWED_ORIGINS=http://localhost:8099,http://localhost:8081).
// With E2E_TURNSTILE=1 the server uses Turnstile test keys and the widget
// solves the real (always-pass) test challenge. Otherwise the server runs
// without Turnstile keys (accepting any token) and the Cloudflare script is
// replaced by a stub that immediately yields a dummy token, so the test also
// runs where challenges.cloudflare.com is unreachable.
const REAL_TURNSTILE = process.env.E2E_TURNSTILE === "1";
const TURNSTILE_STUB = `window.turnstile = {
  render(el, o) { setTimeout(() => o.callback("e2e-stub-token"), 0); return "stub"; },
  remove() {}, reset() {},
};`;
const BASE = (process.env.E2E_BASE_URL ?? "http://localhost:8080").replace(/\/+$/, "");
const HOST_PORT = Number(process.env.E2E_HOST_PORT ?? 8081);
const HOST_URL = `http://localhost:${HOST_PORT}/host.html`;
const here = path.dirname(fileURLToPath(import.meta.url));
const template = fs.readFileSync(path.join(here, "fixtures", "host.html"), "utf8");

let server: http.Server;

test.beforeAll(async () => {
  const html = template.replaceAll("__PMESS_BASE__", BASE);
  server = http.createServer((req, res) => {
    if (req.url?.split("?")[0] === "/host.html") {
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(html);
    } else {
      res.writeHead(404).end();
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(HOST_PORT, "localhost", resolve);
  });
});

test.afterAll(async () => {
  await new Promise((r) => server?.close(r));
});

function widget(page: Page) {
  const host = page.locator("pmess-chat");
  return {
    launcher: host.locator("button.pm-launch"),
    panel: host.getByRole("dialog"),
    input: host.getByRole("textbox", { name: "Type a message..." }),
    send: host.getByRole("button", { name: "Send", exact: true }),
    msg: (text: string) => host.locator(".pm-msg", { hasText: text }),
  };
}

test("guest chats from an embedded widget on another origin", async ({ browser }) => {
  const run = Date.now().toString(36).slice(-7);
  const text = `hi from widget ${run}`;
  const reply = `reply to widget ${run}`;

  const dctx = await browser.newContext();
  const res = await dctx.request.post("/api/test/login", {
    data: { sub: "e2e-dev", email: "dev@e2e.test", username: "dev" },
  });
  expect(res.ok(), `fake login dev: ${res.status()}`).toBeTruthy();
  const dev = await dctx.newPage();
  await dev.goto("/chat");
  await expect(dev.getByRole("heading", { name: /Chat/ })).toBeVisible();

  const gctx = await browser.newContext();
  if (!REAL_TURNSTILE) {
    await gctx.route("https://challenges.cloudflare.com/**", (r) =>
      r.fulfill({ status: 200, contentType: "text/javascript", body: TURNSTILE_STUB }));
  }
  const g = await gctx.newPage();
  await g.goto(HOST_URL);
  let w = widget(g);
  await w.launcher.click();
  await expect(w.panel).toBeVisible();
  // Turnstile (test key, if configured) resolves itself; then the composer appears.
  await w.input.fill(text);
  await w.send.click();
  await expect(w.msg(text)).toContainText("✓");

  // Developer sees the message, opens the conversation and replies.
  await expect(dev.getByText(text).first()).toBeVisible();
  await dev.getByRole("button", { name: new RegExp(text) }).first().click();
  await expect(dev.locator("p.whitespace-pre-wrap", { hasText: text })).toBeVisible();
  await dev.getByPlaceholder("Write a message…").fill(reply);
  await dev.getByRole("button", { name: "Send", exact: true }).click();

  await expect(w.msg(reply)).toBeVisible();
  await expect(w.msg(text)).toContainText("✓✓");

  // Reload: history persists without a new Turnstile challenge.
  await g.reload();
  w = widget(g);
  await w.launcher.click();
  await expect(w.msg(text)).toBeVisible();
  await expect(w.msg(reply)).toBeVisible();
  await expect(w.input).toBeVisible();

  // End the guest session the same way the app's Settings does (DELETE /api/me),
  // using the bearer token the widget stored on the host origin.
  const token = await g.evaluate((k) => localStorage.getItem(k), `pmess:${BASE}`);
  expect(token, "widget token in host localStorage").toBeTruthy();
  const del = await gctx.request.delete(`${BASE}/api/me`, { headers: { Authorization: `Bearer ${token}` } });
  expect(del.status(), `end guest: ${del.status()}`).toBe(204);

  // The server closes the guest's socket (1008); the widget confirms over HTTP
  // and shows the ended state live, without a reload.
  await expect(w.panel.getByRole("button", { name: "Start a new chat" })).toBeVisible();
  await expect(w.panel).toContainText("This conversation has ended.");

  await dctx.close();
  await gctx.close();
});
