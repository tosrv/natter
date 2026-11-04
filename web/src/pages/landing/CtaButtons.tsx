import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import GoogleIcon from "./GoogleIcon";

export const GOOGLE_LOGIN = "/api/auth/google/login";

export interface CtaProps { onGoogle: () => void; onGuest: () => void; guestDisabled: boolean }

export default function CtaButtons({ onGoogle, onGuest, guestDisabled }: CtaProps) {
  const { t } = useTranslation();
  return (
    <div className="flex w-full flex-col items-center justify-center gap-3 sm:w-auto sm:flex-row">
      <Button asChild size="lg" className="w-full gap-2 sm:w-auto">
        <a href={GOOGLE_LOGIN} onClick={onGoogle}><GoogleIcon /> {t("landing.hero.google")}</a>
      </Button>
      <Button variant="outline" size="lg" className="w-full sm:w-auto" onClick={onGuest} disabled={guestDisabled}>
        {t("landing.hero.guest")}
      </Button>
    </div>
  );
}
