import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import en from "./en.json";
import id from "./id.json";

export type Lang = "en" | "id";
const KEY = "natter.lang";

export function detectLang(): Lang {
  try {
    const s = localStorage.getItem(KEY);
    if (s === "en" || s === "id") return s;
  } catch { /* storage unavailable */ }
  const n = typeof navigator !== "undefined" ? navigator.language ?? "" : "";
  return n.toLowerCase().startsWith("id") ? "id" : "en";
}

export function setLang(l: Lang) {
  try { localStorage.setItem(KEY, l); } catch { /* ignore */ }
  void i18n.changeLanguage(l);
}

void i18n.use(initReactI18next).init({
  resources: { en: { translation: en }, id: { translation: id } },
  lng: detectLang(),
  fallbackLng: "en",
  interpolation: { escapeValue: false },
});
if (typeof document !== "undefined") {
  document.documentElement.lang = i18n.language;
  i18n.on("languageChanged", (l) => { document.documentElement.lang = l; });
}

export function errorMessage(code: string): string {
  return i18n.exists(`errors.${code}`) ? i18n.t(`errors.${code}`) : i18n.t("errors.unknown");
}

export default i18n;
