import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Monitor, Moon, Sun } from "lucide-react";
import { Button } from "@/components/ui/button";
import { applyTheme, getThemePref, setThemePref, type ThemePref } from "@/lib/theme";

const ORDER: ThemePref[] = ["light", "dark", "system"];
const ICON = { light: Sun, dark: Moon, system: Monitor } as const;

export default function ThemeToggle() {
  const { t } = useTranslation();
  const [pref, setPref] = useState<ThemePref>(getThemePref);
  const Icon = ICON[pref];
  function cycle() {
    const next = ORDER[(ORDER.indexOf(pref) + 1) % ORDER.length];
    setThemePref(next);
    applyTheme(next);
    setPref(next);
  }
  return (
    <Button variant="ghost" size="sm" onClick={cycle} aria-label={`${t("theme.label")}: ${t(`theme.${pref}`)}`} title={t(`theme.${pref}`)}>
      <Icon className="h-4 w-4" aria-hidden />
    </Button>
  );
}
