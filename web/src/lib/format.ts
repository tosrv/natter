import type { Message } from "./api";

export function statusOf(msg: Message, myId: string, peerLastReadSeq: number, pendingIds: Set<string>): "sending" | "sent" | "read" | null {
  if (msg.sender_id !== myId) return null;
  if (pendingIds.has(msg.id) || (msg.client_msg_id !== undefined && pendingIds.has(msg.client_msg_id))) return "sending";
  return msg.seq <= peerLastReadSeq ? "read" : "sent";
}

const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();

export function dayLabel(d: Date, now: Date, lang: "en" | "id"): string {
  const diff = Math.round((startOfDay(now) - startOfDay(d)) / 86400000);
  if (diff === 0) return lang === "id" ? "Hari ini" : "Today";
  if (diff === 1) return lang === "id" ? "Kemarin" : "Yesterday";
  return new Intl.DateTimeFormat(lang === "id" ? "id-ID" : "en-US", { day: "numeric", month: "long", year: "numeric" }).format(d);
}

const PALETTE = ["#e2552f", "#d9822b", "#b8a100", "#4f9d4f", "#1f9d8f", "#3b82c4", "#7a62d0", "#c2509a"];

export function avatarColor(id: string): string {
  let h = 0x811c9dc5;
  for (let i = 0; i < id.length; i++) { h ^= id.charCodeAt(i); h = Math.imul(h, 0x01000193) >>> 0; }
  return PALETTE[h % PALETTE.length];
}

export function validateDisplayName(s: string): { ok: true; value: string } | { ok: false } {
  const v = s.trim().replace(/\s+/g, " ");
  const n = [...v].length;
  if (n < 1 || n > 40 || !/^[\p{L}\p{M}\p{N} .\-'_]+$/u.test(v)) return { ok: false };
  return { ok: true, value: v };
}
