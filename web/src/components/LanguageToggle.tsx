import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { setLang, type Lang } from "@/i18n";

export default function LanguageToggle() {
  const { i18n } = useTranslation();
  const next: Lang = i18n.language === "id" ? "en" : "id";
  return (
    <Button variant="ghost" size="sm" onClick={() => setLang(next)} aria-label="Language">
      {i18n.language === "id" ? "ID" : "EN"} → {next.toUpperCase()}
    </Button>
  );
}
