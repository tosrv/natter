import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { createRoot, type Root } from "react-dom/client";
import { act } from "react-dom/test-utils";
import "@/i18n";
import ChatHeader from "./ChatHeader";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
let root: Root;
let host: HTMLDivElement;
beforeEach(() => { host = document.createElement("div"); document.body.appendChild(host); root = createRoot(host); });
afterEach(() => { act(() => root.unmount()); host.remove(); document.body.innerHTML = ""; });

const byText = (t: string) => Array.from(document.body.querySelectorAll<HTMLElement>("[role=menuitem],button")).find((e) => e.textContent?.trim() === t);
async function openMenu() {
  const trigger = document.body.querySelector<HTMLElement>('button[aria-label="Conversation menu"]')!;
  await act(async () => {
    trigger.dispatchEvent(new MouseEvent("pointerdown", { bubbles: true, button: 0 }));
    trigger.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  });
}
const render = (onDelete: () => void) => act(() => {
  root.render(<ChatHeader peerId="p" name="Guest" username="" typing={false} onDelete={onDelete} />);
});

describe("ChatHeader delete chat", () => {
  it("shows Delete chat (no Block) when only onDelete is given", async () => {
    render(vi.fn()); await openMenu();
    expect(byText("Delete chat")).toBeTruthy();
    expect(byText("Block")).toBeUndefined();
  });
  it("Cancel does not call onDelete; Delete calls it once", async () => {
    const onDelete = vi.fn();
    render(onDelete); await openMenu();
    await act(async () => { byText("Delete chat")!.click(); });
    expect(document.body.textContent).toContain("Delete this chat?");
    await act(async () => { byText("Cancel")!.click(); });
    expect(onDelete).not.toHaveBeenCalled();
    await openMenu();
    await act(async () => { byText("Delete chat")!.click(); });
    await act(async () => { byText("Delete")!.click(); });
    expect(onDelete).toHaveBeenCalledTimes(1);
  });
});
