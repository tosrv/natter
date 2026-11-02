import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Moon, Sun } from "lucide-react";
import { Button } from "@/components/ui/button";
import { applyTheme, setThemePref } from "@/lib/theme";

const isDark = () => document.documentElement.classList.contains("dark");

export default function ThemeToggle() {
  const { t } = useTranslation();
  const [dark, setDark] = useState(isDark);
  function toggle() {
    const next = dark ? "light" : "dark";
    setThemePref(next);
    applyTheme(next);
    setDark(!dark);
  }
  const target = dark ? "light" : "dark";
  return (
    <Button variant="ghost" size="sm" onClick={toggle} aria-label={`${t("theme.label")}: ${t(`theme.${target}`)}`} title={t(`theme.${target}`)}>
      {dark ? <Sun className="h-4 w-4" aria-hidden /> : <Moon className="h-4 w-4" aria-hidden />}
    </Button>
  );
}
