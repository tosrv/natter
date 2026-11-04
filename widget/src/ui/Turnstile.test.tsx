import { it, expect, vi } from "vitest";
import { render } from "preact";
import { act } from "preact/test-utils";
import { Turnstile } from "./Turnstile";

it("script load failure calls onError", async () => {
  const onError = vi.fn();
  const div = document.createElement("div");
  act(() => { render(<Turnstile siteKey="k" onToken={() => {}} onError={onError} />, div); });
  await new Promise((r) => setTimeout(r, 0));
  const s = document.head.querySelector('script[src*="turnstile"]')!;
  s.dispatchEvent(new Event("error"));
  await new Promise((r) => setTimeout(r, 0));
  expect(onError).toHaveBeenCalled();
});
