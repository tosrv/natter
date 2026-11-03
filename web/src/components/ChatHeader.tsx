import { ArrowLeft } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import Avatar from "@/components/Avatar";
import BlockMenu from "@/components/BlockMenu";

export default function ChatHeader({ peerId, name, username, typing, onBack, blocked, ended, onBlock, onUnblock }: {
  peerId: string; name: string; username: string; typing: boolean; onBack?: () => void;
  blocked?: boolean; ended?: boolean; onBlock?: () => Promise<void> | void; onUnblock?: () => Promise<void> | void;
}) {
  const { t } = useTranslation();
  return (
    <div className="flex items-center gap-3 border-b bg-card px-3 py-2">
      {onBack && (
        <Button variant="ghost" size="icon" className="md:hidden" onClick={onBack} aria-label={t("chat.back")}>
          <ArrowLeft />
        </Button>
      )}
      <Avatar id={peerId} name={name} className="size-9" />
      <div className="min-w-0">
        <p className="truncate font-semibold">{name}</p>
        <p className="truncate text-xs text-muted-foreground">
          {typing ? <span className="italic text-primary">{t("chat.typing")}</span> : `@${username}`}
          {ended
            ? <span className="ml-2 rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-semibold uppercase">{t("chat.ended")}</span>
            : blocked && <span className="ml-2 rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-semibold uppercase">{t("chat.blocked")}</span>}
        </p>
      </div>
      {onBlock && onUnblock && <BlockMenu name={name} blocked={!!blocked} onBlock={onBlock} onUnblock={onUnblock} />}
    </div>
  );
}
