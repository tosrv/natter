import type { Message } from "./api";

export interface ClientMsg {
  client_msg_id: string;
  conversation_id: string;
  body: string;
}

/** Messages sent but not yet acknowledged; resent with the same id on reconnect. */
export class Outbox {
  private items = new Map<string, ClientMsg>();

  add(convId: string, body: string): ClientMsg {
    const m: ClientMsg = { client_msg_id: crypto.randomUUID(), conversation_id: convId, body };
    this.items.set(m.client_msg_id, m);
    return m;
  }

  ack(clientMsgId: string, _message: Message): void {
    this.items.delete(clientMsgId);
  }

  /** Drop a message the server rejected permanently. */
  fail(clientMsgId: string): void {
    this.items.delete(clientMsgId);
  }

  pending(): ClientMsg[] {
    return [...this.items.values()];
  }
}

/** Merge incoming messages into list: dedup by id, ordered by seq. */
export function mergeMessages(list: Message[], incoming: Message[]): Message[] {
  const byId = new Map<string, Message>();
  for (const m of list) byId.set(m.id, m);
  for (const m of incoming) byId.set(m.id, m);
  return [...byId.values()].sort((a, b) => a.seq - b.seq);
}
