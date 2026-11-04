import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import LanguageToggle from "@/components/LanguageToggle";
import ThemeSwitch from "@/components/ThemeSwitch";
import { ApiError, getMe, setDisplayName, setUsername } from "@/lib/api";
import { validateDisplayName } from "@/lib/format";
import { errorMessage } from "@/i18n";

const USERNAME_RE = /^[a-z0-9_]{3,20}$/;

export default function Onboarding() {
  const { t } = useTranslation();
  const nav = useNavigate();
  const [display, setDisplay] = useState("");
  const [username, setUser] = useState("");
  const [currentUsername, setCurrentUsername] = useState("");
  const [currentDisplay, setCurrentDisplay] = useState("");
  const [errs, setErrs] = useState<{ display?: string; username?: string; form?: string }>({});
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    getMe().then((me) => {
      if (me.username && me.display_name !== "User") { nav("/chat", { replace: true }); return; }
      if (me.display_name !== "User") { setDisplay(me.display_name); setCurrentDisplay(me.display_name); }
      if (me.username) { setUser(me.username); setCurrentUsername(me.username); }
    }).catch(() => nav("/", { replace: true }));
  }, [nav]);

  const dn = validateDisplayName(display);
  const uname = username.trim().toLowerCase();
  const dnShown = display.length > 0 && !dn.ok;
  const unShown = uname.length > 0 && !USERNAME_RE.test(uname);
  const valid = dn.ok && USERNAME_RE.test(uname);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!dn.ok || !USERNAME_RE.test(uname)) return;
    setBusy(true);
    setErrs({});
    let step: "display" | "username" = "display";
    try {
      if (dn.value !== currentDisplay) await setDisplayName(dn.value);
      step = "username";
      if (uname !== currentUsername) await setUsername(uname);
      nav("/chat", { replace: true });
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) { nav("/", { replace: true }); return; }
      const code = e instanceof ApiError ? e.code : "internal";
      const msg = errorMessage(code);
      if (code === "display_name_invalid") setErrs({ display: msg });
      else if (code === "username_invalid" || code === "username_taken") setErrs({ username: msg });
      else setErrs(step === "display" ? { display: msg } : { username: msg });
      if (step === "username") setCurrentDisplay(dn.ok ? dn.value : currentDisplay);
      setBusy(false);
    }
  }

  return (
    <main className="relative flex min-h-screen flex-col items-center justify-center bg-gradient-to-b from-primary/15 via-background to-background p-6">
      <div className="absolute right-3 top-3 flex items-center gap-1"><ThemeSwitch /><LanguageToggle /></div>
      <form onSubmit={submit} className="flex w-full max-w-sm flex-col gap-4 rounded-3xl border border-border bg-card p-6 shadow-sm">
        <h1 className="text-2xl font-extrabold">{t("onboarding.heading")}</h1>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="display-name" className="text-sm font-semibold">{t("onboarding.displayName")}</label>
          {/* eslint-disable-next-line jsx-a11y/no-autofocus -- intentional: the only field on the onboarding step */}
          <Input id="display-name" value={display} onChange={(e) => setDisplay(e.target.value)} autoFocus autoComplete="off"
            maxLength={40} placeholder={t("onboarding.displayNamePlaceholder")} aria-invalid={dnShown || !!errs.display}
            aria-describedby="display-name-msg" className="rounded-full" />
          <p id="display-name-msg" role={errs.display || dnShown ? "alert" : undefined}
            className={`text-xs ${errs.display || dnShown ? "text-destructive" : "text-muted-foreground"}`}>
            {errs.display ?? (dnShown ? errorMessage("display_name_invalid") : t("onboarding.displayNameHint"))}
          </p>
        </div>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="username" className="text-sm font-semibold">{t("onboarding.title")}</label>
          <Input id="username" value={username} onChange={(e) => setUser(e.target.value)} autoCapitalize="none" autoComplete="off"
            maxLength={20} placeholder={t("onboarding.placeholder")} aria-invalid={unShown || !!errs.username}
            aria-describedby="username-msg" className="rounded-full" />
          <p id="username-msg" role={errs.username || unShown ? "alert" : undefined}
            className={`text-xs ${errs.username || unShown ? "text-destructive" : "text-muted-foreground"}`}>
            {errs.username ?? (unShown ? errorMessage("username_invalid") : t("onboarding.hint"))}
          </p>
        </div>
        <Button type="submit" size="lg" className="" disabled={busy || !valid}>{t("onboarding.submit")}</Button>
      </form>
    </main>
  );
}
