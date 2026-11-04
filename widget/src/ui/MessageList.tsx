import { useEffect, useRef } from "preact/hooks";
import type { ViewState } from "../controller";
import { t, type Lang } from "../i18n";
import { Ticks } from "./Ticks";

export function MessageList({ s, lang }: { s: ViewState; lang: Lang }) {
  const ref = useRef<HTMLDivElement>(null);
  const { messages, outbox, peerReadSeq, peerTyping } = s.chat;
  useEffect(() => { const el = ref.current; if (el) el.scrollTop = el.scrollHeight; }, [messages.length, outbox.length, peerTyping]);
  return (
    <div class="pm-list" ref={ref} role="log" aria-live="polite">
      {messages.map((m) => {
        const me = m.sender_id === s.meId;
        return (
          <div key={m.id} class={me ? "pm-msg pm-me" : "pm-msg"}>
            {m.body}
            {me && (m.seq <= peerReadSeq
              ? <Ticks status="read" label={t(lang, "seen")} />
              : <Ticks status="sent" label={t(lang, "sent")} />)}
          </div>
        );
      })}
      {outbox.map((o) => (
        <div key={o.client_msg_id} class="pm-msg pm-me pm-pend">
          {o.body}
          <Ticks status="sending" label={t(lang, "sending")} />
        </div>
      ))}
      {peerTyping && <div class="pm-typing">{t(lang, "typing")}</div>}
    </div>
  );
}
