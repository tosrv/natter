// @vitest-environment node
import { describe, expect, it } from "vitest";
import { render } from "./entry-prerender";

describe("prerender render()", () => {
  it("renders the landing page without browser globals", () => {
    expect(typeof window).toBe("undefined");
    const html = render("/");
    expect(html).toContain("Conversations, made simple.");
    expect(html).toContain("Continue as guest");
  });
  it("renders the privacy page", () => {
    expect(render("/privacy")).toContain("Privacy Policy");
  });
  it("renders the terms page", () => {
    expect(render("/terms")).toContain("<h1");
  });
});
