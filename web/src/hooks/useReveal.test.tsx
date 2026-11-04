import { describe, it, expect, afterEach, vi } from "vitest";
import { createRoot } from "react-dom/client";
import { act } from "react-dom/test-utils";
import { useReveal } from "./useReveal";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

function Probe() {
  const ref = useReveal<HTMLDivElement>();
  return <div ref={ref} data-testid="p" className="reveal" />;
}
function render() {
  const host = document.createElement("div");
  document.body.appendChild(host);
  act(() => createRoot(host).render(<Probe />));
  return host.firstElementChild as HTMLElement;
}

afterEach(() => { vi.unstubAllGlobals(); document.body.innerHTML = ""; });

describe("useReveal", () => {
  it("is visible immediately without IntersectionObserver", () => {
    vi.stubGlobal("IntersectionObserver", undefined);
    expect(render().classList.contains("is-visible")).toBe(true);
  });

  it("adds is-visible once when intersecting, then disconnects", () => {
    let cb: IntersectionObserverCallback = () => {};
    const disconnect = vi.fn();
    vi.stubGlobal("IntersectionObserver", class {
      constructor(c: IntersectionObserverCallback) { cb = c; }
      observe() {}
      disconnect = disconnect;
    });
    const el = render();
    expect(el.classList.contains("is-visible")).toBe(false);
    act(() => cb([{ isIntersecting: true, target: el } as unknown as IntersectionObserverEntry], {} as IntersectionObserver));
    expect(el.classList.contains("is-visible")).toBe(true);
    expect(disconnect).toHaveBeenCalled();
  });
});
