import { renderToString } from "react-dom/server";
import { StaticRouter } from "react-router-dom/server";
import { createInstance } from "i18next";
import { I18nextProvider, initReactI18next } from "react-i18next";
import en from "./i18n/en.json";
import id from "./i18n/id.json";
import { AppRoutes } from "./App";
import type { PublicRoute } from "./seo/meta";

export { headTags, pageMeta } from "./seo/meta";

/** Renders a public route to HTML in English (the client hydrates in English, then switches). */
export function render(route: PublicRoute): string {
  const i18n = createInstance();
  void i18n.use(initReactI18next).init({
    resources: { en: { translation: en }, id: { translation: id } },
    lng: "en",
    fallbackLng: "en",
    interpolation: { escapeValue: false },
  });
  return renderToString(
    <I18nextProvider i18n={i18n}>
      <StaticRouter location={route}><AppRoutes /></StaticRouter>
    </I18nextProvider>,
  );
}
