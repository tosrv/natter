import { describe, it, expect } from "vitest";
import { initial, reduce, lastSeq } from "./store";
import type { Message } from "./types";

const m = (seq: number, client_msg_id?: string): Message => ({
  id: `id${seq}`, conversation_id: "c1", sender_id: "u1", seq, body: `b${seq}`, client_msg_id, created_at: "t",
});

describe("store", () => {
  it("queue then ack empties outbox, one message", () => {
    let s = reduce(initial(), { t: "queue", client_msg_id: "x", body: "hi" });
    expect(s.outbox).toHaveLength(1);
    s = reduce(s, { t: "ack", message: m(1, "x") });
    expect(s.outbox).toHaveLength(0);
    expect(s.messages).toHaveLength(1);
  });
  it("resend after reconnect keeps one copy", () => {
    let s = reduce(initial(), { t: "queue", client_msg_id: "x", body: "hi" });
    s = reduce(s, { t: "history", messages: [m(1, "x")] });
    s = reduce(s, { t: "ack", message: m(1, "x") });
    expect(s.messages).toHaveLength(1);
    expect(s.outbox).toHaveLength(0);
  });
  it("sorts by seq", () => {
    let s = reduce(initial(), { t: "message", message: m(3) });
    s = reduce(s, { t: "message", message: m(2) });
    expect(s.messages.map((x) => x.seq)).toEqual([2, 3]);
    expect(lastSeq(s)).toBe(3);
    expect(lastSeq(initial())).toBe(0);
  });
  it("read never decreases", () => {
    let s = reduce(initial(), { t: "read", seq: 5 });
    s = reduce(s, { t: "read", seq: 4 });
    expect(s.peerReadSeq).toBe(5);
  });
  it("ended, typing, online", () => {
    let s = reduce(initial(), { t: "ended" });
    expect(s.ended).toBe(true);
    s = reduce(s, { t: "typing", on: true });
    expect(s.peerTyping).toBe(true);
    s = reduce(s, { t: "online", on: false });
    expect(s.online).toBe(false);
  });
  it("queue dedups client_msg_id", () => {
    let s = reduce(initial(), { t: "queue", client_msg_id: "x", body: "hi" });
    s = reduce(s, { t: "queue", client_msg_id: "x", body: "hi" });
    expect(s.outbox).toHaveLength(1);
  });
});
