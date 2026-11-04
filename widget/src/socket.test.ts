import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { createSocket } from "./socket";

class FakeWS {
  static instances: FakeWS[] = [];
  static OPEN = 1;
  readyState = 0;
  sent: string[] = [];
  onopen: (() => void) | null = null;
  onclose: ((e: { code: number }) => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  constructor(public url: string, public protocols: string[]) { FakeWS.instances.push(this); }
  send(d: string) { this.sent.push(d); }
  close() { this.readyState = 3; }
  open() { this.readyState = 1; this.onopen?.(); }
  drop(code = 1006) { this.readyState = 3; this.onclose?.({ code }); }
}

const mk = (extra: object = {}) => {
  const onClose = vi.fn(), onOpen = vi.fn(), onFrame = vi.fn();
  const s = createSocket({
    url: "ws://x/api/ws", token: () => "tok", onFrame, onOpen, onClose,
    WS: FakeWS as unknown as typeof WebSocket, random: () => 1, ...extra,
  });
  return { s, onClose, onOpen, onFrame };
};

beforeEach(() => { FakeWS.instances = []; vi.useFakeTimers(); });
afterEach(() => vi.useRealTimers());

describe("socket", () => {
  it("sends bearer subprotocols", () => {
    mk();
    expect(FakeWS.instances[0].protocols).toEqual(["pmess.bearer", "tok"]);
  });
  it("send returns false before open, true after", () => {
    const { s } = mk();
    expect(s.send({ type: "typing" })).toBe(false);
    FakeWS.instances[0].open();
    expect(s.send({ type: "typing" })).toBe(true);
    expect(FakeWS.instances[0].sent).toHaveLength(1);
  });
  it("reconnects with backoff, reports code", () => {
    const { onClose } = mk();
    FakeWS.instances[0].drop(1006);
    expect(onClose).toHaveBeenCalledWith(1006);
    vi.advanceTimersByTime(1000);
    expect(FakeWS.instances).toHaveLength(2);
    FakeWS.instances[1].drop();
    vi.advanceTimersByTime(1999);
    expect(FakeWS.instances).toHaveLength(2);
    vi.advanceTimersByTime(1);
    expect(FakeWS.instances).toHaveLength(3);
  });
  it("resets backoff after open", () => {
    mk();
    FakeWS.instances[0].drop();
    vi.advanceTimersByTime(1000);
    FakeWS.instances[1].open();
    FakeWS.instances[1].drop();
    vi.advanceTimersByTime(1000);
    expect(FakeWS.instances).toHaveLength(3);
  });
  it("does not reconnect after close()", () => {
    const { s } = mk();
    FakeWS.instances[0].open();
    s.close();
    FakeWS.instances[0].drop(1000);
    vi.advanceTimersByTime(60000);
    expect(FakeWS.instances).toHaveLength(1);
  });
  it("parses frames and ignores bad json", () => {
    const { onFrame } = mk();
    FakeWS.instances[0].onmessage?.({ data: '{"type":"ack"}' });
    FakeWS.instances[0].onmessage?.({ data: "nope" });
    expect(onFrame).toHaveBeenCalledTimes(1);
  });
});
