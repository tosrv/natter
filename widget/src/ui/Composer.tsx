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
      <button type="submit">{t(lang, "send")}</button>
    </form>
  );
}
