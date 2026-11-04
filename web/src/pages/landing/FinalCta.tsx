import { useTranslation } from "react-i18next";
import Reveal from "./Reveal";
import CtaButtons, { type CtaProps } from "./CtaButtons";

export default function FinalCta(props: CtaProps) {
  const { t } = useTranslation();
  return (
    <section className="px-4 py-20 sm:px-6 md:py-24">
      <div className="mx-auto flex max-w-2xl flex-col items-center text-center">
        <Reveal as="h2" className="mb-8 text-3xl font-semibold tracking-tight">{t("landing.cta.title")}</Reveal>
        <CtaButtons {...props} />
      </div>
    </section>
  );
}
