import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { createRoot, type Root } from "react-dom/client";
import { act } from "react-dom/test-utils";
import { MemoryRouter } from "react-router-dom";
import i18n from "@/i18n";

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    getMe: () => Promise.reject(new Error("401")),
    getConfig: () => Promise.resolve({ turnstile_site_key: "" }),
  };
});
vi.mock("@/components/TurnstileWidget", () => ({ default: () => null }));

import Landing from "./Landing";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let host: HTMLDivElement;
let root: Root;
beforeEach(() => { host = document.createElement("div"); document.body.appendChild(host); root = createRoot(host); });
afterEach(async () => { act(() => root.unmount()); host.remove(); await i18n.changeLanguage("en"); });

async function render(lang: "en" | "id") {
  await i18n.changeLanguage(lang);
  await act(async () => { root.render(<MemoryRouter><Landing /></MemoryRouter>); });
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
}
const buttons = (name: string) => Array.from(document.querySelectorAll<HTMLElement>("button, a")).filter((b) => b.textContent?.trim() === name);

describe("Landing", () => {
  it("renders the EN copy with one h1 and both CTAs", async () => {
    await render("en");
    const h1s = host.querySelectorAll("h1");
    expect(h1s).toHaveLength(1);
    expect(h1s[0].textContent).toBe("Conversations, made simple.");
    expect(buttons("Sign in with Google").length).toBeGreaterThan(0);
    expect(buttons("Continue as guest").length).toBeGreaterThan(0);
    const text = host.textContent ?? "";
    expect(text).toContain("Natter is Matter.");
    expect(text).toContain("Less noise. More conversation.");
    expect(text).toContain("Not end-to-end encrypted.");
    expect(text).not.toMatch(/FAQ|Frequently/);
    expect(host.querySelector('a[href="#main"]')).not.toBeNull();
    expect(host.querySelector("main#main")).not.toBeNull();
    for (const id of ["product", "how", "privacy"]) expect(host.querySelector(`#${id}`)).not.toBeNull();
  });

  it("renders the ID copy", async () => {
    await render("id");
    expect(host.querySelector("h1")!.textContent).toBe("Ngobrol, jadi sederhana.");
    expect(buttons("Lanjut sebagai tamu").length).toBeGreaterThan(0);
  });

  it("opens the guest dialog when guest is enabled", async () => {
    await render("en");
    const btn = buttons("Continue as guest").find((b) => b.tagName === "BUTTON")!;
    expect(btn.hasAttribute("disabled")).toBe(false);
    await act(async () => { btn.click(); });
    expect(document.querySelector('[role="dialog"]')).not.toBeNull();
  });
});
