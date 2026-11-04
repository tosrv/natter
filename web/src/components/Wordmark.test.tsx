import { describe, it, expect } from "vitest";
import { createRoot } from "react-dom/client";
import { act } from "react-dom/test-utils";
import { Wordmark } from "./Wordmark";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe("Wordmark", () => {
  it("is an accessible image named natter", () => {
    const host = document.createElement("div");
    document.body.appendChild(host);
    act(() => createRoot(host).render(<Wordmark className="h-6" />));
    const el = host.querySelector('svg[role="img"]')!;
    expect(el.getAttribute("aria-label")).toBe("natter");
    expect(el.getAttribute("class")).toContain("h-6");
  });
});
