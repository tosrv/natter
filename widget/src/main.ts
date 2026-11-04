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

class PmessChat extends HTMLElement {
  private root?: ShadowRoot;
  connectedCallback() {
    if (this.root) return;
    try {
      const server = (this.getAttribute("server") || defaultServer).replace(/\/+$/, "");
      const root = (this.root = this.attachShadow({ mode: "open" }));
      const style = document.createElement("style");
      style.textContent = css;
      root.appendChild(style);
      const color = this.getAttribute("color");
      if (color) this.style.setProperty("--pm-color", color);
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
      console.warn("[pmess]", e);
    }
  }
}

if (!customElements.get("pmess-chat")) customElements.define("pmess-chat", PmessChat);
