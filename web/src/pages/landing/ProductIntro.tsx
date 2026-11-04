import { useTranslation } from "react-i18next";
import Reveal from "./Reveal";

export default function ProductIntro() {
  const { t } = useTranslation();
  return (
    <section id="product" className="scroll-mt-16 bg-surface px-4 py-20 sm:px-6 md:py-24">
      <div className="mx-auto grid max-w-5xl items-center gap-10 md:grid-cols-2 md:gap-12">
        <div>
          <Reveal as="h2" className="mb-4 text-3xl font-semibold tracking-tight">{t("landing.intro.title")}</Reveal>
          <p className="leading-relaxed text-muted-foreground">{t("landing.intro.body")}</p>
        </div>
        <blockquote className="border-l-2 border-primary pl-8">
          <p className="text-xl font-medium italic">“{t("landing.philosophy")}”</p>
        </blockquote>
      </div>
    </section>
  );
}
