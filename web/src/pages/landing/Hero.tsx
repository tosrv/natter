import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import CtaButtons from "./CtaButtons";

export interface HeroProps { onGoogle: () => void; onGuest: () => void; guestDisabled: boolean; productShot: ReactNode; children?: ReactNode }

export default function Hero({ onGoogle, onGuest, guestDisabled, productShot, children }: HeroProps) {
  const { t } = useTranslation();
  return (
    <section id="top" className="scroll-mt-16 px-4 pb-16 pt-16 sm:px-6 md:pb-24 md:pt-28">
      <div className="mx-auto flex max-w-4xl flex-col items-center text-center">
        <h1 className="mb-6 text-4xl font-semibold tracking-tight text-foreground sm:text-5xl md:text-7xl">{t("landing.hero.title")}</h1>
        <p className="mb-10 max-w-2xl text-lg text-muted-foreground md:text-xl">{t("landing.hero.sub")}</p>
        <CtaButtons onGoogle={onGoogle} onGuest={onGuest} guestDisabled={guestDisabled} />
        <p className="mt-4 text-xs text-muted-foreground">{t("landing.hero.guestNote")}</p>
        {children}
      </div>
      <div className="mx-auto mt-16 max-w-6xl md:mt-20">{productShot}</div>
    </section>
  );
}
