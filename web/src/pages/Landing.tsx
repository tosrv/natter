import { useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import LanguageToggle from "@/components/LanguageToggle";
import ThemeToggle from "@/components/ThemeToggle";
import { ApiError, createGuest, getMe } from "@/lib/api";
import { errorMessage } from "@/i18n";

export default function Landing() {
  const { t } = useTranslation();
  const nav = useNavigate();
  const [params] = useSearchParams();
  const [err, setErr] = useState<string | null>(params.get("error"));
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (params.has("error")) window.history.replaceState(null, "", window.location.pathname);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    let alive = true;
    getMe().then((me) => {
      if (!alive) return;
      const needsProfile = !me.username || me.display_name === "User";
      nav(needsProfile ? "/onboarding" : "/chat", { replace: true });
    }).catch(() => { /* not logged in */ });
    return () => { alive = false; };
  }, [nav]);

  async function guest() {
    setBusy(true);
    setErr(null);
    try {
      await createGuest();
      nav("/chat", { state: { newGuest: true } });
    } catch (e) {
      setErr(e instanceof ApiError ? e.code : "internal");
      setBusy(false);
    }
  }

  return (
    <main className="relative flex min-h-screen flex-col items-center justify-center overflow-hidden bg-gradient-to-b from-primary/15 via-background to-background p-6">
      <div className="absolute right-3 top-3 flex items-center gap-1"><ThemeToggle /><LanguageToggle /></div>
      <div className="flex w-full max-w-sm flex-col items-center gap-5 text-center">
        <div aria-hidden className="flex w-full flex-col gap-3 py-2">
          <div className="max-w-[75%] self-start rounded-2xl rounded-bl-sm border border-border bg-card px-4 py-3 text-left text-sm shadow-sm">
            {t("landing.bubbleIn")}
          </div>
          <div className="max-w-[75%] self-end rounded-2xl rounded-br-sm bg-primary px-4 py-3 text-left text-sm text-primary-foreground shadow-sm">
            {t("landing.bubbleOut")}
          </div>
        </div>
        <h1 className="text-4xl font-extrabold tracking-tight">{t("app.name")}</h1>
        <p className="text-muted-foreground">{t("app.tagline")}</p>
        {err && <p role="alert" className="w-full rounded-2xl border border-destructive bg-card p-3 text-sm text-destructive">{errorMessage(err)}</p>}
        <div className="flex w-full flex-col gap-3">
          <Button asChild size="lg" className="rounded-full"><a href="/api/auth/google/login">{t("landing.google")}</a></Button>
          <Button variant="outline" size="lg" className="rounded-full" onClick={guest} disabled={busy}>{t("landing.guest")}</Button>
        </div>
        <Link to="/info" className="text-sm text-muted-foreground underline">{t("landing.info")}</Link>
      </div>
    </main>
  );
}
