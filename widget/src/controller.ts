import type { createApi } from "./api";
import { initial, reduce, lastSeq, type State, type Event } from "./store";
import type { SocketOpts } from "./socket";

export type Phase = "idle" | "loading" | "turnstile" | "chat" | "ended" | "full" | "unavailable";
export interface ViewState { phase: Phase; chat: State; siteKey?: string; peerId?: string }

type Api = ReturnType<typeof createApi>;
interface Session { get(): string | null; set(t: string): void; clear(): void }
interface Sock { send(f: object): boolean; close(): void }
export interface Deps {
  api: Api;
  session: Session;
  socketFactory: (o: Omit<SocketOpts, "url" | "WS" | "random">) => Sock;
}

const PAGE = 100;
const uuid = () =>
  typeof crypto !== "undefined" && crypto.randomUUID
    ? crypto.randomUUID()
    : "10000000-1000-4000-8000-100000000000".replace(/[018]/g, (c) =>
        (+c ^ (Math.random() * 16) >> (+c / 4)).toString(16));

export function createController({ api, session, socketFactory }: Deps) {
  let vs: ViewState = { phase: "idle", chat: initial() };
  let conv: string | null = null;
  let sock: Sock | null = null;
  let isOpen = false;
  let lastRead = 0;
  const subs = new Set<() => void>();

  const set = (p: Partial<ViewState>) => { vs = { ...vs, ...p }; subs.forEach((f) => f()); };
  const ev = (e: Event) => { set({ chat: reduce(vs.chat, e) }); markRead(); };

  function markRead() {
    if (!isOpen || !sock || !conv || vs.phase !== "chat") return;
    const inc = vs.chat.messages.filter((m) => m.sender_id === vs.peerId);
    const seq = inc.length ? inc[inc.length - 1].seq : 0;
    if (seq > lastRead && sock.send({ type: "read", conversation_id: conv, seq })) lastRead = seq;
  }

  function end() {
    session.clear();
    sock?.close();
    sock = null;
    set({ phase: "ended", chat: reduce(vs.chat, { t: "ended" }) });
  }

  function fail(err: unknown) {
    const e = err as { status?: number; code?: string };
    if (e?.code === "limit_reached" || e?.status === 429) return set({ phase: "full" });
    if (e?.status === 401) return end();
    console.warn("[pmess]", err);
    set({ phase: "unavailable" });
  }

  async function catchUp() {
    if (!conv) return;
    for (;;) {
      const ms = await api.messagesAfter(conv, lastSeq(vs.chat), PAGE);
      if (ms.length) ev({ t: "history", messages: ms });
      if (ms.length < PAGE) break;
    }
  }

  const sendItem = (o: { client_msg_id: string; body: string }) =>
    conv != null && sock?.send({ type: "send", conversation_id: conv, ...o });

  function onFrame(f: any) {
    if (!f || (f.conversation_id && conv && f.conversation_id !== conv)) return;
    switch (f.type) {
      case "ack": return f.message && ev({ t: "ack", message: f.message });
      case "message": return f.message && ev({ t: "message", message: f.message });
      case "typing": return typingOn();
      case "read": return ev({ t: "read", seq: f.seq });
      case "conversation_ended": case "conversation_deleted": return end();
      case "error": if (f.code === "peer_ended") end(); return;
    }
  }

  let typingTimer: ReturnType<typeof setTimeout> | undefined;
  function typingOn() {
    ev({ t: "typing", on: true });
    clearTimeout(typingTimer);
    typingTimer = setTimeout(() => ev({ t: "typing", on: false }), 5000);
  }

  async function startChat(convId: string, peerId: string | undefined, peerRead: number) {
    conv = convId;
    lastRead = 0;
    let chat = initial();
    chat = reduce(chat, { t: "read", seq: peerRead });
    set({ phase: "chat", chat, peerId });
    await catchUp();
    sock?.close();
    sock = socketFactory({
      token: () => session.get(),
      onFrame,
      onOpen: () => {
        ev({ t: "online", on: true });
        catchUp().then(() => { vs.chat.outbox.forEach(sendItem); markRead(); }, fail);
      },
      onClose: () => ev({ t: "online", on: false }),
    });
  }

  async function toTurnstile() {
    set({ phase: "loading", chat: initial() });
    const cfg = await api.config();
    set({ phase: "turnstile", siteKey: cfg.turnstile_site_key });
  }

  return {
    state: () => vs,
    subscribe(cb: () => void) { subs.add(cb); return () => { subs.delete(cb); }; },
    async open() {
      isOpen = true;
      if (vs.phase !== "idle") { markRead(); return; }
      set({ phase: "loading" });
      try {
        if (session.get()) {
          try {
            const cs = await api.conversations();
            const c = cs[0];
            if (c) {
              if (c.peer_ended) { end(); return; }
              await startChat(c.id, c.peer_id, c.peer_last_read_seq);
              return;
            }
            session.clear();
          } catch (e) {
            if ((e as { status?: number }).status !== 401) throw e;
            session.clear();
          }
        }
        await toTurnstile();
      } catch (e) { fail(e); }
    },
    close() { isOpen = false; },
    async startGuest(turnstileToken: string) {
      set({ phase: "loading" });
      try {
        const g = await api.createGuest(turnstileToken);
        session.set(g.token);
        let peer: string | undefined;
        let read = 0;
        try {
          const c = (await api.conversations()).find((x) => x.id === g.conversation_id);
          peer = c?.peer_id; read = c?.peer_last_read_seq ?? 0;
        } catch { /* fall back to sender id comparison */ }
        await startChat(g.conversation_id, peer, read);
      } catch (e) { fail(e); }
    },
    send(body: string) {
      const b = body.trim();
      if (!b || vs.phase !== "chat") return;
      const item = { client_msg_id: uuid(), body: b };
      ev({ t: "queue", ...item });
      sendItem(item);
    },
    typing() { if (conv) sock?.send({ type: "typing", conversation_id: conv }); },
    restart() {
      sock?.close(); sock = null; conv = null;
      session.clear();
      toTurnstile().catch(fail);
    },
  };
}
export type Controller = ReturnType<typeof createController>;
