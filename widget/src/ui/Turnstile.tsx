import { useEffect, useRef } from "preact/hooks";

const SRC = "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";
let loading: Promise<void> | null = null;

declare global {
  interface Window { turnstile?: { render(el: HTMLElement, o: object): string; remove?(id: string): void; reset?(id: string): void } }
}

function load(): Promise<void> {
  if (window.turnstile) return Promise.resolve();
  if (!loading) {
    loading = new Promise((res, rej) => {
      const s = document.createElement("script");
      s.src = SRC; s.async = true;
      s.onload = () => res();
      s.onerror = () => { loading = null; rej(new Error("turnstile load failed")); };
      document.head.appendChild(s);
    });
  }
  return loading;
}

export function Turnstile({ siteKey, onToken, onError, onReady }: { siteKey: string; onToken: (t: string) => void; onError: (e: unknown) => void; onReady?: (reset: () => void) => void }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    let id: string | undefined;
    let alive = true;
    load().then(() => {
      if (alive && ref.current && window.turnstile) id = window.turnstile.render(ref.current, {
        sitekey: siteKey, callback: onToken,
        "error-callback": () => onError(new Error("turnstile error")),
        "expired-callback": () => { onToken(""); if (id) window.turnstile?.reset?.(id); },
      });
      if (id) onReady?.(() => { if (id) window.turnstile?.reset?.(id); });
    }, onError);
    return () => { alive = false; if (id) window.turnstile?.remove?.(id); };
  }, [siteKey]);
  return <div class="pm-ts" ref={ref} />;
}
