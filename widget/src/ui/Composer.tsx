import { useRef, useState } from "preact/hooks";
import { t, type Lang } from "../i18n";

export function Composer({ lang, onSend, onTyping }: { lang: Lang; onSend: (b: string) => void; onTyping: () => void }) {
  const [v, setV] = useState("");
  const last = useRef(0);
  const submit = (e: Event) => {
    e.preventDefault();
    if (!v.trim()) return;
    onSend(v);
    setV("");
  };
  return (
    <form class="pm-comp" onSubmit={submit}>
      <input
        value={v}
        maxLength={4000}
        placeholder={t(lang, "placeholder")}
        aria-label={t(lang, "placeholder")}
        onInput={(e) => {
          setV((e.target as HTMLInputElement).value);
          const now = Date.now();
          if (now - last.current > 3000) { last.current = now; onTyping(); }
        }}
      />
      <button type="submit" disabled={!v.trim()} aria-label={t(lang, "send")} title={t(lang, "send")}>
        <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="m5 12 7-7 7 7M12 19V5" />
        </svg>
      </button>
    </form>
  );
}
