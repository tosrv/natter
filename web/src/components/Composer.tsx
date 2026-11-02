import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export const MAX_BODY = 2000;
export const runeLength = (s: string) => [...s].length;

export default function Composer({ onSend, onTyping }: { onSend: (body: string) => void; onTyping: () => void }) {
  const { t } = useTranslation();
  const [body, setBody] = useState("");
  const lastTyping = useRef(0);
  const len = runeLength(body);
  const disabled = body.trim() === "" || len > MAX_BODY;

  function submit() {
    if (disabled) return;
    onSend(body);
    setBody("");
  }

  return (
    <form className="flex items-end gap-2 border-t p-3" onSubmit={(e) => { e.preventDefault(); submit(); }}>
      <div className="flex-1">
        <textarea value={body} rows={2} placeholder={t("chat.placeholder")} aria-label={t("chat.placeholder")}
          className="w-full resize-none rounded-md border bg-background p-2 text-sm"
          onChange={(e) => {
            setBody(e.target.value);
            const now = Date.now();
            if (now - lastTyping.current >= 3000) { lastTyping.current = now; onTyping(); }
          }}
          onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); submit(); } }} />
        <p className={cn("text-right text-xs text-muted-foreground", len > MAX_BODY && "text-destructive")}>{len}/{MAX_BODY}</p>
      </div>
      <Button type="submit" disabled={disabled}>{t("chat.send")}</Button>
    </form>
  );
}
