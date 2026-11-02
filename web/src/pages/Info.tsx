import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { ArrowLeft, Lock, UserX, Clock, Mail } from "lucide-react";
import LanguageToggle from "@/components/LanguageToggle";
import ThemeToggle from "@/components/ThemeToggle";

const ITEMS = [
  { key: "e2e", Icon: Lock },
  { key: "guest", Icon: UserX },
  { key: "retention", Icon: Clock },
  { key: "contact", Icon: Mail },
] as const;

export default function Info() {
  const { t } = useTranslation();
  return (
    <main className="min-h-screen bg-gradient-to-b from-primary/10 via-background to-background p-6">
      <div className="mx-auto max-w-xl space-y-5">
        <div className="flex items-center justify-between">
          <Link to="/" className="inline-flex items-center gap-1 text-sm font-semibold text-primary"><ArrowLeft className="h-4 w-4" aria-hidden />{t("common.back")}</Link>
          <div className="flex items-center gap-1"><ThemeToggle /><LanguageToggle /></div>
        </div>
        <h1 className="text-3xl font-extrabold tracking-tight">{t("info.title")}</h1>
        <ul className="space-y-3">
          {ITEMS.map(({ key, Icon }) => (
            <li key={key} className="flex gap-3 rounded-2xl border border-border bg-card p-4 shadow-sm">
              <Icon className="mt-0.5 h-5 w-5 shrink-0 text-primary" aria-hidden />
              <span>{t(`info.${key}`)}</span>
            </li>
          ))}
        </ul>
      </div>
    </main>
  );
}
