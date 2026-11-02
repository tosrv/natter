import { useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import type { Message } from "@/lib/api";
import type { ClientMsg } from "@/lib/outbox";
import { cn } from "@/lib/utils";

export default function MessageView({ messages, pending, meId, peerReadSeq, typing }: {
  messages: Message[]; pending: ClientMsg[]; meId: string; peerReadSeq: number; typing: boolean;
}) {
  const { t } = useTranslation();
  const end = useRef<HTMLDivElement>(null);
  useEffect(() => { end.current?.scrollIntoView({ block: "end" }); }, [messages.length, pending.length, typing]);

  return (
    <div className="flex-1 space-y-2 overflow-y-auto p-3">
      {messages.length === 0 && pending.length === 0 && <p className="text-center text-sm text-muted-foreground">{t("chat.noMessages")}</p>}
      {messages.map((m) => {
        const mine = m.sender_id === meId;
        const read = mine && peerReadSeq >= m.seq;
        return (
          <Bubble key={m.id} mine={mine} body={m.body}
            status={mine ? <span title={t(read ? "chat.read" : "chat.sent")} aria-label={t(read ? "chat.read" : "chat.sent")}>{read ? "✓✓" : "✓"}</span> : null} />
        );
      })}
      {pending.map((p) => <Bubble key={p.client_msg_id} mine body={p.body} status={<span>{t("chat.sending")}</span>} />)}
      {typing && <p className="text-sm italic text-muted-foreground">{t("chat.typing")}</p>}
      <div ref={end} />
    </div>
  );
}

function Bubble({ mine, body, status }: { mine: boolean; body: string; status: React.ReactNode }) {
  return (
    <div className={cn("flex", mine ? "justify-end" : "justify-start")}>
      <div className={cn("max-w-[80%] rounded-lg px-3 py-2", mine ? "bg-primary text-primary-foreground" : "bg-muted")}>
        {/* Rendered as plain text; never HTML. */}
        <p className="whitespace-pre-wrap break-words">{body}</p>
        {status && <p className="mt-1 text-right text-xs opacity-75">{status}</p>}
      </div>
    </div>
  );
}
