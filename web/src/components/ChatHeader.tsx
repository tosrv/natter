import { ArrowLeft } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import Avatar from "@/components/Avatar";

export default function ChatHeader({ peerId, name, username, typing, onBack }: {
  peerId: string; name: string; username: string; typing: boolean; onBack?: () => void;
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
        </p>
      </div>
    </div>
  );
}
