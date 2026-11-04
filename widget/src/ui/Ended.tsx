import { t, type Lang } from "../i18n";

export function Ended({ lang, onRestart }: { lang: Lang; onRestart: () => void }) {
  return (
    <div class="pm-note">
      <p>{t(lang, "ended")}</p>
      <button type="button" class="pm-btn" onClick={onRestart}>{t(lang, "newChat")}</button>
    </div>
  );
}
