import { describe, it, expect } from "vitest";
import { createRoot } from "react-dom/client";
import { act } from "react-dom/test-utils";
import ProductShot from "./ProductShot";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

function render(el: React.ReactElement) {
  const host = document.createElement("div");
  document.body.appendChild(host);
  act(() => createRoot(host).render(el));
  return host;
}

describe("ProductShot", () => {
  it("renders light and dark pictures with sized, prioritized images", () => {
    const host = render(<ProductShot variant="desktop" priority />);
    const pics = host.querySelectorAll("picture");
    expect(pics).toHaveLength(2);
    expect(pics[0].className).toContain("dark:hidden");
    expect(pics[1].className).toContain("hidden");
    expect(pics[1].className).toContain("dark:block");
    for (const p of pics) {
      expect(p.querySelector('source[type="image/avif"]')).not.toBeNull();
      expect(p.querySelector('source[type="image/webp"]')).not.toBeNull();
      const img = p.querySelector("img")!;
      expect(Number(img.getAttribute("width"))).toBeGreaterThan(0);
      expect(Number(img.getAttribute("height"))).toBeGreaterThan(0);
      expect(img.getAttribute("alt")).toBeTruthy();
    }
    const [light, dark] = [...pics].map((p) => p.querySelector("img")!);
    expect(light.getAttribute("loading")).toBe("eager");
    expect(light.getAttribute("fetchpriority")).toBe("high");
    expect(dark.getAttribute("loading")).toBe("lazy");
    expect(dark.hasAttribute("fetchpriority")).toBe(false);
  });
  it("hero art-directs desktop and mobile sources by media query", () => {
    const pic = render(<ProductShot variant="hero" priority />).querySelector("picture")!;
    const media = [...pic.querySelectorAll("source")].map((s) => `${s.getAttribute("type")} ${s.getAttribute("media")} ${s.getAttribute("srcset")?.split(" ")[0]}`);
    expect(media).toContain("image/avif (min-width: 768px) /product/desktop-light.avif");
    expect(media).toContain("image/avif (max-width: 767px) /product/mobile-light.avif");
  });
  it("lazy loads without priority", () => {
    const img = render(<ProductShot variant="receipts" />).querySelector("img")!;
    expect(img.getAttribute("loading")).toBe("lazy");
    expect(img.hasAttribute("fetchpriority")).toBe(false);
  });
});
