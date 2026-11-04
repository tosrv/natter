import { it, expect, vi } from "vitest";
import { render } from "preact";
import { act } from "preact/test-utils";
import { GuestForm } from "./GuestForm";

function mount(startGuest = vi.fn(async () => {}), nameError = false) {
  const div = document.createElement("div");
  document.body.appendChild(div);
  const ctl: any = { startGuest, fail: vi.fn(), state: () => ({ phase: "chat" }) };
  act(() => { render(<GuestForm ctl={ctl} lang="en" siteKey="k" nameError={nameError} />, div); });
  return { div, startGuest };
}
const btn = (d: HTMLElement) => d.querySelector("button[type=submit]") as HTMLButtonElement;
const nameIn = (d: HTMLElement) => d.querySelector("input") as HTMLInputElement;

it("prefills random name", () => {
  const { div } = mount();
  expect(nameIn(div).value).toMatch(/^natter-[a-z0-9]{4}$/);
});
it("disabled without turnstile token", () => {
  const { div } = mount();
  expect(btn(div).disabled).toBe(true);
});
it("empty name disables; token enables; click starts guest", async () => {
  // simulate turnstile callback
  const render_ = vi.fn((_el: any, o: any) => { o.callback("TOK"); return "id"; });
  (window as any).turnstile = { render: render_, reset: vi.fn(), remove: vi.fn() };
  const { div: d2, startGuest: sg2 } = mount();
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  const inp = nameIn(d2);
  expect(btn(d2).disabled).toBe(false);
  await act(async () => { inp.value = " "; inp.dispatchEvent(new Event("input", { bubbles: true })); });
  expect(btn(d2).disabled).toBe(true);
  await act(async () => { inp.value = "Budi"; inp.dispatchEvent(new Event("input", { bubbles: true })); });
  const ta = d2.querySelector("textarea") as HTMLTextAreaElement;
  await act(async () => { ta.value = "wa 1"; ta.dispatchEvent(new Event("input", { bubbles: true })); });
  await act(async () => { btn(d2).click(); });
  expect(sg2).toHaveBeenCalledWith("TOK", "Budi", "wa 1");
  delete (window as any).turnstile;
});
