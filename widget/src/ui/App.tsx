import { useEffect, useState } from "preact/hooks";
import type { Controller } from "../controller";
import { t, type Lang } from "../i18n";
import { Launcher } from "./Launcher";
import { Panel } from "./Panel";

export function App({ ctl, lang }: { ctl: Controller; lang: Lang }) {
  const [, tick] = useState(0);
  const [open, setOpen] = useState(false);
  useEffect(() => ctl.subscribe(() => tick((n) => n + 1)), [ctl]);
  const toggle = () => {
    const o = !open;
    setOpen(o);
    if (o) ctl.open(); else ctl.close();
  };
  return (
    <div class="pm">
      {open && <Panel ctl={ctl} lang={lang} onClose={toggle} />}
      <Launcher open={open} label={t(lang, open ? "close" : "open")} onClick={toggle} />
    </div>
  );
}
