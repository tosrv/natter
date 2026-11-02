import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import LanguageToggle from "@/components/LanguageToggle";
import ConversationList from "@/components/ConversationList";
import MessageView from "@/components/MessageView";
import Composer from "@/components/Composer";
import UserSearch from "@/components/UserSearch";
import GuestContactDialog from "@/components/GuestContactDialog";
import {
  ApiError, getMe, listConversations, listMessages, logout, openConversation,
  type ConversationSummary, type Me, type Message, type UserHit,
} from "@/lib/api";
import { Outbox, mergeMessages, type ClientMsg } from "@/lib/outbox";
import { ChatSocket, type ServerFrame } from "@/lib/socket";
import { fetchMissing } from "@/lib/paging";
import { errorMessage } from "@/i18n";
import { cn } from "@/lib/utils";

const TYPING_MS = 5000;
const RATE_RETRY_BASE_MS = 10000;
const MAX_RATE_RETRIES = 3;

export default function Chat() {
  const { t } = useTranslation();
  const nav = useNavigate();
  const location = useLocation();
  const [me, setMe] = useState<Me | null>(null);
  const [convs, setConvs] = useState<ConversationSummary[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [msgs, setMsgs] = useState<Record<string, Message[]>>({});
  const [pending, setPending] = useState<ClientMsg[]>([]);
  const [typingUntil, setTypingUntil] = useState<Record<string, number>>({});
  const [online, setOnline] = useState(true);
  const [contactOpen, setContactOpen] = useState(false);
  const [, tick] = useState(0);
  const [bootError, setBootError] = useState(false);
  const [bootAttempt, setBootAttempt] = useState(0);
  const rateRetries = useRef<Record<string, number>>({});

  const outbox = useRef(new Outbox());
  const sock = useRef<ChatSocket | null>(null);
  const activeRef = useRef<string | null>(null);
  const msgsRef = useRef(msgs);
  const convsRef = useRef(convs);
  const lastReadSent = useRef<Record<string, number>>({});
  const historyLoaded = useRef<Record<string, boolean>>({});
  const typingTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  activeRef.current = activeId;
  msgsRef.current = msgs;
  convsRef.current = convs;

  const toLanding = useCallback(() => nav("/", { replace: true }), [nav]);
  const handleErr = useCallback((e: unknown) => {
    if (e instanceof ApiError && e.status === 401) return toLanding();
    toast.error(errorMessage(e instanceof ApiError ? e.code : "internal"));
  }, [toLanding]);

  const addMessages = useCallback((convId: string, incoming: Message[]) => {
    setMsgs((prev) => ({ ...prev, [convId]: mergeMessages(prev[convId] ?? [], incoming) }));
  }, []);

  const refreshConvs = useCallback(() => listConversations().then((c) => { setConvs(c); return c; }), []);

  const loadMessages = useCallback((convId: string) => {
    const state = { historyLoaded: !!historyLoaded.current[convId], messages: msgsRef.current[convId] ?? [] };
    return fetchMissing((p) => listMessages(convId, { ...p, limit: 100 }).then((r) => r.messages), state, 100)
      .then((ms) => { historyLoaded.current[convId] = true; addMessages(convId, ms); });
  }, [addMessages]);

  // Bootstrap: profile, conversations, socket.
  useEffect(() => {
    let alive = true;
    const newGuest = (location.state as { newGuest?: boolean } | null)?.newGuest === true;
    setBootError(false);
    getMe().then(async (m) => {
      if (!alive) return;
      if (!m.username) { nav("/onboarding", { replace: true }); return; }
      setMe(m);
      const c = await refreshConvs();
      if (!alive) return;
      if (m.kind === "guest" && c[0]) setActiveId(c[0].id);
      if (m.kind === "guest" && newGuest) {
        setContactOpen(true);
        nav(".", { replace: true, state: null }); // show the dialog only once
      }
      const s = new ChatSocket(outbox.current);
      sock.current = s;
      s.on("ack", (f) => {
        const a = f as Extract<ServerFrame, { type: "ack" }>;
        outbox.current.ack(a.client_msg_id, a.message);
        delete rateRetries.current[a.client_msg_id];
        setPending(outbox.current.pending());
        addMessages(a.message.conversation_id, [a.message]);
        setConvs((cs) => cs.map((x) => x.id === a.message.conversation_id ? { ...x, last_message: a.message } : x));
      });
      s.on("message", (f) => {
        const msg = (f as Extract<ServerFrame, { type: "message" }>).message;
        addMessages(msg.conversation_id, [msg]);
        setTypingUntil((tu) => ({ ...tu, [msg.conversation_id]: 0 }));
        const known = convsRef.current.find((x) => x.id === msg.conversation_id);
        if (!known) { void refreshConvs().catch(handleErr); }
        const viewing = activeRef.current === msg.conversation_id && document.visibilityState === "visible";
        setConvs((cs) => cs.map((x) => x.id === msg.conversation_id
          ? { ...x, last_message: msg, unread: viewing ? x.unread : x.unread + 1 } : x));
        if (activeRef.current !== msg.conversation_id) {
          toast(t("chat.newMessage", { name: known?.peer_username ?? "" }), { description: msg.body.slice(0, 100) });
        }
      });
      s.on("typing", (f) => {
        const ty = f as Extract<ServerFrame, { type: "typing" }>;
        setTypingUntil((tu) => ({ ...tu, [ty.conversation_id]: Date.now() + TYPING_MS }));
        clearTimeout(typingTimers.current[ty.conversation_id]);
        typingTimers.current[ty.conversation_id] = setTimeout(() => tick((n) => n + 1), TYPING_MS + 50);
      });
      s.on("read", (f) => {
        const r = f as Extract<ServerFrame, { type: "read" }>;
        if (r.user_id === m.id) return;
        setConvs((cs) => cs.map((x) => x.id === r.conversation_id
          ? { ...x, peer_last_read_seq: Math.max(x.peer_last_read_seq, r.seq) } : x));
      });
      s.on("error", (f) => {
        const e = f as Extract<ServerFrame, { type: "error" }>;
        if (e.client_msg_id && e.code === "rate_limited") {
          // Keep it in the outbox; retry with backoff 10s, 20s, 40s (also resent
          // on reconnect), then give up and mark it failed.
          const id = e.client_msg_id;
          const n = rateRetries.current[id] ?? 0;
          if (n >= MAX_RATE_RETRIES) {
            delete rateRetries.current[id];
            outbox.current.fail(id);
            setPending(outbox.current.pending());
            toast.error(t("chat.sendFailed"));
            return;
          }
          rateRetries.current[id] = n + 1;
          setTimeout(() => {
            const m = outbox.current.pending().find((x) => x.client_msg_id === id);
            if (m) sock.current?.send({ type: "send", ...m });
          }, RATE_RETRY_BASE_MS * 2 ** n);
        } else if (e.client_msg_id) { outbox.current.fail(e.client_msg_id); setPending(outbox.current.pending()); }
        toast.error(errorMessage(e.code));
      });
      s.onReconnect(() => {
        setOnline(true);
        if (activeRef.current) void loadMessages(activeRef.current).catch(handleErr);
        void refreshConvs().catch(handleErr);
      });
      s.onClose(() => {
        setOnline(false);
        getMe().catch((e) => { if (e instanceof ApiError && e.status === 401) { s.close(); toLanding(); } });
      });
      s.connect();
    }).catch((e) => {
      if (!alive) return;
      if (e instanceof ApiError && e.status === 401) toLanding(); else setBootError(true);
    });
    return () => { alive = false; Object.values(typingTimers.current).forEach(clearTimeout); sock.current?.close(); sock.current = null; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bootAttempt]);

  // Load history when a conversation opens.
  useEffect(() => {
    if (activeId) void loadMessages(activeId).catch(handleErr);
  }, [activeId, loadMessages, handleErr]);

  // Send "read" while the conversation is open and the tab is visible.
  const activeMsgs = useMemo(() => (activeId ? msgs[activeId] ?? [] : []), [activeId, msgs]);
  const markRead = useCallback(() => {
    const id = activeRef.current;
    if (!id || document.visibilityState !== "visible") return;
    const list = msgsRef.current[id] ?? [];
    const seq = list.length ? list[list.length - 1].seq : 0;
    if (seq <= (lastReadSent.current[id] ?? 0)) return;
    if (sock.current?.send({ type: "read", conversation_id: id, seq })) {
      lastReadSent.current[id] = seq;
      setConvs((cs) => cs.map((x) => x.id === id ? { ...x, unread: 0 } : x));
    }
  }, []);
  useEffect(() => { markRead(); }, [activeMsgs, online, markRead]);
  useEffect(() => {
    document.addEventListener("visibilitychange", markRead);
    return () => document.removeEventListener("visibilitychange", markRead);
  }, [markRead]);

  function send(convId: string, body: string) {
    const m = outbox.current.add(convId, body);
    setPending(outbox.current.pending());
    sock.current?.send({ type: "send", ...m }); // if offline, resent on reconnect
  }

  async function pickUser(u: UserHit) {
    try {
      const { id } = await openConversation(u.id);
      if (!convsRef.current.some((c) => c.id === id)) await refreshConvs();
      setActiveId(id);
    } catch (e) { handleErr(e); }
  }

  async function out() {
    sock.current?.close();
    try { await logout(); } catch { /* session may already be gone */ }
    toLanding();
  }

  if (!me && bootError) {
    return (
      <div className="flex flex-col items-start gap-3 p-6" role="alert">
        <p className="text-muted-foreground">{t("chat.loadFailed")}</p>
        <Button onClick={() => setBootAttempt((n) => n + 1)}>{t("common.retry")}</Button>
      </div>
    );
  }
  if (!me) return <p className="p-6 text-muted-foreground">{t("common.loading")}</p>;

  const isGuest = me.kind === "guest";
  const active = convs.find((c) => c.id === activeId) ?? null;
  const typing = !!activeId && (typingUntil[activeId] ?? 0) > Date.now();

  return (
    <main className="flex h-screen flex-col">
      <header className="flex items-center justify-between border-b p-3">
        <h1 className="text-lg font-bold">{t("chat.title")} <span className="text-sm font-normal text-muted-foreground">· {me.username}</span></h1>
        <div className="flex items-center gap-2">
          {!online && <span className="text-xs text-muted-foreground">{t("chat.connecting")}</span>}
          <LanguageToggle />
          <Button variant="outline" size="sm" onClick={out}>{t("chat.logout")}</Button>
        </div>
      </header>
      <div className="flex min-h-0 flex-1">
        {!isGuest && (
          <aside className={cn("w-full overflow-y-auto border-r md:block md:w-72", activeId && "hidden")}>
            <UserSearch onPick={pickUser} onUnauthorized={toLanding} />
            <h2 className="px-3 text-sm font-semibold text-muted-foreground">{t("chat.conversations")}</h2>
            <ConversationList items={convs} activeId={activeId} onSelect={setActiveId} />
          </aside>
        )}
        <section className={cn("min-w-0 flex-1 flex-col", activeId || isGuest ? "flex" : "hidden md:flex")}>
          {active ? (
            <>
              <div className="flex items-center gap-2 border-b p-3">
                {!isGuest && <Button variant="ghost" size="sm" className="md:hidden" onClick={() => setActiveId(null)}>{t("chat.back")}</Button>}
                <span className="font-medium">{active.peer_username}</span>
              </div>
              <MessageView messages={activeMsgs} pending={pending.filter((p) => p.conversation_id === active.id)}
                meId={me.id} peerReadSeq={active.peer_last_read_seq} typing={typing} />
              <Composer onSend={(b) => send(active.id, b)}
                onTyping={() => sock.current?.send({ type: "typing", conversation_id: active.id })} />
            </>
          ) : (
            <p className="m-auto text-muted-foreground">{t("chat.select")}</p>
          )}
        </section>
      </div>
      {isGuest && (
        <GuestContactDialog open={contactOpen} onClose={() => setContactOpen(false)}
          onSubmit={(a) => { if (activeRef.current) send(activeRef.current, a); }} />
      )}
    </main>
  );
}
