import { describe, it, expect, afterEach, vi } from "vitest";
import { getThemePref, setThemePref, applyTheme } from "./theme";
import { readFileSync } from "node:fs";
import path from "node:path";

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

describe("theme-init", () => {
  const src = readFileSync(path.resolve(__dirname, "../../public/theme-init.js"), "utf8");
  const run = () => new Function(src)();
  const dark = () => document.documentElement.classList.contains("dark");
  const mm = (matches: boolean) => vi.stubGlobal("matchMedia", () => ({ matches }));

  it("dark pref adds .dark", () => { localStorage.setItem("natter.theme", "dark"); mm(false); run(); expect(dark()).toBe(true); });
  it("light pref wins over dark system", () => { localStorage.setItem("natter.theme", "light"); mm(true); run(); expect(dark()).toBe(false); });
  it("theme-init falls back to system on bad value", () => {
    localStorage.setItem("natter.theme", "blue"); mm(true); run(); expect(dark()).toBe(true);
    document.documentElement.classList.remove("dark");
    mm(false); run(); expect(dark()).toBe(false);
  });
  it("throwing storage follows matchMedia and does not throw", () => {
    vi.stubGlobal("localStorage", { getItem() { throw new Error("x"); } });
    mm(true); expect(run).not.toThrow(); expect(dark()).toBe(true);
  });
  it("missing matchMedia does not throw", () => {
    vi.stubGlobal("matchMedia", undefined);
    expect(run).not.toThrow(); expect(dark()).toBe(false);
  });
});
