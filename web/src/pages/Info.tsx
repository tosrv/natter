import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import LanguageToggle from "@/components/LanguageToggle";

export default function Info() {
  const { t } = useTranslation();
  return (
    <main className="mx-auto max-w-xl space-y-4 p-6">
      <div className="flex items-center justify-between">
        <Link to="/" className="text-sm underline">{t("common.back")}</Link>
        <LanguageToggle />
      </div>
      <h1 className="text-2xl font-bold">{t("info.title")}</h1>
      <ul className="list-disc space-y-3 pl-5">
        <li>{t("info.e2e")}</li>
        <li>{t("info.guest")}</li>
        <li>{t("info.retention")}</li>
        <li>{t("info.contact")}</li>
      </ul>
    </main>
  );
}
