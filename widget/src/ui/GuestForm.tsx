import { useRef, useState } from "preact/hooks";
import type { Controller } from "../controller";
import { t, type Lang } from "../i18n";
import { randomGuestName } from "../guestName";
import { Turnstile } from "./Turnstile";

const MAX_NAME = 40;
// Matches the backend body limit (2000 runes).
const MAX_BODY = 2000;
const runes = (s: string) => [...s].length;

export function GuestForm({ ctl, lang, siteKey, nameError }: { ctl: Controller; lang: Lang; siteKey: string; nameError: boolean }) {
  const [name, setName] = useState(randomGuestName);
  const [contact, setContact] = useState("");
  const [token, setToken] = useState("");
  const [busy, setBusy] = useState(false);
  const reset = useRef<(() => void) | null>(null);
  const n = runes(name.trim());
  const ok = n >= 1 && n <= MAX_NAME && runes(contact) <= MAX_BODY && !!token && !busy;

  const submit = async (e: Event) => {
    e.preventDefault();
    if (!ok) return;
    setBusy(true);
    await ctl.startGuest(token, name.trim(), contact);
    // Still on this form: the attempt failed (e.g. invalid name). Tokens are
    // single-use, so require a fresh one.
    if (ctl.state().phase === "turnstile") { reset.current?.(); setToken(""); setBusy(false); }
  };

  return (
    <form class="pm-guest" onSubmit={submit}>
      <label>
        {t(lang, "guestName")}
        <input value={name} maxLength={MAX_NAME} autocomplete="off" aria-invalid={nameError || undefined}
          onInput={(e) => setName((e.target as HTMLInputElement).value)} />
        {nameError && <span class="pm-err" role="alert">{t(lang, "guestNameError")}</span>}
      </label>
      <label>
        {t(lang, "guestContact")}
        <textarea value={contact} rows={2} maxLength={MAX_BODY}
          onInput={(e) => setContact((e.target as HTMLTextAreaElement).value)} />
      </label>
      <Turnstile siteKey={siteKey} onToken={setToken} onError={ctl.fail} onReady={(r) => { reset.current = r; }} />
      <button type="submit" class="pm-go" disabled={!ok}>{t(lang, "startChat")}</button>
    </form>
  );
}
