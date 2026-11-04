import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import Wordmark from "@/components/Wordmark";

export const GITHUB_URL = "https://github.com/tosrv/natter";

export default function Footer() {
  const { t } = useTranslation();
  return (
    <footer className="border-t px-4 py-8 sm:px-6">
      <div className="mx-auto flex w-full max-w-6xl flex-col items-center justify-between gap-4 text-sm text-muted-foreground md:flex-row">
        <div className="flex items-center gap-2">
          <Wordmark className="h-5 w-auto text-foreground" />
          <span>— {t("landing.intro.title")}</span>
        </div>
        <nav aria-label="Footer" className="flex items-center gap-6">
          <Link to="/terms" className="transition-colors hover:text-foreground">{t("landing.terms")}</Link>
          <Link to="/privacy" className="transition-colors hover:text-foreground">{t("landing.privacy")}</Link>
          <a href={GITHUB_URL} target="_blank" rel="noreferrer" className="transition-colors hover:text-foreground">{t("landing.github")}</a>
        </nav>
        <span>© {new Date().getFullYear()} {t("app.name")}</span>
      </div>
    </footer>
  );
}
