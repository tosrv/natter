import type { Message } from "./api";
import type { Outbox } from "./outbox";

export type ServerFrame =
  | { type: "ack"; client_msg_id: string; message: Message }
  | { type: "message"; message: Message }
  | { type: "typing"; conversation_id: string; user_id: string }
  | { type: "read"; conversation_id: string; user_id: string; seq: number }
  | { type: "conversation_deleted"; conversation_id: string }
  | { type: "conversation_cleared"; conversation_id: string }
  | { type: "conversation_ended"; conversation_id: string }
  | { type: "error"; code: string; message?: string; client_msg_id?: string };

export type ClientFrame =
  | { type: "send"; conversation_id: string; client_msg_id: string; body: string }
  | { type: "typing"; conversation_id: string }
  | { type: "read"; conversation_id: string; seq: number };

/** Reconnect delay: 1s, 2s, 4s … capped at 30s. */
export function backoffDelay(attempt: number): number {
  return Math.min(30000, 1000 * 2 ** attempt);
}

export function wsURL(): string {
  return `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/api/ws`;
}

type Handler = (f: ServerFrame) => void;

export class ChatSocket {
  private ws: WebSocket | null = null;
  private handlers = new Map<string, Set<Handler>>();
  private reconnectCbs = new Set<() => void>();
  private closeCbs = new Set<() => void>();
  private attempt = 0;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private closed = false;
  private everOpened = false;

  constructor(private outbox: Outbox, private url: string = wsURL()) {}

  connect(): void {
    this.closed = false;
    const ws = new WebSocket(this.url);
    this.ws = ws;
    ws.onopen = () => {
      const isReconnect = this.everOpened;
      this.everOpened = true;
      this.attempt = 0;
      for (const m of this.outbox.pending()) this.send({ type: "send", ...m });
      if (isReconnect) this.reconnectCbs.forEach((cb) => cb());
    };
    ws.onmessage = (ev) => {
      let f: ServerFrame;
      try { f = JSON.parse(String(ev.data)); } catch { return; }
      this.handlers.get(f.type)?.forEach((cb) => cb(f));
    };
    ws.onclose = () => {
      if (this.ws !== ws) return;
      this.ws = null;
      this.closeCbs.forEach((cb) => cb());
      if (this.closed) return;
      this.timer = setTimeout(() => this.connect(), backoffDelay(this.attempt++));
    };
  }

  /** Returns false when the socket is not open (frame dropped; sends stay in the outbox). */
  send(frame: ClientFrame): boolean {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return false;
    this.ws.send(JSON.stringify(frame));
    return true;
  }

  on(type: ServerFrame["type"], cb: Handler): () => void {
    if (!this.handlers.has(type)) this.handlers.set(type, new Set());
    this.handlers.get(type)!.add(cb);
    return () => this.handlers.get(type)!.delete(cb);
  }

  onReconnect(cb: () => void): () => void {
    this.reconnectCbs.add(cb);
    return () => this.reconnectCbs.delete(cb);
  }

  /** Called on every close (useful to check whether the session is still valid). */
  onClose(cb: () => void): () => void {
    this.closeCbs.add(cb);
    return () => this.closeCbs.delete(cb);
  }

  close(): void {
    this.closed = true;
    if (this.timer) clearTimeout(this.timer);
    const ws = this.ws;
    this.ws = null;
    ws?.close();
  }
}
