import { Fragment, useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import type { Message } from "@/lib/api";
import type { ClientMsg } from "@/lib/outbox";
import { dayLabel, statusOf } from "@/lib/format";
import MessageBubble from "@/components/MessageBubble";

const dayKey = (d: Date) => `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;

export default function MessageList({ messages, pending, pendingIds, meId, peerReadSeq }: {
  messages: Message[]; pending: ClientMsg[]; pendingIds: Set<string>; meId: string; peerReadSeq: number;
}) {
  const { t, i18n } = useTranslation();
  const lang = i18n.language?.startsWith("id") ? "id" : "en";
  const end = useRef<HTMLDivElement>(null);
  useEffect(() => { end.current?.scrollIntoView({ block: "end" }); }, [messages.length, pending.length]);
  const now = new Date();
  let prevDay = "";

  return (
    <div className="flex-1 space-y-1.5 overflow-y-auto bg-background p-3">
      {messages.length === 0 && pending.length === 0 && <p className="mt-6 text-center text-sm text-muted-foreground">{t("chat.noMessages")}</p>}
      {messages.map((m) => {
        const d = new Date(m.created_at);
        const k = dayKey(d);
        const sep = !isNaN(d.getTime()) && k !== prevDay;
        if (sep) prevDay = k;
        return (
          <Fragment key={m.id}>
            {sep && (
              <div className="flex justify-center py-2">
                <span className="rounded-full bg-muted px-3 py-0.5 text-xs text-muted-foreground">{dayLabel(d, now, lang)}</span>
              </div>
            )}
            <MessageBubble mine={m.sender_id === meId} body={m.body} createdAt={m.created_at}
              status={statusOf(m, meId, peerReadSeq, pendingIds)} />
          </Fragment>
        );
      })}
      {pending.map((p) => <MessageBubble key={p.client_msg_id} mine body={p.body} status="sending" />)}
      <div ref={end} />
    </div>
  );
}
