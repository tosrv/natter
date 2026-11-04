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
