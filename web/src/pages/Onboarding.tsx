import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import LanguageToggle from "@/components/LanguageToggle";
import { ApiError, getMe, setUsername } from "@/lib/api";
import { errorMessage } from "@/i18n";

export default function Onboarding() {
  const { t } = useTranslation();
  const nav = useNavigate();
  const [name, setName] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    getMe().then((me) => { if (me.username) nav("/chat", { replace: true }); })
      .catch(() => nav("/", { replace: true }));
  }, [nav]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    try {
      await setUsername(name.trim().toLowerCase());
      nav("/chat", { replace: true });
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) { nav("/", { replace: true }); return; }
      setErr(e instanceof ApiError ? e.code : "internal");
      setBusy(false);
    }
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-sm flex-col justify-center gap-4 p-6">
      <div className="absolute right-4 top-4"><LanguageToggle /></div>
      <h1 className="text-2xl font-bold">{t("onboarding.title")}</h1>
      <form onSubmit={submit} className="flex flex-col gap-3">
        <Input value={name} onChange={(e) => setName(e.target.value)} placeholder={t("onboarding.placeholder")}
          autoFocus autoCapitalize="none" autoComplete="off" maxLength={20} aria-label={t("onboarding.title")} />
        <p className="text-sm text-muted-foreground">{t("onboarding.hint")}</p>
        {err && <p role="alert" className="text-sm text-destructive">{errorMessage(err)}</p>}
        <Button type="submit" disabled={busy || name.length === 0}>{t("onboarding.submit")}</Button>
      </form>
    </main>
  );
}
