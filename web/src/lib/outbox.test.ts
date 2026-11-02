import { describe, it, expect } from "vitest";
import { Outbox, mergeMessages } from "./outbox";
import type { Message } from "./api";

const msg = (id: string, seq: number): Message => ({
  id, conversation_id: "c", sender_id: "u", seq, body: "b" + seq, created_at: "2026-11-02T00:00:00Z",
});

describe("Outbox", () => {
  it("add twice yields distinct ids, both pending", () => {
    const o = new Outbox();
    const a = o.add("c", "hi");
    const b = o.add("c", "hi");
    expect(a.client_msg_id).not.toBe(b.client_msg_id);
    expect(o.pending().map((m) => m.client_msg_id)).toEqual([a.client_msg_id, b.client_msg_id]);
  });
  it("ack removes from pending", () => {
    const o = new Outbox();
    const a = o.add("c", "hi");
    o.ack(a.client_msg_id, msg("m1", 1));
    expect(o.pending()).toEqual([]);
  });
  it("fail removes from pending", () => {
    const o = new Outbox();
    const a = o.add("c", "hi");
    o.fail(a.client_msg_id);
    expect(o.pending()).toEqual([]);
  });
});

describe("mergeMessages", () => {
  it("dedups by id and sorts by seq", () => {
    const out = mergeMessages([msg("a", 2), msg("b", 1)], [msg("a", 2), msg("c", 3)]);
    expect(out.map((m) => m.id)).toEqual(["b", "a", "c"]);
  });
});
