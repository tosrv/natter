import { useEffect, useRef, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import LanguageToggle from "@/components/LanguageToggle";
import ThemeToggle from "@/components/ThemeToggle";
import TurnstileWidget from "@/components/TurnstileWidget";
import LegalNote from "@/components/LegalNote";
import { toast } from "sonner";
import { ApiError, createGuest, getConfig, getMe } from "@/lib/api";
import { errorMessage } from "@/i18n";
import { cn } from "@/lib/utils";
import { AtSign, CheckCheck, Languages, LogIn, MessageCircle, MoreHorizontal, Send, ShieldAlert, SunMoon, UserRound, Zap } from "lucide-react";

export default function Landing() {
  const { t } = useTranslation();
  const nav = useNavigate();
  const [params] = useSearchParams();
  const [err, setErr] = useState<string | null>(params.get("error"));
  const [busy, setBusy] = useState(false);
  // undefined = loading, "" = Turnstile disabled, "error" = config unavailable (fail closed)
  const [siteKey, setSiteKey] = useState<string | undefined>();
  const [configFailed, setConfigFailed] = useState(false);
  const [token, setToken] = useState<string | null>(null);
  const resetWidget = useRef<(() => void) | null>(null);

  useEffect(() => {
    if (params.has("error")) window.history.replaceState(null, "", window.location.pathname);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    let alive = true;
    getMe().then((me) => {
      if (!alive) return;
      const needsProfile = !me.username || me.display_name === "User";
      nav(needsProfile ? "/onboarding" : "/chat", { replace: true });
    }).catch(() => { /* not logged in */ });
    return () => { alive = false; };
  }, [nav]);

  useEffect(() => {
    let alive = true;
    getConfig().then((c) => { if (alive) setSiteKey(c.turnstile_site_key ?? ""); })
      .catch(() => { if (alive) setConfigFailed(true); });
    return () => { alive = false; };
  }, []);

  const needsToken = !!siteKey;
  const guestDisabled = busy || configFailed || siteKey === undefined || (needsToken && !token);

  async function guest() {
    if (guestDisabled) return;
    setBusy(true);
    setErr(null);
    try {
      await createGuest(needsToken ? token ?? undefined : undefined);
      nav("/chat", { state: { newGuest: true } });
    } catch (e) {
      const code = e instanceof ApiError ? e.code : "internal";
      setErr(code);
      if (code === "captcha_failed") toast.error(errorMessage(code));
      // A Turnstile token is single-use: always get a fresh one after a failure.
      resetWidget.current?.();
      setToken(null);
      setBusy(false);
    }
  }

  const navLinks = [
    ["how", "landing.navHow"], ["features", "landing.navFeat"], ["privacy", "landing.navData"], ["faq", "landing.navFaq"],
  ] as const;
  const steps = [
    { icon: LogIn, title: t("landing.s1t"), desc: t("landing.s1d") },
    { icon: AtSign, title: t("landing.s2t"), desc: t("landing.s2d") },
    { icon: Send, title: t("landing.s3t"), desc: t("landing.s3d") },
  ];
  const grid = [
    { icon: AtSign, n: 1 }, { icon: Zap, n: 2 }, { icon: CheckCheck, n: 3 },
    { icon: MoreHorizontal, n: 4 }, { icon: SunMoon, n: 5 }, { icon: Languages, n: 6 },
  ];
  const features = [
    { icon: AtSign, text: t("landing.f1") },
    { icon: Zap, text: t("landing.f2") },
    { icon: UserRound, text: t("landing.f3") },
  ];

  return (
    <div className="flex min-h-screen flex-col bg-gradient-to-b from-primary/15 via-background dark:from-primary/10 to-background text-foreground">
      <header className="sticky top-0 z-20 bg-background/50 backdrop-blur-md">
        <div className="mx-auto flex h-14 w-full max-w-5xl items-center justify-between gap-4 px-4">
          <a href="#top" className="flex items-center gap-2 text-lg font-extrabold text-primary">
            <MessageCircle className="size-5" aria-hidden /> {t("app.name")}
          </a>
          <nav className="hidden items-center gap-6 text-sm text-muted-foreground md:flex">
            {navLinks.map(([id, key]) => (
              <a key={id} href={`#${id}`} className="transition-colors hover:text-foreground">{t(key)}</a>
            ))}
          </nav>
          <div className="flex items-center gap-1"><ThemeToggle /><LanguageToggle /></div>
        </div>
      </header>

      <div id="top" className="flex min-h-[calc(100svh-3.5rem)] scroll-mt-14 flex-col">
        <main className="mx-auto grid w-full max-w-5xl flex-1 items-center gap-10 px-4 py-8 md:grid-cols-2">
          <div className="flex flex-col gap-6">
            <div className="flex flex-col gap-3">
              <h1 className="text-4xl font-extrabold tracking-tight md:text-5xl">{t("landing.headline")}</h1>
              <p className="text-muted-foreground">{t("landing.sub")}</p>
            </div>
            {err && <p role="alert" className="rounded-2xl border border-destructive bg-card p-3 text-sm text-destructive">{errorMessage(err)}</p>}
            <div className="flex flex-col gap-3 sm:max-w-sm">
              <Button asChild size="lg" className="gap-3 rounded-full">
                <a href="/api/auth/google/login"><span className="flex size-6 items-center justify-center rounded-full bg-white"><GoogleIcon /></span> {t("landing.google")}</a>
              </Button>
              <div className="flex flex-col gap-1">
                <Button variant="outline" size="lg" className="rounded-full" onClick={guest} disabled={guestDisabled}>{t("landing.guest")}</Button>
                <span className="text-center text-xs text-muted-foreground">{t("landing.guestNote")}</span>
              </div>
              {configFailed && <p role="alert" className="text-center text-xs text-destructive">{t("landing.configFailed")}</p>}
              {needsToken && (
                <div className="flex justify-center">
                  <TurnstileWidget siteKey={siteKey!} onToken={setToken} onReady={(r) => { resetWidget.current = r; }} />
                </div>
              )}
              <LegalNote className="text-center text-xs text-muted-foreground" />
            </div>
            <ul className="flex flex-col gap-2 text-sm sm:flex-row sm:flex-wrap sm:gap-4">
              {features.map(({ icon: Icon, text }) => (
                <li key={text} className="flex items-center gap-2 text-muted-foreground">
                  <Icon className="size-4 text-primary" aria-hidden /> {text}
                </li>
              ))}
            </ul>
          </div>

          <div aria-hidden className="mx-auto w-full max-w-sm overflow-hidden rounded-3xl border bg-card shadow-xl">
            <div className="flex items-center gap-3 border-b px-4 py-3">
              <div className="flex size-9 items-center justify-center rounded-full bg-primary/15 font-bold text-primary">A</div>
              <div className="text-left">
                <p className="text-sm font-semibold">Alya</p>
                <p className="text-xs text-primary">{t("landing.online")}</p>
              </div>
            </div>
            <div className="flex flex-col gap-2 p-4 text-left text-sm">
              <Bubble>{t("landing.bubbleIn")}</Bubble>
              <Bubble out>{t("landing.bubbleOut")}</Bubble>
              <Bubble>{t("landing.bubble3")}</Bubble>
              <Bubble out read>{t("landing.bubble4")}</Bubble>
              <p className="text-xs italic text-muted-foreground">Alya {t("landing.typing")}</p>
            </div>
          </div>
        </main>
      </div>

      <Section id="how" title={t("landing.howTitle")}>
        <ol className="grid gap-4 md:grid-cols-3">
          {steps.map(({ icon: Icon, title, desc }, i) => (
            <li key={title} className="flex flex-col gap-2 rounded-2xl border bg-card p-5">
              <span className="flex items-center gap-2 text-sm font-semibold text-primary">
                <span className="flex size-7 items-center justify-center rounded-full bg-primary text-primary-foreground">{i + 1}</span>
                <Icon className="size-4" aria-hidden />
              </span>
              <h3 className="font-bold">{title}</h3>
              <p className="text-sm text-muted-foreground">{desc}</p>
            </li>
          ))}
        </ol>
      </Section>

      <Section id="features" title={t("landing.featTitle")}>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {grid.map(({ icon: Icon, n }) => (
            <div key={n} className="flex gap-3 rounded-2xl border bg-card p-5">
              <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/15 text-primary"><Icon className="size-5" aria-hidden /></span>
              <div>
                <h3 className="font-bold">{t(`landing.ft${n}`)}</h3>
                <p className="text-sm text-muted-foreground">{t(`landing.fd${n}`)}</p>
              </div>
            </div>
          ))}
        </div>
      </Section>

      <Section id="privacy" title={t("landing.trustTitle")} sub={t("landing.trustSub")}>
        <ul className="flex flex-col gap-3 rounded-2xl border bg-card p-5">
          {[1, 2, 3].map((n) => (
            <li key={n} className="flex items-start gap-3 text-sm">
              <ShieldAlert className="mt-0.5 size-4 shrink-0 text-primary" aria-hidden /> {t(`landing.tr${n}`)}
            </li>
          ))}
        </ul>
        <Link to="/privacy" className="mt-3 inline-block text-sm text-primary underline">{t("landing.info")}</Link>
      </Section>

      <Section id="faq" title={t("landing.faqTitle")}>
        <div className="flex flex-col gap-3">
          {[1, 2, 3, 4].map((n) => (
            <details key={n} className="group rounded-2xl border bg-card p-5">
              <summary className="cursor-pointer list-none font-semibold marker:hidden">
                <span className="flex items-center justify-between gap-2">{t(`landing.q${n}`)}<span className="text-primary transition-transform group-open:rotate-45">+</span></span>
              </summary>
              <p className="mt-2 text-sm text-muted-foreground">{t(`landing.a${n}`)}</p>
            </details>
          ))}
        </div>
      </Section>

      <section className="mx-auto w-full max-w-5xl px-4 py-12">
        <div className="flex flex-col items-center gap-4 rounded-3xl bg-primary px-6 py-10 text-center text-primary-foreground">
          <h2 className="text-3xl font-extrabold tracking-tight">{t("landing.ctaTitle")}</h2>
          <p className="opacity-90">{t("landing.ctaSub")}</p>
          <div className="flex flex-col gap-3 sm:flex-row">
            <Button asChild size="lg" variant="secondary" className="gap-3 rounded-full">
              <a href="/api/auth/google/login"><GoogleIcon /> {t("landing.google")}</a>
            </Button>
            <Button size="lg" variant="ghost" className="rounded-full border border-primary-foreground/40 hover:bg-primary-foreground/10 hover:text-primary-foreground" onClick={guest} disabled={guestDisabled}>
              {t("landing.ctaGuest")}
            </Button>
          </div>
        </div>
      </section>

      <footer className="mx-auto flex w-full max-w-5xl items-center justify-between gap-2 border-t px-4 py-4 text-xs text-muted-foreground">
        <span>© {new Date().getFullYear()} {t("app.name")}. {t("landing.rights")}</span>
        <span className="flex items-center gap-3">
          <Link to="/terms" className="underline">{t("landing.terms")}</Link>
          <Link to="/privacy" className="underline">{t("landing.info")}</Link>
        </span>
      </footer>
    </div>
  );
}

function Section({ id, title, sub, children }: { id: string; title: string; sub?: string; children: React.ReactNode }) {
  return (
    <section id={id} className="mx-auto w-full max-w-5xl scroll-mt-14 px-4 py-12">
      <h2 className="text-2xl font-extrabold tracking-tight md:text-3xl">{title}</h2>
      {sub && <p className="mt-2 text-muted-foreground">{sub}</p>}
      <div className="mt-6">{children}</div>
    </section>
  );
}

function Bubble({ out, read, children }: { out?: boolean; read?: boolean; children: React.ReactNode }) {
  return (
    <div className={cn("max-w-[80%] rounded-2xl px-3 py-2 shadow-sm",
      out ? "self-end rounded-br-sm bg-primary text-primary-foreground" : "self-start rounded-bl-sm border bg-background")}>
      {children}
      {read && <CheckCheck className="ml-1 inline size-3.5 opacity-80" />}
    </div>
  );
}

function GoogleIcon() {
  return (
    <svg viewBox="0 0 24 24" className="size-4" aria-hidden>
      <path fill="#FFC107" d="M21.8 10.2H12v3.9h5.6c-.5 2.5-2.7 4.3-5.6 4.3a6.4 6.4 0 1 1 4.2-11.2l2.8-2.8A10.3 10.3 0 1 0 22 12c0-.6-.1-1.2-.2-1.8z" />
      <path fill="#FF3D00" d="M3.2 7.1l3.2 2.4A6.4 6.4 0 0 1 16.2 7.2L19 4.4A10.3 10.3 0 0 0 3.2 7.1z" />
      <path fill="#4CAF50" d="M12 22.3c2.7 0 5.1-1 6.9-2.7l-3.2-2.6A6.4 6.4 0 0 1 6.4 14.5l-3.2 2.5A10.3 10.3 0 0 0 12 22.3z" />
      <path fill="#1976D2" d="M21.8 10.2H12v3.9h5.6a6.4 6.4 0 0 1-2 2.9l3.2 2.6c-.2.2 3.2-2.4 3.2-7.6 0-.6-.1-1.2-.2-1.8z" />
    </svg>
  );
}
