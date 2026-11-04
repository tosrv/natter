import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { GITHUB_URL } from "./Footer";

export default function Privacy() {
  const { t } = useTranslation();
  return (
    <section id="privacy" className="scroll-mt-16 border-t px-4 py-20 sm:px-6 md:py-24">
      <div className="mx-auto max-w-3xl">
        <h2 className="text-2xl font-semibold tracking-tight">{t("landing.trust.title")}</h2>
        <p className="mb-10 mt-2 text-muted-foreground">{t("landing.trust.sub")}</p>
        <ul className="space-y-4 border-l pl-6 text-sm leading-relaxed">
          <li><span className="font-medium">{t("landing.trust.e2e.t")}</span> {t("landing.trust.e2e.d")}</li>
          <li>{t("landing.trust.retention")}</li>
          <li>{t("landing.trust.guests")}</li>
          <li>{t("landing.trust.oss")}</li>
        </ul>
        <div className="mt-8 flex gap-6 pl-6 text-sm">
          <Link to="/privacy" className="text-primary-text underline underline-offset-4">{t("landing.privacy")}</Link>
          <a href={GITHUB_URL} className="text-primary-text underline underline-offset-4" target="_blank" rel="noreferrer">{t("landing.github")}</a>
        </div>
      </div>
    </section>
  );
}
