import { useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import GuestStartDialog from "@/components/GuestStartDialog";
import LegalNote from "@/components/LegalNote";
import { getConfig, getMe } from "@/lib/api";
import { errorMessage } from "@/i18n";
import Nav from "./landing/Nav";
import Hero, { ProductShotPlaceholder } from "./landing/Hero";
import ProductIntro from "./landing/ProductIntro";
import Features from "./landing/Features";
import HowItWorks from "./landing/HowItWorks";
import Philosophy from "./landing/Philosophy";
import Privacy from "./landing/Privacy";
import FinalCta from "./landing/FinalCta";
import Footer from "./landing/Footer";

export default function Landing() {
  const { t } = useTranslation();
  const nav = useNavigate();
  const [params] = useSearchParams();
  const [err, setErr] = useState<string | null>(params.get("error"));
  const [guestOpen, setGuestOpen] = useState(false);
  // undefined = loading, "" = Turnstile disabled, "error" = config unavailable (fail closed)
  const [siteKey, setSiteKey] = useState<string | undefined>();
  const [configFailed, setConfigFailed] = useState(false);

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

  const guestDisabled = configFailed || siteKey === undefined;
  const guest = () => { if (!guestDisabled) { setErr(null); setGuestOpen(true); } };
  // The Google CTA is a plain link to /api/auth/google/login; clearing a stale error is all it needs.
  const google = () => setErr(null);
  const cta = { onGoogle: google, onGuest: guest, guestDisabled };

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <a href="#main" className="sr-only z-50 rounded-md bg-background px-4 py-2 text-sm focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:ring-2 focus:ring-ring">
        {t("landing.nav.skip")}
      </a>
      <Nav />
      <main id="main" className="flex-1">
        <Hero {...cta} productShot={<ProductShotPlaceholder />}>
          {err && <p role="alert" className="mt-6 rounded-lg border border-destructive bg-card p-3 text-sm text-destructive">{errorMessage(err)}</p>}
          {configFailed && <p role="alert" className="mt-4 text-xs text-destructive">{t("landing.configFailed")}</p>}
          <LegalNote className="mt-2 text-xs text-muted-foreground" />
        </Hero>
        <ProductIntro />
        <Features />
        <HowItWorks />
        <Philosophy />
        <Privacy />
        <FinalCta {...cta} />
      </main>
      <Footer />
      <GuestStartDialog open={guestOpen} onClose={() => setGuestOpen(false)} siteKey={siteKey ?? ""} />
    </div>
  );
}
