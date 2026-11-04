import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Settings, WifiOff } from "lucide-react";
import { Button } from "@/components/ui/button";
import ConversationList from "@/components/ConversationList";
import MessageList from "@/components/MessageList";
import ChatHeader from "@/components/ChatHeader";
import Composer from "@/components/Composer";
import UserSearch from "@/components/UserSearch";
import { peerName, useChat } from "@/hooks/useChat";
import { cn } from "@/lib/utils";

export default function Chat() {
  const { t } = useTranslation();
  const c = useChat();

  if (c.bootError) {
    return (
      <div className="flex min-h-screen flex-col items-start gap-3 bg-background p-6 text-foreground" role="alert">
        <p className="text-muted-foreground">{t("chat.loadFailed")}</p>
        <Button onClick={c.retryBoot}>{t("common.retry")}</Button>
      </div>
    );
  }
  if (!c.me) return <p className="min-h-screen bg-background p-6 text-muted-foreground">{t("common.loading")}</p>;

  const me = c.me;
  const isGuest = me.kind === "guest";
  const active = c.convs.find((x) => x.id === c.activeId) ?? null;

  return (
    <main className="flex h-screen flex-col bg-background text-foreground">
      {!c.online && (
        <div role="status" className="flex items-center justify-center gap-2 bg-primary px-3 py-1 text-xs font-semibold text-primary-foreground">
          <WifiOff className="size-3.5" /> {t("chat.connecting")}
        </div>
      )}
      <div className="flex min-h-0 flex-1">
        <aside className={cn("flex w-full min-w-0 flex-col border-r bg-card md:w-80 md:flex-none",
          (c.activeId || isGuest) && "hidden md:flex")}>
          <header className="flex items-center justify-between gap-2 border-b px-3 py-2">
            <h1 className="min-w-0 truncate text-lg font-extrabold text-primary">
              {t("chat.title")} <span className="text-sm font-normal text-muted-foreground">· {me.username}</span>
            </h1>
            <Button asChild variant="ghost" size="icon" aria-label={t("chat.settings")}>
              <Link to="/settings"><Settings /></Link>
            </Button>
          </header>
          {!isGuest && <UserSearch onPick={(u) => void c.openPeer(u.id)} onUnauthorized={c.toLanding} />}
          <h2 className={cn("px-3 pb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground", isGuest && "pt-4")}>{t("chat.conversations")}</h2>
          <div className="min-h-0 flex-1 overflow-y-auto">
            <ConversationList items={c.convs} activeId={c.activeId} onSelect={c.setActiveId} />
          </div>
        </aside>
        <section className={cn("min-w-0 flex-1 flex-col", c.activeId || isGuest ? "flex" : "hidden md:flex")}>
          {isGuest && (
            <div className="flex items-center justify-between border-b bg-card px-3 py-1 md:hidden">
              <span className="text-sm font-bold text-primary">{t("chat.title")} · {me.username}</span>
              <Button asChild variant="ghost" size="icon" aria-label={t("chat.settings")}>
                <Link to="/settings"><Settings /></Link>
              </Button>
            </div>
          )}
          {active ? (
            <>
              <ChatHeader peerId={active.peer_id} name={peerName(active)} username={active.peer_username}
                typing={c.typing(active.id)} onBack={isGuest ? undefined : () => c.setActiveId(null)}
                blocked={active.blocked_by_me} ended={active.peer_ended}
                onBlock={isGuest || active.peer_ended ? undefined : () => c.setPeerBlocked(active.peer_id, true)}
                onUnblock={isGuest || active.peer_ended ? undefined : () => c.setPeerBlocked(active.peer_id, false)} />
              <MessageList messages={c.messages(active.id)} pending={c.pending(active.id)} pendingIds={c.pendingIds}
                meId={me.id} peerReadSeq={active.peer_last_read_seq} />
              {active.peer_ended ? (
                <div className="border-t bg-card p-3" role="status">
                  <p className="text-sm text-muted-foreground">{t("chat.peerEnded")}</p>
                </div>
              ) : active.blocked_by_me ? (
                <div className="flex items-center justify-between gap-3 border-t bg-card p-3">
                  <p className="text-sm text-muted-foreground">{t("chat.youBlocked")}</p>
                  <Button size="sm" variant="outline" className="shrink-0 rounded-full" onClick={() => void c.setPeerBlocked(active.peer_id, false)}>{t("chat.unblock")}</Button>
                </div>
              ) : (
                <Composer onSend={(b) => c.send(active.id, b)} onTyping={() => c.sendTyping(active.id)} />
              )}
            </>
          ) : (
            <div className="m-auto text-center text-muted-foreground">
              <p className="font-semibold">{t("chat.select")}</p>
              <p className="text-sm">{t("chat.selectHint")}</p>
            </div>
          )}
        </section>
      </div>
    </main>
  );
}
