import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { setLang, type Lang } from "@/i18n";

export default function LanguageToggle() {
  const { t, i18n } = useTranslation();
  const current: Lang = i18n.language === "id" ? "id" : "en";
  const next: Lang = current === "id" ? "en" : "id";
  // Accessible name includes the visible code (WCAG 2.5.3); the title describes the action.
  return (
    <Button variant="ghost" size="sm" onClick={() => setLang(next)} aria-label={`${t("common.language")}: ${current.toUpperCase()}`}
      title={t(next === "id" ? "common.switchToId" : "common.switchToEn")}>
      {current.toUpperCase()}
    </Button>
  );
}
