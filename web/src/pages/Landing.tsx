import { useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import LanguageToggle from "@/components/LanguageToggle";
import { ApiError, createGuest, getMe } from "@/lib/api";
import { errorMessage } from "@/i18n";

export default function Landing() {
  const { t } = useTranslation();
  const nav = useNavigate();
  const [params] = useSearchParams();
  const [err, setErr] = useState<string | null>(params.get("error"));
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let alive = true;
    getMe().then((me) => {
      if (alive) nav(me.username ? "/chat" : "/onboarding", { replace: true });
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
    <main className="mx-auto flex min-h-screen max-w-sm flex-col justify-center gap-4 p-6">
      <div className="absolute right-4 top-4"><LanguageToggle /></div>
      <h1 className="text-3xl font-bold">{t("app.name")}</h1>
      <p className="text-muted-foreground">{t("app.tagline")}</p>
      {err && <p role="alert" className="rounded-md border border-destructive p-3 text-sm text-destructive">{errorMessage(err)}</p>}
      <Button asChild><a href="/api/auth/google/login">{t("landing.google")}</a></Button>
      <Button variant="outline" onClick={guest} disabled={busy}>{t("landing.guest")}</Button>
      <Link to="/info" className="text-center text-sm underline">{t("landing.info")}</Link>
    </main>
  );
}
