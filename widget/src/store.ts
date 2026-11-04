import type { Message } from "./types";

export interface OutboxItem { client_msg_id: string; body: string }
export interface State {
  messages: Message[];
  outbox: OutboxItem[];
  peerReadSeq: number;
  peerTyping: boolean;
  ended: boolean;
  online: boolean;
}

export type Event =
  | { t: "queue"; client_msg_id: string; body: string }
  | { t: "ack" | "message"; message: Message }
  | { t: "history"; messages: Message[] }
  | { t: "read"; seq: number }
  | { t: "typing"; on: boolean }
  | { t: "ended" }
  | { t: "online"; on: boolean };

export const initial = (): State => ({
  messages: [], outbox: [], peerReadSeq: 0, peerTyping: false, ended: false, online: true,
});

/** Merge by seq (and client_msg_id), sorted ascending; later copies replace earlier ones. */
function merge(existing: Message[], incoming: Message[]): Message[] {
  const bySeq = new Map<number, Message>();
  for (const m of existing) bySeq.set(m.seq, m);
  for (const m of incoming) bySeq.set(m.seq, m);
  const seenCid = new Set<string>();
  const out: Message[] = [];
  for (const m of [...bySeq.values()].sort((a, b) => a.seq - b.seq)) {
    if (m.client_msg_id) {
      if (seenCid.has(m.client_msg_id)) continue;
      seenCid.add(m.client_msg_id);
    }
    out.push(m);
  }
  return out;
}

const dropOutbox = (outbox: OutboxItem[], ms: Message[]): OutboxItem[] => {
  const ids = new Set(ms.map((m) => m.client_msg_id).filter(Boolean));
  return ids.size ? outbox.filter((o) => !ids.has(o.client_msg_id)) : outbox;
};

export function reduce(s: State, e: Event): State {
  switch (e.t) {
    case "queue":
      if (s.outbox.some((o) => o.client_msg_id === e.client_msg_id)) return s;
      if (s.messages.some((m) => m.client_msg_id === e.client_msg_id)) return s;
      return { ...s, outbox: [...s.outbox, { client_msg_id: e.client_msg_id, body: e.body }] };
    case "ack":
    case "message":
      return { ...s, messages: merge(s.messages, [e.message]), outbox: dropOutbox(s.outbox, [e.message]) };
    case "history":
      return { ...s, messages: merge(s.messages, e.messages), outbox: dropOutbox(s.outbox, e.messages) };
    case "read":
      return e.seq > s.peerReadSeq ? { ...s, peerReadSeq: e.seq } : s;
    case "typing":
      return { ...s, peerTyping: e.on };
    case "ended":
      return { ...s, ended: true };
    case "online":
      return { ...s, online: e.on };
  }
}

export const lastSeq = (s: State): number =>
  s.messages.length ? s.messages[s.messages.length - 1].seq : 0;
