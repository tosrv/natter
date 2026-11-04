import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Monitor, Moon, Sun } from "lucide-react";
import { cn } from "@/lib/utils";
import { applyTheme, getThemePref, setThemePref, type ThemePref } from "@/lib/theme";

const OPTIONS: { pref: ThemePref; Icon: typeof Sun }[] = [
  { pref: "light", Icon: Sun },
  { pref: "dark", Icon: Moon },
  { pref: "system", Icon: Monitor },
];

export default function ThemeSwitch({ className }: { className?: string }) {
  const { t } = useTranslation();
  const [pref, setPref] = useState<ThemePref>(getThemePref);

  function choose(p: ThemePref) {
    setThemePref(p);
    applyTheme(p);
    setPref(p);
  }

  function onKeyDown(e: React.KeyboardEvent<HTMLButtonElement>, i: number) {
    const step = e.key === "ArrowRight" || e.key === "ArrowDown" ? 1 : e.key === "ArrowLeft" || e.key === "ArrowUp" ? -1 : 0;
    if (!step) return;
    e.preventDefault();
    const next = OPTIONS[(i + step + OPTIONS.length) % OPTIONS.length].pref;
    choose(next);
    const group = e.currentTarget.parentElement;
    requestAnimationFrame(() => group?.querySelector<HTMLButtonElement>(`[data-pref="${next}"]`)?.focus());
  }

  return (
    <div role="radiogroup" aria-label={t("theme.label")} className={cn("inline-flex items-center rounded-md border border-border bg-background p-0.5", className)}>
      {OPTIONS.map(({ pref: p, Icon }, i) => {
        const checked = pref === p;
        return (
          <button
            key={p}
            type="button"
            role="radio"
            aria-checked={checked}
            aria-label={t(`theme.${p}`)}
            title={t(`theme.${p}`)}
            data-pref={p}
            tabIndex={checked ? 0 : -1}
            onClick={() => choose(p)}
            onKeyDown={(e) => onKeyDown(e, i)}
            className={cn(
              "inline-flex h-7 w-7 items-center justify-center rounded-sm text-muted-foreground transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
              checked ? "bg-accent text-foreground" : "hover:text-foreground",
            )}
          >
            <Icon className="h-4 w-4" strokeWidth={1.5} aria-hidden />
          </button>
        );
      })}
    </div>
  );
}
