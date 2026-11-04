import type { Controller } from "../controller";
import { t, type Lang } from "../i18n";
import { Turnstile } from "./Turnstile";
import { MessageList } from "./MessageList";
import { Composer } from "./Composer";
import { Ended } from "./Ended";

export function Panel({ ctl, lang, onClose }: { ctl: Controller; lang: Lang; onClose: () => void }) {
  const s = ctl.state();
  let body;
  switch (s.phase) {
    case "turnstile":
      body = <Turnstile siteKey={s.siteKey ?? ""} onToken={(tk) => ctl.startGuest(tk)} />;
      break;
    case "chat":
      body = <>
        {!s.chat.online && <div class="pm-banner">{t(lang, "connecting")}</div>}
        <MessageList s={s} lang={lang} />
        <Composer lang={lang} onSend={ctl.send} onTyping={ctl.typing} />
      </>;
      break;
    case "ended":
      body = <Ended lang={lang} onRestart={ctl.restart} />;
      break;
    case "full":
    case "unavailable":
      body = <p class="pm-note">{t(lang, s.phase)}</p>;
      break;
    default:
      body = <p class="pm-note">{t(lang, "connecting")}</p>;
  }
  return (
    <section class="pm-panel" role="dialog" aria-label={t(lang, "title")}>
      <header class="pm-head">
        <span>{t(lang, "title")}</span>
        <button type="button" class="pm-x" aria-label={t(lang, "close")} onClick={onClose}>×</button>
      </header>
      {body}
    </section>
  );
}
