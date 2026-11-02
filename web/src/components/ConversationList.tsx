import { useTranslation } from "react-i18next";
import type { ConversationSummary } from "@/lib/api";
import Avatar from "@/components/Avatar";
import { peerName } from "@/hooks/useChat";
import { cn } from "@/lib/utils";

export default function ConversationList({ items, activeId, onSelect }: {
  items: ConversationSummary[]; activeId: string | null; onSelect: (id: string) => void;
}) {
  const { t } = useTranslation();
  if (items.length === 0) return <p className="p-3 text-sm text-muted-foreground">{t("chat.empty")}</p>;
  return (
    <ul>
      {items.map((c) => (
        <li key={c.id}>
          <button type="button" onClick={() => onSelect(c.id)}
            className={cn("flex w-full items-center gap-3 px-3 py-2.5 text-left hover:bg-muted", c.id === activeId && "bg-muted")}>
            <Avatar id={c.peer_id} name={peerName(c)} />
            <span className="min-w-0 flex-1">
              <span className="flex items-baseline gap-1.5">
                <span className="truncate font-semibold">{peerName(c)}</span>
                <span className="truncate text-xs text-muted-foreground">@{c.peer_username}</span>
              </span>
              {c.last_message && <span className="block truncate text-sm text-muted-foreground">{c.last_message.body}</span>}
            </span>
            {c.unread > 0 && c.id !== activeId && (
              <span className="rounded-full bg-primary px-2 py-0.5 text-xs font-bold text-primary-foreground" aria-label="unread">{c.unread}</span>
            )}
          </button>
        </li>
      ))}
    </ul>
  );
}
