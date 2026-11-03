import { Link } from "react-router-dom";
import { Trans, useTranslation } from "react-i18next";

export default function LegalNote({ className }: { className?: string }) {
  const { t } = useTranslation();
  const cls = "underline hover:text-foreground";
  return (
    <p className={className ?? "text-xs text-muted-foreground"}>
      <Trans
        i18nKey="landing.consent"
        values={{ terms: t("landing.terms"), privacy: t("landing.privacy") }}
        components={{
          terms: <Link to="/terms" className={cls} />,
          privacy: <Link to="/privacy" className={cls} />,
        }}
      />
    </p>
  );
}
