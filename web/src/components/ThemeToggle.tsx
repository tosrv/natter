import { useTranslation } from "react-i18next";
import { Moon, Sun } from "lucide-react";
import { cn } from "@/lib/utils";
import { choosePref } from "@/lib/theme";
import { useIsDark } from "@/hooks/useTheme";

/** Single borderless button that flips between light and dark (no "system" option). */
export default function ThemeToggle({ className }: { className?: string }) {
  const { t } = useTranslation();
  const dark = useIsDark();

  function toggle() {
    choosePref(dark ? "light" : "dark");
  }

  const label = t(dark ? "theme.light" : "theme.dark");
  const Icon = dark ? Sun : Moon;
  return (
    <button
      type="button"
      onClick={toggle}
      aria-label={label}
      title={label}
      className={cn(
        "inline-flex h-8 w-8 items-center justify-center rounded-full text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        className,
      )}
    >
      <Icon className="h-4 w-4" strokeWidth={1.5} aria-hidden />
    </button>
  );
}
