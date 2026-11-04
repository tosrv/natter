import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowUp } from "lucide-react";
import { Button } from "@/components/ui/button";

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
    <form className="flex items-start gap-2 border-t bg-card p-3" onSubmit={(e) => { e.preventDefault(); submit(); }}>
      <div className="flex-1">
        <textarea value={body} rows={2} placeholder={t("chat.placeholder")} aria-label={t("chat.placeholder")}
          className="w-full resize-none border-0 bg-transparent p-0 text-sm text-foreground focus:outline-none"
          onChange={(e) => {
            setBody(e.target.value);
            const now = Date.now();
            if (now - lastTyping.current >= 3000) { lastTyping.current = now; onTyping(); }
          }}
          onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); submit(); } }} />
        {len > MAX_BODY && <p className="text-right text-xs text-destructive">{len}/{MAX_BODY}</p>}
      </div>
      <Button type="submit" size="icon" className="shrink-0" disabled={disabled} aria-label={t("chat.send")} title={t("chat.send")}>
        <ArrowUp className="h-4 w-4" aria-hidden />
      </Button>
    </form>
  );
}
