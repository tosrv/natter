import { describe, it, expect } from "vitest";
import { statusOf, dayLabel, avatarColor, validateDisplayName } from "./format";
import type { Message } from "./api";

const m = (o: Partial<Message>): Message => ({ id: "1", conversation_id: "c", sender_id: "me", seq: 5, body: "x", created_at: "", ...o });

describe("statusOf", () => {
  it("pending -> sending", () => expect(statusOf(m({}), "me", 0, new Set(["1"]))).toBe("sending"));
  it("sent", () => expect(statusOf(m({}), "me", 4, new Set())).toBe("sent"));
  it("read", () => expect(statusOf(m({}), "me", 5, new Set())).toBe("read"));
  it("pending by client_msg_id", () => expect(statusOf(m({ client_msg_id: "k" }), "me", 0, new Set(["k"]))).toBe("sending"));
  it("others -> null", () => expect(statusOf(m({ sender_id: "x" }), "me", 9, new Set())).toBeNull());
});

describe("dayLabel", () => {
  const now = new Date(2026, 10, 2, 10, 0);
  it("today", () => {
    expect(dayLabel(new Date(2026, 10, 2, 0, 5), now, "id")).toBe("Hari ini");
    expect(dayLabel(new Date(2026, 10, 2, 23, 59), now, "en")).toBe("Today");
  });
  it("yesterday", () => {
    expect(dayLabel(new Date(2026, 10, 1, 23, 59), now, "id")).toBe("Kemarin");
    expect(dayLabel(new Date(2026, 10, 1, 1, 0), now, "en")).toBe("Yesterday");
  });
  it("other dates", () => {
    expect(dayLabel(new Date(2026, 7, 17), now, "id")).toContain("Agustus");
    expect(dayLabel(new Date(2026, 7, 17), now, "en")).toContain("August");
  });
});

describe("avatarColor", () => {
  it("deterministic, from 8 colors", () => {
    expect(avatarColor("abc")).toBe(avatarColor("abc"));
    const s = new Set<string>();
    for (let i = 0; i < 500; i++) s.add(avatarColor("user-" + i));
    expect(s.size).toBe(8);
    for (const c of s) expect(c).toMatch(/^#[0-9a-f]{6}$/i);
  });
});

describe("validateDisplayName", () => {
  it("accepts and normalizes", () => {
    expect(validateDisplayName("  Budi   Santoso ")).toEqual({ ok: true, value: "Budi Santoso" });
    expect(validateDisplayName("O'Neil-Jr._1")).toEqual({ ok: true, value: "O'Neil-Jr._1" });
    expect(validateDisplayName("Zoë 李")).toEqual({ ok: true, value: "Zoë 李" });
  });
  it("rejects", () => {
    for (const s of ["", "   ", "Hi 😀", "a@b", "a<b>", "a\tb\u0000", "x".repeat(41)]) expect(validateDisplayName(s).ok).toBe(false);
  });
  it("counts code points", () => {
    expect(validateDisplayName("𝐀".repeat(40)).ok).toBe(true);
    expect(validateDisplayName("𝐀".repeat(41)).ok).toBe(false);
  });
});
