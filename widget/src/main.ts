import { h, render } from "preact";
import css from "./styles.css?inline";
import { App } from "./ui/App";
import { createController } from "./controller";
import { createApi } from "./api";
import { createSession } from "./session";
import { createSocket } from "./socket";
import { pickLang } from "./i18n";

const scriptSrc = (document.currentScript as HTMLScriptElement | null)?.src;
let defaultServer = location.origin;
try { if (scriptSrc) defaultServer = new URL(scriptSrc).origin; } catch { /* keep fallback */ }

class NatterChat extends HTMLElement {
  private root?: ShadowRoot;
  connectedCallback() {
    if (this.root) return;
    try {
      const server = (this.getAttribute("server") || defaultServer).replace(/\/+$/, "");
      const root = (this.root = this.attachShadow({ mode: "open" }));
      // Constructable stylesheet: not subject to host `style-src`. Fallback to <style> if unsupported.
      let adopted = false;
      try {
        if (typeof CSSStyleSheet !== "undefined" && "adoptedStyleSheets" in root) {
          const sheet = new CSSStyleSheet();
          sheet.replaceSync(css);
          root.adoptedStyleSheets = [sheet];
          adopted = true;
        }
      } catch { /* fall back below */ }
      if (!adopted) {
        const style = document.createElement("style");
        style.textContent = css;
        root.appendChild(style);
      }
      const color = this.getAttribute("color");
      // A custom accent keeps white text and the light read-tick in both themes.
      if (color) for (const [k, v] of [["--pm-color", color], ["--pm-me", color], ["--pm-on", "#fff"], ["--pm-read", "#bfe9ff"]]) this.style.setProperty(k, v);
      const session = createSession(server);
      const api = createApi(server, () => session.get());
      const ws = server.replace(/^http/, "ws") + "/api/ws";
      const ctl = createController({
        api, session,
        socketFactory: (o) => createSocket({ ...o, url: ws }),
      });
      const mount = document.createElement("div");
      root.appendChild(mount);
      render(h(App, { ctl, lang: pickLang(this.getAttribute("lang")) }), mount);
    } catch (e) {
      console.warn("[natter]", e);
    }
  }
}

if (!customElements.get("natter-chat")) customElements.define("natter-chat", NatterChat);
