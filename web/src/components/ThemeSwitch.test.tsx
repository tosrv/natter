import { describe, it, expect, afterEach, vi } from "vitest";
import { createRoot, type Root } from "react-dom/client";
import { act } from "react-dom/test-utils";
import "@/i18n";
import ThemeSwitch from "./ThemeSwitch";

const root = document.documentElement;
(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let host: HTMLDivElement;
let rr: Root;
function render(el: React.ReactElement) { host = document.createElement("div"); document.body.appendChild(host); rr = createRoot(host); act(() => rr.render(el)); }
const radios = () => Array.from(host.querySelectorAll<HTMLElement>('[role="radio"]'));
const radio = (name: string) => radios().find((r) => r.getAttribute("aria-label") === name)!;
const click = (el: HTMLElement) => act(() => { el.dispatchEvent(new MouseEvent("click", { bubbles: true })); });
const key = (el: HTMLElement, k: string) => act(() => { el.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true })); });

function fakeMatchMedia(initial: boolean) {
  let matches = initial;
  const listeners = new Set<() => void>();
  const mq = {
    get matches() { return matches; },
    addEventListener: (_: string, l: () => void) => listeners.add(l),
    removeEventListener: (_: string, l: () => void) => listeners.delete(l),
  };
  vi.stubGlobal("matchMedia", () => mq);
  return (v: boolean) => { matches = v; listeners.forEach((l) => l()); };
}

afterEach(() => { act(() => rr.unmount()); host.remove(); vi.unstubAllGlobals(); localStorage.clear(); root.classList.remove("dark"); });

describe("ThemeSwitch", () => {
  it("renders a radiogroup with three radios", () => {
    render(<ThemeSwitch />);
    expect(radios()).toHaveLength(3);
    expect(host.querySelector('[role="radiogroup"]')).toBeTruthy();
  });
  it("Dark click stores pref and sets .dark", () => {
    render(<ThemeSwitch />);
    click(radio("Dark"));
    expect(localStorage.getItem("natter.theme")).toBe("dark");
    expect(root.classList.contains("dark")).toBe(true);
    expect(radio("Dark").getAttribute("aria-checked")).toBe("true");
  });
  it("ThemeSwitch system follows matchMedia change", () => {
    const set = fakeMatchMedia(true);
    render(<ThemeSwitch />);
    click(radio("System"));
    expect(localStorage.getItem("natter.theme")).toBe("system");
    expect(root.classList.contains("dark")).toBe(true);
    act(() => set(false));
    expect(root.classList.contains("dark")).toBe(false);
  });
  it("ArrowRight from Light selects Dark", () => {
    localStorage.setItem("natter.theme", "light");
    render(<ThemeSwitch />);
    key(radio("Light"), "ArrowRight");
    expect(radio("Dark").getAttribute("aria-checked")).toBe("true");
    expect(root.classList.contains("dark")).toBe(true);
  });
  it("ArrowLeft wraps from Light to System", () => {
    localStorage.setItem("natter.theme", "light");
    render(<ThemeSwitch />);
    key(radio("Light"), "ArrowLeft");
    expect(radio("System").getAttribute("aria-checked")).toBe("true");
  });
});
