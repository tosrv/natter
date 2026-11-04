import { describe, it, expect, vi } from "vitest";

describe("main", () => {
  it("double load is harmless", async () => {
    await import("./main");
    vi.resetModules();
    await expect(import("./main")).resolves.toBeDefined();
    expect(customElements.get("natter-chat")).toBeDefined();
  });
  it("renders launcher with aria-label in shadow root", async () => {
    await import("./main");
    const el = document.createElement("natter-chat");
    el.setAttribute("lang", "en");
    document.body.appendChild(el);
    await new Promise((r) => setTimeout(r, 0));
    const btn = el.shadowRoot!.querySelector("button");
    expect(btn?.getAttribute("aria-label")).toBe("Open chat");
  });
});
