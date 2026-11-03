import { describe, expect, it } from "vitest";
import { removeConversation, setBlocked, type ChatState } from "./useChat";
import type { ConversationSummary, Message } from "@/lib/api";

const conv = (id: string) => ({ id, peer_id: "p" + id, peer_username: "u" + id, peer_display_name: "U" + id, peer_kind: "user",
  last_message: null, unread: 0, peer_last_read_seq: 0, last_activity_at: "" }) as ConversationSummary;
const msg = (c: string) => ({ id: "m" + c, conversation_id: c, sender_id: "x", seq: 1, body: "b", created_at: "" }) as Message;

describe("removeConversation", () => {
  const base: ChatState = { convs: [conv("a"), conv("b")], msgs: { a: [msg("a")], b: [msg("b")] }, activeId: "a" };
  it("drops the conversation and its messages and clears activeId when active", () => {
    const s = removeConversation(base, "a");
    expect(s.convs.map((c) => c.id)).toEqual(["b"]);
    expect(s.msgs.a).toBeUndefined();
    expect(s.activeId).toBeNull();
  });
  it("leaves other conversations untouched", () => {
    const s = removeConversation(base, "b");
    expect(s.convs).toEqual([base.convs[0]]);
    expect(s.msgs.a).toBe(base.msgs.a);
    expect(s.activeId).toBe("a");
  });
});

describe("setBlocked", () => {
  const base: ChatState = { convs: [conv("a"), conv("b")], msgs: {}, activeId: "a" };
  it("marks only the matching conversation by id", () => {
    const s = setBlocked(base, "a", true);
    expect(s.convs[0].blocked_by_me).toBe(true);
    expect(s.convs[1]).toBe(base.convs[1]);
  });
  it("matches by peer id and can unblock", () => {
    const s = setBlocked(setBlocked(base, "pb", true), "pb", false);
    expect(s.convs[1].blocked_by_me).toBe(false);
    expect(s.convs[0]).toBe(base.convs[0]);
  });
});
