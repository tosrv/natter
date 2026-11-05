import { useTranslation } from "react-i18next";
import { Monitor, Moon, Sun } from "lucide-react";
import { cn } from "@/lib/utils";
import { choosePref, type ThemePref } from "@/lib/theme";
import { useThemePref } from "@/hooks/useTheme";

const OPTIONS: { pref: ThemePref; Icon: typeof Sun }[] = [
  { pref: "light", Icon: Sun },
  { pref: "dark", Icon: Moon },
  { pref: "system", Icon: Monitor },
];

export default function ThemeSwitch({ className }: { className?: string }) {
  const { t } = useTranslation();
  const pref = useThemePref();

  function choose(p: ThemePref) {
    choosePref(p);
  }

  function onKeyDown(e: React.KeyboardEvent<HTMLButtonElement>, i: number) {
    let idx: number;
    if (e.key === "Home") idx = 0;
    else if (e.key === "End") idx = OPTIONS.length - 1;
    else if (e.key === "ArrowRight" || e.key === "ArrowDown") idx = (i + 1) % OPTIONS.length;
    else if (e.key === "ArrowLeft" || e.key === "ArrowUp") idx = (i - 1 + OPTIONS.length) % OPTIONS.length;
    else return;
    e.preventDefault();
    const next = OPTIONS[idx].pref;
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
