import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import LanguageToggle from "@/components/LanguageToggle";
import { getMe, logout } from "@/lib/api";

// Placeholder: replaced by the real chat UI in Task 14.
export default function Chat() {
  const { t } = useTranslation();
  const nav = useNavigate();

  useEffect(() => {
    getMe().then((me) => { if (!me.username) nav("/onboarding", { replace: true }); })
      .catch(() => nav("/", { replace: true }));
  }, [nav]);

  async function out() {
    try { await logout(); } catch { /* session may already be gone */ }
    nav("/", { replace: true });
  }

  return (
    <main className="mx-auto max-w-xl space-y-4 p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">{t("chat.title")}</h1>
        <div className="flex gap-2"><LanguageToggle /><Button variant="outline" size="sm" onClick={out}>{t("chat.logout")}</Button></div>
      </div>
      <p className="text-muted-foreground">{t("chat.placeholder")}</p>
    </main>
  );
}
