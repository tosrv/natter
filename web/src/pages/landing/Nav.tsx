import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Menu } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import ThemeToggle from "@/components/ThemeToggle";
import LanguageToggle from "@/components/LanguageToggle";
import Wordmark from "@/components/Wordmark";
import { cn } from "@/lib/utils";
import { GOOGLE_LOGIN } from "./CtaButtons";

const LINKS = [["product", "landing.nav.product"], ["how", "landing.nav.how"], ["privacy", "landing.nav.privacy"]] as const;

export default function Nav() {
  const { t } = useTranslation();
  const [scrolled, setScrolled] = useState(false);
  useEffect(() => {
    const on = () => setScrolled(window.scrollY > 4);
    on();
    window.addEventListener("scroll", on, { passive: true });
    return () => window.removeEventListener("scroll", on);
  }, []);

  return (
    <header className={cn("sticky top-0 z-40 border-b bg-background/90 backdrop-blur-md transition-colors", scrolled ? "border-border" : "border-transparent")}>
      <div className="mx-auto flex h-16 w-full max-w-6xl items-center justify-between gap-2 px-4 sm:px-6">
        <div className="flex items-center gap-8">
          <a href="#top" className="rounded-md text-foreground transition-opacity hover:opacity-80 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
            <Wordmark className="h-5 w-auto sm:h-6" />
          </a>
          <nav aria-label="Primary" className="hidden items-center gap-6 text-sm text-muted-foreground md:flex">
            {LINKS.map(([id, key]) => (
              <a key={id} href={`#${id}`} className="rounded-sm transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">{t(key)}</a>
            ))}
          </nav>
        </div>
        <div className="flex items-center gap-1.5 sm:gap-2">
          <div className="flex items-center gap-1 sm:border-r sm:pr-3">
            <ThemeToggle />
            <LanguageToggle />
          </div>
          <Button asChild size="sm"><a href={GOOGLE_LOGIN}>{t("landing.nav.signIn")}</a></Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="size-9 md:hidden" aria-label={t("landing.nav.menu")}><Menu className="size-5" aria-hidden /></Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-48">
              {LINKS.map(([id, key]) => (
                <DropdownMenuItem key={id} asChild><a href={`#${id}`}>{t(key)}</a></DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </header>
  );
}
