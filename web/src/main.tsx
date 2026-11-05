import React, { useEffect } from "react";
import ReactDOM from "react-dom/client";
import i18n, { detectLang } from "./i18n";
import "./index.css";
import App from "./App";
import { applyTheme, getThemePref } from "./lib/theme";

applyTheme(getThemePref());

const PRERENDERED = ["/", "/privacy", "/terms"];
const rootEl = document.getElementById("root")!;
const pre = rootEl.hasChildNodes() && PRERENDERED.includes(location.pathname.replace(/\/$/, "") || "/");

/** Switches to the visitor's language only once hydration has committed. */
function AfterHydrate({ lang }: { lang: string }) {
  useEffect(() => { if (i18n.language !== lang) void i18n.changeLanguage(lang); }, [lang]);
  return null;
}

if (pre) {
  // Prerendered markup is English; the first client render must match it exactly.
  const lang = detectLang();
  void i18n.changeLanguage("en");
  ReactDOM.hydrateRoot(rootEl, <React.StrictMode><App><AfterHydrate lang={lang} /></App></React.StrictMode>, {
    onRecoverableError: (e) => console.error("hydration error:", e),
  });
} else {
  ReactDOM.createRoot(rootEl).render(<React.StrictMode><App /></React.StrictMode>);
}
