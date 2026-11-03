import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import {
  ApiError, blockUser, getMe, unblockUser, listConversations, listMessages, logout as apiLogout, openConversation,
  type ConversationSummary, type Me, type Message,
} from "@/lib/api";
import { Outbox, mergeMessages, type ClientMsg } from "@/lib/outbox";
import { ChatSocket, type ServerFrame } from "@/lib/socket";
import { fetchMissing } from "@/lib/paging";
import { errorMessage } from "@/i18n";

const TYPING_MS = 5000;
const RATE_RETRY_BASE_MS = 10000;
const MAX_RATE_RETRIES = 3;

export interface ChatState {
  convs: ConversationSummary[];
  msgs: Record<string, Message[]>;
  activeId: string | null;
}

/** Pure: drop a conversation from the list and the message cache; clear activeId if it was open. */
export function removeConversation(s: ChatState, convId: string): ChatState {
  const msgs = { ...s.msgs };
  delete msgs[convId];
  return {
    convs: s.convs.filter((c) => c.id !== convId),
    msgs,
    activeId: s.activeId === convId ? null : s.activeId,
  };
}

/** Pure: set blocked_by_me on the conversation whose id or peer_id matches. */
export function setBlocked(s: ChatState, convIdOrPeerId: string, blocked: boolean): ChatState {
  return {
    ...s,
    convs: s.convs.map((c) => (c.id === convIdOrPeerId || c.peer_id === convIdOrPeerId) ? { ...c, blocked_by_me: blocked } : c),
  };
}

/** Pure: mark the conversation as ended by the peer (guest session over). */
export function markEnded(s: ChatState, convId: string): ChatState {
  return { ...s, convs: s.convs.map((c) => c.id === convId ? { ...c, peer_ended: true } : c) };
}

/** Pure-ish: mark ended and drop that conversation's pending outbox entries; returns dropped client_msg_ids. */
export function handlePeerEnded(s: ChatState, outbox: Outbox, convId: string): { state: ChatState; dropped: string[] } {
  return { state: markEnded(s, convId), dropped: outbox.dropConversation(convId) };
}

export const peerName = (c: Pick<ConversationSummary, "peer_display_name" | "peer_username"> | undefined) =>
  c ? (c.peer_display_name || c.peer_username) : "";

export function useChat() {
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
          toast(t("chat.newMessage", { name: peerName(known) }), { description: msg.body.slice(0, 100) });
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
      s.on("conversation_deleted", (f) => {
        const id = (f as Extract<ServerFrame, { type: "conversation_deleted" }>).conversation_id;
        const wasActive = activeRef.current === id;
        const next = removeConversation({ convs: convsRef.current, msgs: msgsRef.current, activeId: activeRef.current }, id);
        setConvs(next.convs); setMsgs(next.msgs); setActiveId(next.activeId);
        convsRef.current = next.convs; msgsRef.current = next.msgs; activeRef.current = next.activeId;
        delete historyLoaded.current[id]; delete lastReadSent.current[id];
        for (const cid of outbox.current.dropConversation(id)) delete rateRetries.current[cid];
        setPending(outbox.current.pending());
        clearTimeout(typingTimers.current[id]); delete typingTimers.current[id];
        setTypingUntil((tu) => { const n = { ...tu }; delete n[id]; return n; });
        if (wasActive) toast(t("chat.conversationDeleted"));
      });
      s.on("conversation_ended", (f) => {
        const id = (f as Extract<ServerFrame, { type: "conversation_ended" }>).conversation_id;
        const { state, dropped } = handlePeerEnded({ convs: convsRef.current, msgs: msgsRef.current, activeId: activeRef.current }, outbox.current, id);
        setConvs(state.convs); convsRef.current = state.convs;
        for (const cid of dropped) delete rateRetries.current[cid];
        setPending(outbox.current.pending());
        clearTimeout(typingTimers.current[id]); delete typingTimers.current[id];
        setTypingUntil((tu) => ({ ...tu, [id]: 0 }));
        if (activeRef.current === id) toast(t("chat.peerEndedToast"));
      });
      s.on("error", (f) => {
        const e = f as Extract<ServerFrame, { type: "error" }>;
        if (e.code === "peer_ended") {
          // The error frame carries no conversation id; find it via the pending message.
          const convId = e.client_msg_id
            ? outbox.current.pending().find((x) => x.client_msg_id === e.client_msg_id)?.conversation_id : undefined;
          if (convId) {
            const { state, dropped } = handlePeerEnded({ convs: convsRef.current, msgs: msgsRef.current, activeId: activeRef.current }, outbox.current, convId);
            setConvs(state.convs); convsRef.current = state.convs;
            for (const cid of dropped) delete rateRetries.current[cid];
            setPending(outbox.current.pending());
          } else if (e.client_msg_id) { outbox.current.fail(e.client_msg_id); setPending(outbox.current.pending()); }
          toast.error(errorMessage("peer_ended"));
          return;
        }
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
            const pm = outbox.current.pending().find((x) => x.client_msg_id === id);
            if (pm) sock.current?.send({ type: "send", ...pm });
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
      // A failure after getMe (e.g. loading conversations) must still show the
      // retry screen, so clear the half-loaded profile too.
      if (e instanceof ApiError && e.status === 401) toLanding(); else { setMe(null); setBootError(true); }
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

  const send = useCallback((convId: string, body: string) => {
    const m = outbox.current.add(convId, body);
    setPending(outbox.current.pending());
    sock.current?.send({ type: "send", ...m }); // if offline, resent on reconnect
  }, []);

  const sendTyping = useCallback((convId: string) => {
    sock.current?.send({ type: "typing", conversation_id: convId });
  }, []);

  const setPeerBlocked = useCallback(async (peerId: string, blocked: boolean) => {
    try {
      if (blocked) await blockUser(peerId); else await unblockUser(peerId);
      setConvs((cs) => setBlocked({ convs: cs, msgs: {}, activeId: null }, peerId, blocked).convs);
      toast.success(t(blocked ? "chat.blockedToast" : "chat.unblockedToast"));
    } catch (e) { handleErr(e); }
  }, [handleErr, t]);

  const openPeer = useCallback(async (peerId: string) => {
    try {
      const { id } = await openConversation(peerId);
      if (!convsRef.current.some((c) => c.id === id)) await refreshConvs();
      setActiveId(id);
    } catch (e) { handleErr(e); }
  }, [refreshConvs, handleErr]);

  const logout = useCallback(async () => {
    sock.current?.close();
    try { await apiLogout(); } catch { /* session may already be gone */ }
    toLanding();
  }, [toLanding]);

  const messages = useCallback((convId: string) => msgs[convId] ?? [], [msgs]);
  const pendingFor = useCallback((convId: string) => pending.filter((p) => p.conversation_id === convId), [pending]);
  const pendingIds = useMemo(() => new Set(pending.map((p) => p.client_msg_id)), [pending]);
  const typing = useCallback((convId: string) => (typingUntil[convId] ?? 0) > Date.now(), [typingUntil]);
  const retryBoot = useCallback(() => setBootAttempt((n) => n + 1), []);

  return {
    me, convs, activeId, setActiveId, messages, pending: pendingFor, pendingIds, typing, online,
    bootError, retryBoot, send, sendTyping, openPeer, setPeerBlocked, markRead, logout, toLanding,
    contactOpen, setContactOpen,
  };
}
