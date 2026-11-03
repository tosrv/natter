export const TURNSTILE_SRC = "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";

export interface TurnstileApi {
  render(container: HTMLElement, opts: {
    sitekey: string;
    callback: (token: string) => void;
    "expired-callback": () => void;
    "error-callback": () => void;
  }): string;
  reset(id: string): void;
  remove?(id: string): void;
}

declare global {
  interface Window { turnstile?: TurnstileApi }
}

let pending: Promise<void> | null = null;

/** Loads the Turnstile script once. Rejects on error or timeout; a later call retries. */
export function loadTurnstileScript(timeoutMs = 10000): Promise<void> {
  if (pending) return pending;
  if (window.turnstile) return Promise.resolve();
  pending = new Promise<void>((resolve, reject) => {
    const s = document.createElement("script");
    s.src = TURNSTILE_SRC;
    s.async = true;
    s.defer = true;
    const timer = setTimeout(() => fail(new Error("turnstile script timeout")), timeoutMs);
    function fail(err: Error) {
      clearTimeout(timer);
      s.remove();
      pending = null;
      reject(err);
    }
    s.addEventListener("load", () => { clearTimeout(timer); resolve(); });
    s.addEventListener("error", () => fail(new Error("turnstile script failed to load")));
    document.head.appendChild(s);
  });
  return pending;
}
