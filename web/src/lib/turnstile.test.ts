import { describe, it, expect, afterEach, beforeEach, vi } from "vitest";
const load = async () => (await import("./turnstile"));

beforeEach(() => { vi.resetModules(); });
afterEach(() => {
  document.head.querySelectorAll("script").forEach((s) => s.remove());
  vi.useRealTimers();
});

describe("loadTurnstileScript", () => {
  it("inserts a single script when called twice", async () => {
    const { loadTurnstileScript, TURNSTILE_SRC } = await load();
    const a = loadTurnstileScript();
    const b = loadTurnstileScript();
    const scripts = document.head.querySelectorAll(`script[src="${TURNSTILE_SRC}"]`);
    expect(scripts.length).toBe(1);
    scripts[0].dispatchEvent(new Event("load"));
    await expect(Promise.all([a, b])).resolves.toBeDefined();
  });
  it("rejects on timeout", async () => {
    const { loadTurnstileScript } = await load();
    vi.useFakeTimers();
    const p = loadTurnstileScript(50);
    const assertion = expect(p).rejects.toThrow();
    await vi.advanceTimersByTimeAsync(60);
    await assertion;
  });
  it("rejects on script error and allows a retry with a fresh script", async () => {
    const { loadTurnstileScript } = await load();
    const p = loadTurnstileScript();
    document.head.querySelector("script")!.dispatchEvent(new Event("error"));
    await expect(p).rejects.toThrow();
    expect(document.head.querySelectorAll("script").length).toBe(0);
    const p2 = loadTurnstileScript();
    expect(document.head.querySelectorAll("script").length).toBe(1);
    document.head.querySelector("script")!.dispatchEvent(new Event("load"));
    await expect(p2).resolves.toBeUndefined();
  });
});
