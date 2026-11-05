export type ThemePref = "light" | "dark" | "system";
const KEY = "natter.theme";

export function getThemePref(): ThemePref {
  try {
    const v = localStorage.getItem(KEY);
    if (v === "light" || v === "dark" || v === "system") return v;
  } catch { /* storage unavailable */ }
  return "system";
}

export function setThemePref(p: ThemePref): void {
  try { localStorage.setItem(KEY, p); } catch { /* ignore */ }
}

let mq: MediaQueryList | null = null;
let listener: (() => void) | null = null;

export function applyTheme(p: ThemePref): void {
  const root = document.documentElement;
  if (mq && listener) { mq.removeEventListener("change", listener); mq = null; listener = null; }
  if (p !== "system") {
    root.classList.toggle("dark", p === "dark");
    return;
  }
  if (typeof window.matchMedia !== "function") { root.classList.remove("dark"); return; }
  mq = window.matchMedia("(prefers-color-scheme: dark)");
  const m = mq;
  listener = () => root.classList.toggle("dark", m.matches);
  listener();
  mq.addEventListener("change", listener);
}

// --- Hydration-safe subscriptions (server/hydration snapshot never depends on the theme) ---
const prefListeners = new Set<() => void>();

/** Persists the pref and notifies useThemePref subscribers. */
export function choosePref(p: ThemePref): void {
  setThemePref(p);
  applyTheme(p);
  prefListeners.forEach((l) => l());
}

export function subscribePref(cb: () => void): () => void {
  prefListeners.add(cb);
  return () => { prefListeners.delete(cb); };
}

/** Subscribes to the `dark` class on <html> (set by theme-init.js / applyTheme). */
export function subscribeDark(cb: () => void): () => void {
  if (typeof MutationObserver === "undefined") return () => {};
  const mo = new MutationObserver(cb);
  mo.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
  return () => mo.disconnect();
}

export const isDarkNow = () => document.documentElement.classList.contains("dark");
