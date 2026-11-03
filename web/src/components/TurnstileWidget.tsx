import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { loadTurnstileScript } from "@/lib/turnstile";

interface Props {
  siteKey: string;
  onToken: (token: string | null) => void;
  /** Receives a reset function once the widget is rendered (null on unmount). */
  onReady?: (reset: (() => void) | null) => void;
}

export default function TurnstileWidget({ siteKey, onToken, onReady }: Props) {
  const { t } = useTranslation();
  const box = useRef<HTMLDivElement>(null);
  const cb = useRef({ onToken, onReady });
  cb.current = { onToken, onReady };
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let alive = true;
    let id: string | undefined;
    loadTurnstileScript().then(() => {
      if (!alive || !box.current || !window.turnstile) return;
      id = window.turnstile.render(box.current, {
        sitekey: siteKey,
        callback: (token) => cb.current.onToken(token),
        "expired-callback": () => cb.current.onToken(null),
        "error-callback": () => cb.current.onToken(null),
      });
      const rid = id;
      cb.current.onReady?.(() => { cb.current.onToken(null); window.turnstile?.reset(rid); });
    }).catch(() => { if (alive) setFailed(true); });
    return () => {
      alive = false;
      cb.current.onReady?.(null);
      if (id !== undefined) window.turnstile?.remove?.(id);
    };
  }, [siteKey]);

  if (failed) return <p role="alert" className="text-xs text-destructive">{t("landing.captchaLoadFailed")}</p>;
  return <div ref={box} data-testid="turnstile" />;
}
