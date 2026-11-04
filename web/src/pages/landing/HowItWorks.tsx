import { useTranslation } from "react-i18next";
import Reveal from "./Reveal";

export default function HowItWorks() {
  const { t } = useTranslation();
  return (
    <section id="how" className="scroll-mt-16 border-y px-4 py-20 sm:px-6 md:py-24">
      <div className="mx-auto max-w-5xl">
        <Reveal as="h2" className="mb-12 text-center text-2xl font-semibold tracking-tight md:mb-16">{t("landing.how.title")}</Reveal>
        <ol className="relative grid gap-10 sm:grid-cols-3 sm:gap-8">
          <div aria-hidden className="absolute left-[16.67%] right-[16.67%] top-6 hidden h-px bg-border sm:block" />
          {[1, 2, 3].map((n) => (
            <li key={n} className="relative flex flex-col items-center text-center">
              <span aria-hidden className="mb-4 flex size-12 items-center justify-center rounded-full border bg-background font-semibold text-primary-text shadow-sm">{n}</span>
              <h3 className="mb-2 font-medium">{t(`landing.how.s${n}.t`)}</h3>
              <p className="max-w-xs text-sm text-muted-foreground">{t(`landing.how.s${n}.d`)}</p>
            </li>
          ))}
        </ol>
      </div>
    </section>
  );
}
