import { useTranslation } from "react-i18next";
import Reveal from "./Reveal";
import { AtSign, CheckCheck, MoreHorizontal, ShieldCheck, Zap } from "lucide-react";

const ITEMS = [
  ["search", AtSign], ["realtime", Zap], ["receipts", CheckCheck], ["typing", MoreHorizontal], ["controls", ShieldCheck],
] as const;

export default function Features({ shot }: { shot?: React.ReactNode }) {
  const { t } = useTranslation();
  return (
    <section className="px-4 py-20 sm:px-6 md:py-24">
      <div className="mx-auto grid max-w-5xl items-center gap-12 md:grid-cols-2 md:gap-16">
        <div>
          <Reveal as="h2" className="mb-8 text-2xl font-semibold tracking-tight">{t("landing.features.title")}</Reveal>
          <ul className="space-y-6">
            {ITEMS.map(([k, Icon]) => (
              <li key={k} className="flex gap-4">
                <Icon className="mt-0.5 size-5 shrink-0 text-primary-text" aria-hidden />
                <div>
                  <h3 className="font-medium">{t(`landing.features.${k}.t`)}</h3>
                  <p className="mt-1 text-sm text-muted-foreground">{t(`landing.features.${k}.d`)}</p>
                </div>
              </li>
            ))}
          </ul>
        </div>
        <Reveal>{shot ?? <div aria-hidden className="aspect-[4/3] w-full rounded-xl border bg-surface shadow-sm" />}</Reveal>
      </div>
    </section>
  );
}
