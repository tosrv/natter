import { useTranslation } from "react-i18next";

export default function Philosophy() {
  const { t } = useTranslation();
  return (
    <section className="bg-surface px-4 py-24 sm:px-6 md:py-32">
      <p className="mx-auto max-w-4xl text-balance text-center text-4xl font-semibold tracking-tight md:text-6xl">
        {t("landing.philosophy")}
      </p>
    </section>
  );
}
