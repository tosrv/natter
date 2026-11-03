import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { ArrowLeft, MessageCircle } from "lucide-react";
import LanguageToggle from "@/components/LanguageToggle";
import ThemeToggle from "@/components/ThemeToggle";

export default function Info() {
  const { t } = useTranslation();
  const links = [
    { to: "/terms", title: t("info.terms"), desc: t("info.termsDesc") },
    { to: "/privacy", title: t("info.privacy"), desc: t("info.privacyDesc") },
  ];
  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <header className="sticky top-0 z-20 bg-background/50 backdrop-blur-md">
        <div className="mx-auto flex h-14 w-full max-w-5xl items-center justify-between gap-4 px-4">
          <Link to="/" className="flex items-center gap-2 text-lg font-extrabold text-primary">
            <MessageCircle className="size-5" aria-hidden /> {t("app.name")}
          </Link>
          <div className="flex items-center gap-1"><ThemeToggle /><LanguageToggle /></div>
        </div>
      </header>

      <main className="mx-auto w-full max-w-3xl flex-1 px-4 py-10 md:py-16">
        <Link to="/" className="inline-flex items-center gap-1 text-sm font-semibold text-primary"><ArrowLeft className="h-4 w-4" aria-hidden />{t("common.back")}</Link>
        <article className="mt-6">
          <h1 className="text-3xl font-extrabold tracking-tight md:text-4xl">{t("info.title")}</h1>
          <p className="mt-6 leading-relaxed">{t("info.intro")}</p>
          <ul className="mt-8 flex flex-col gap-3">
            {links.map((l) => (
              <li key={l.to}>
                <Link to={l.to} className="block rounded-2xl border bg-card p-5 transition-colors hover:border-primary">
                  <h2 className="text-lg font-bold text-primary">{l.title}</h2>
                  <p className="mt-1 text-sm text-muted-foreground">{l.desc}</p>
                </Link>
              </li>
            ))}
          </ul>
        </article>
      </main>

      <footer className="mx-auto flex w-full max-w-5xl items-center justify-between gap-2 border-t px-4 py-4 text-xs text-muted-foreground">
        <span>© {new Date().getFullYear()} {t("app.name")}. {t("landing.rights")}</span>
        <Link to="/" className="underline">{t("common.back")}</Link>
      </footer>
    </div>
  );
}
