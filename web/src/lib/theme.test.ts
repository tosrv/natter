import { describe, it, expect, afterEach, vi } from "vitest";
import { getThemePref, setThemePref, applyTheme } from "./theme";

afterEach(() => { vi.unstubAllGlobals(); localStorage.clear(); document.documentElement.classList.remove("dark"); });

describe("theme", () => {
  it("round-trips", () => { setThemePref("dark"); expect(getThemePref()).toBe("dark"); });
  it("defaults to system", () => expect(getThemePref()).toBe("system"));
  it("throwing storage -> system", () => {
    vi.stubGlobal("localStorage", { getItem() { throw new Error("x"); }, setItem() { throw new Error("x"); } });
    expect(getThemePref()).toBe("system");
    expect(() => setThemePref("light")).not.toThrow();
  });
  it("applyTheme toggles .dark", () => {
    applyTheme("dark");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    applyTheme("light");
    expect(document.documentElement.classList.contains("dark")).toBe(false);
  });
});
