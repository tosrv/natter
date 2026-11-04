import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { createRoot, type Root } from "react-dom/client";
import { act } from "react-dom/test-utils";
import "@/i18n";

const createGuest = vi.fn();
const navigate = vi.fn();
const resetSpy = vi.fn();

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return { ...actual, createGuest: (...a: unknown[]) => createGuest(...a) };
});
vi.mock("react-router-dom", () => ({ useNavigate: () => navigate }));
vi.mock("@/components/TurnstileWidget", () => ({
  default: ({ onToken, onReady }: { onToken: (t: string | null) => void; onReady?: (r: (() => void) | null) => void }) => {
    onReady?.(resetSpy);
    return <button data-testid="ts" onClick={() => onToken("tok")} />;
  },
}));

import GuestStartDialog from "./GuestStartDialog";
import { ApiError } from "@/lib/api";

let root: Root;
let host: HTMLDivElement;
beforeEach(() => {
  createGuest.mockReset(); navigate.mockReset(); resetSpy.mockReset();
  host = document.createElement("div");
  document.body.appendChild(host);
  root = createRoot(host);
});
afterEach(() => { act(() => root.unmount()); host.remove(); document.body.innerHTML = ""; });

const q = <T extends Element>(sel: string) => document.body.querySelector<T>(sel);
const setVal = (el: HTMLInputElement | HTMLTextAreaElement, v: string) => {
  const proto = el instanceof HTMLInputElement ? HTMLInputElement.prototype : HTMLTextAreaElement.prototype;
  Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(el, v);
  act(() => { el.dispatchEvent(new Event("input", { bubbles: true })); });
};
const startBtn = () => [...document.body.querySelectorAll("button")].find((b) => b.textContent === "Start chat") as HTMLButtonElement;
const render = (siteKey = "k") => act(() => { root.render(<GuestStartDialog open onClose={() => {}} siteKey={siteKey} />); });
const nameInput = () => q<HTMLInputElement>("input")!;
const contactInput = () => q<HTMLTextAreaElement>("textarea")!;
const flush = () => act(async () => { await Promise.resolve(); });

describe("GuestStartDialog", () => {
  it("opens with a random name", () => {
    render();
    expect(nameInput().value).toMatch(/^natter-[a-z0-9]{4}$/);
  });
  it("disables start when name is empty", () => {
    render();
    act(() => q<HTMLButtonElement>("[data-testid=ts]")!.click());
    expect(startBtn().disabled).toBe(false);
    setVal(nameInput(), "   ");
    expect(startBtn().disabled).toBe(true);
  });
  it("submits createGuest(name, token) and navigates with contact", async () => {
    createGuest.mockResolvedValue({});
    render();
    setVal(nameInput(), "Budi");
    setVal(contactInput(), " budi@x.id ");
    act(() => q<HTMLButtonElement>("[data-testid=ts]")!.click());
    act(() => startBtn().click());
    await flush();
    expect(createGuest).toHaveBeenCalledWith("Budi", "tok");
    expect(navigate).toHaveBeenCalledWith("/chat", { state: { guestContact: "budi@x.id" } });
  });
  it("whitespace contact is not sent", async () => {
    createGuest.mockResolvedValue({});
    render();
    setVal(nameInput(), "Budi");
    setVal(contactInput(), "   ");
    act(() => q<HTMLButtonElement>("[data-testid=ts]")!.click());
    act(() => startBtn().click());
    await flush();
    expect(navigate).toHaveBeenCalledWith("/chat", { state: undefined });
  });
  it("needs no token when Turnstile is disabled", async () => {
    createGuest.mockResolvedValue({});
    render("");
    expect(q("[data-testid=ts]")).toBeNull();
    act(() => startBtn().click());
    await flush();
    expect(createGuest).toHaveBeenCalledWith(expect.stringMatching(/^natter-/), undefined);
  });
  it("invalid name resets turnstile and shows name error", async () => {
    createGuest.mockRejectedValue(new ApiError("display_name_invalid", 400));
    render();
    act(() => q<HTMLButtonElement>("[data-testid=ts]")!.click());
    act(() => startBtn().click());
    await flush();
    expect(resetSpy).toHaveBeenCalled();
    expect(document.body.textContent).toContain("Use 1–40 letters, numbers, spaces or . - ' _");
    expect(navigate).not.toHaveBeenCalled();
  });
});
