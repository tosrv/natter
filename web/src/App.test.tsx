import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { createRoot, type Root } from "react-dom/client";
import { act } from "react-dom/test-utils";
import i18n from "@/i18n";
import App from "@/App";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let host: HTMLDivElement;
let root: Root;

beforeEach(async () => {
  await i18n.changeLanguage("en");
  host = document.createElement("div");
  document.body.appendChild(host);
  root = createRoot(host);
});
afterEach(() => {
  act(() => root.unmount());
  host.remove();
});

async function renderAt(path: string) {
  window.history.pushState({}, "", path);
  await act(async () => {
    root.render(<App />);
  });
}

const waitFor = async (cond: () => boolean) => {
  for (let i = 0; i < 100 && !cond(); i++) await act(async () => { await new Promise((r) => setTimeout(r, 20)); });
};

describe("App routes", () => {
  it("renders the landing page eagerly at /", async () => {
    await renderAt("/");
    expect(host.querySelector("h1")).not.toBeNull();
    expect(host.textContent).not.toContain(i18n.t("common.loading"));
  });

  it("shows the Suspense fallback then the lazy Terms page at /terms", async () => {
    window.history.pushState({}, "", "/terms");
    act(() => { root.render(<App />); });
    expect(host.querySelector("p.text-muted-foreground")?.textContent).toBe(i18n.t("common.loading"));
    await waitFor(() => host.querySelector("h1") !== null);
    expect(host.querySelector("h1")?.textContent).toBe(i18n.t("terms.title"));
  });
});
