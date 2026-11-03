import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { ArrowLeft, MessageCircle } from "lucide-react";
import LanguageToggle from "@/components/LanguageToggle";
import ThemeToggle from "@/components/ThemeToggle";

type LegalSection = { h: string; p: string[] };

/** Shared layout for /terms and /privacy; `ns` is the i18n namespace holding title, updated, intro, sections. */
export default function LegalPage({ ns }: { ns: "terms" | "privacy" }) {
  const { t } = useTranslation();
  const sections = t(`${ns}.sections`, { returnObjects: true }) as LegalSection[];
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
          <h1 className="text-3xl font-extrabold tracking-tight md:text-4xl">{t(`${ns}.title`)}</h1>
          <p className="mt-2 text-sm text-muted-foreground">{t(`${ns}.updated`)}</p>
          <p className="mt-6 leading-relaxed">{t(`${ns}.intro`)}</p>
          {sections.map((s) => (
            <section key={s.h} className="mt-10">
              <h2 className="text-xl font-bold">{s.h}</h2>
              {s.p.map((para) => <p key={para} className="mt-3 leading-relaxed text-muted-foreground">{para}</p>)}
            </section>
          ))}
        </article>
      </main>

      <footer className="mx-auto flex w-full max-w-5xl items-center justify-between gap-2 border-t px-4 py-4 text-xs text-muted-foreground">
        <span>© {new Date().getFullYear()} {t("app.name")}. {t("landing.rights")}</span>
        <Link to="/" className="underline">{t("common.back")}</Link>
      </footer>
    </div>
  );
}
