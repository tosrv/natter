import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { ArrowLeft } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import DeleteAccountDialog from "@/components/DeleteAccountDialog";
import { ApiError, deleteAccount, getMe, logout, setDisplayName, setUsername, type Me } from "@/lib/api";
import { validateDisplayName } from "@/lib/format";
import { applyTheme, getThemePref, setThemePref, type ThemePref } from "@/lib/theme";
import { errorMessage, setLang, type Lang } from "@/i18n";

const USERNAME_RE = /^[a-z0-9_]{3,20}$/;
const THEMES: ThemePref[] = ["light", "dark", "system"];

function Card({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="grid gap-3 border-b border-border py-6 last:border-b-0 md:grid-cols-[14rem_1fr] md:gap-8">
      <h2 className="text-lg font-extrabold md:text-base">{title}</h2>
      <div className="flex min-w-0 flex-col gap-3">{children}</div>
    </section>
  );
}

function Segment({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <Button type="button" size="sm" variant={active ? "default" : "outline"} className="rounded-full" aria-pressed={active} onClick={onClick}>
      {children}
    </Button>
  );
}

export default function Settings() {
  const { t, i18n } = useTranslation();
  const nav = useNavigate();
  const [me, setMe] = useState<Me | null>(null);
  const [display, setDisplay] = useState("");
  const [username, setUser] = useState("");
  const [errs, setErrs] = useState<{ display?: string; username?: string; del?: string }>({});
  const [busy, setBusy] = useState(false);
  const [delOpen, setDelOpen] = useState(false);
  const [theme, setTheme] = useState<ThemePref>(getThemePref);

  const home = () => nav("/", { replace: true });
  const handle = (e: unknown): string | undefined => {
    if (e instanceof ApiError && e.status === 401) { home(); return undefined; }
    return errorMessage(e instanceof ApiError ? e.code : "internal");
  };

  useEffect(() => {
    getMe().then((m) => { setMe(m); setDisplay(m.display_name); setUser(m.username ?? ""); }).catch(() => nav("/", { replace: true }));
  }, [nav]);

  if (!me) return <main className="flex min-h-screen items-center justify-center text-muted-foreground">{t("common.loading")}</main>;

  const guest = me.kind === "guest";
  const dn = validateDisplayName(display);
  const dnShown = display.length > 0 && !dn.ok;
  const uname = username.trim().toLowerCase();
  const unShown = uname.length > 0 && !USERNAME_RE.test(uname);

  async function saveDisplay(e: React.FormEvent) {
    e.preventDefault();
    if (!dn.ok) { setErrs((s) => ({ ...s, display: errorMessage("display_name_invalid") })); return; }
    setBusy(true);
    try {
      const m = await setDisplayName(dn.value);
      setMe(m); setDisplay(m.display_name); setErrs((s) => ({ ...s, display: undefined }));
      toast.success(t("settings.saved"));
    } catch (err) { setErrs((s) => ({ ...s, display: handle(err) })); }
    setBusy(false);
  }

  async function saveUsername(e: React.FormEvent) {
    e.preventDefault();
    if (!USERNAME_RE.test(uname)) { setErrs((s) => ({ ...s, username: errorMessage("username_invalid") })); return; }
    setBusy(true);
    try {
      const m = await setUsername(uname);
      setMe(m); setUser(m.username ?? ""); setErrs((s) => ({ ...s, username: undefined }));
      toast.success(t("settings.saved"));
    } catch (err) { setErrs((s) => ({ ...s, username: handle(err) })); }
    setBusy(false);
  }

  async function doLogout() {
    try { await logout(); } catch { /* ignore */ }
    home();
  }

  async function doDelete(fromDialog: boolean) {
    setBusy(true);
    try {
      await deleteAccount();
      home();
    } catch (err) {
      const msg = handle(err);
      if (fromDialog) setErrs((s) => ({ ...s, del: msg }));
      else if (msg) toast.error(msg);
      setBusy(false);
    }
  }

  const noLeft = me.username_changes_left <= 0;
  const fieldMsg = (id: string, err?: string, shown?: boolean, hint?: string, invalidMsg?: string) => (
    <p id={id} role={err || shown ? "alert" : undefined} className={`text-xs ${err || shown ? "text-destructive" : "text-muted-foreground"}`}>
      {err ?? (shown ? invalidMsg : hint)}
    </p>
  );

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <header className="sticky top-0 z-20 bg-background/50 backdrop-blur-md">
        <div className="mx-auto flex h-14 w-full max-w-4xl items-center gap-2 px-4">
          <Button variant="ghost" size="sm" onClick={() => nav("/chat")} aria-label={t("common.back")}><ArrowLeft className="h-4 w-4" aria-hidden /></Button>
          <h1 className="text-xl font-extrabold">{t("settings.title")}</h1>
        </div>
      </header>
      <main className="mx-auto flex w-full max-w-4xl flex-1 flex-col justify-start px-4 pb-10 md:justify-center md:pb-14">
        <div className="rounded-3xl border border-border bg-card px-5 shadow-sm md:px-8">
            {!guest && (
              <Card title={t("settings.profile")}>
                <form onSubmit={saveDisplay} className="flex flex-col gap-1.5">
                  <label htmlFor="settings-display-name" className="text-sm font-semibold">{t("settings.displayName")}</label>
                  <div className="flex max-w-md gap-2">
                    <Input id="settings-display-name" value={display} onChange={(e) => setDisplay(e.target.value)} autoComplete="off" maxLength={60}
                      aria-invalid={dnShown || !!errs.display} aria-describedby="settings-display-name-msg" className="rounded-full border border-input bg-transparent focus-visible:ring-0 focus-visible:border-ring" />
                    <Button type="submit" className="rounded-full" disabled={busy || !dn.ok || (dn.ok && dn.value === me.display_name)}>{t("common.save")}</Button>
                  </div>
                  {fieldMsg("settings-display-name-msg", errs.display, dnShown, undefined, errorMessage("display_name_invalid"))}
                </form>
                <form onSubmit={saveUsername} className="flex flex-col gap-1.5">
                  <label htmlFor="settings-username" className="text-sm font-semibold">{t("settings.username")}</label>
                  <div className="flex max-w-md gap-2">
                    <Input id="settings-username" value={username} onChange={(e) => setUser(e.target.value)} autoCapitalize="none" autoComplete="off" maxLength={20}
                      disabled={noLeft} aria-invalid={unShown || !!errs.username} aria-describedby="settings-username-msg" className="rounded-full border border-input bg-transparent focus-visible:ring-0 focus-visible:border-ring" />
                    <Button type="submit" className="rounded-full" disabled={busy || noLeft || !USERNAME_RE.test(uname) || uname === me.username}>{t("common.save")}</Button>
                  </div>
                  {fieldMsg("settings-username-msg", errs.username, unShown, t("settings.changesLeft", { count: me.username_changes_left }), errorMessage("username_invalid"))}
                </form>
              </Card>
            )}

            <Card title={t("settings.language")}>
              <div className="flex gap-2">
                {(["en", "id"] as Lang[]).map((l) => (
                  <Segment key={l} active={i18n.language === l} onClick={() => setLang(l)}>{l.toUpperCase()}</Segment>
                ))}
              </div>
            </Card>

            <Card title={t("settings.theme")}>
              <div className="flex flex-wrap gap-2">
                {THEMES.map((p) => (
                  <Segment key={p} active={theme === p} onClick={() => { setThemePref(p); applyTheme(p); setTheme(p); }}>{t(`theme.${p}`)}</Segment>
                ))}
              </div>
            </Card>

            {guest ? (
              <Card title={t("settings.danger")}>
                <p className="text-sm text-muted-foreground">{t("settings.guestHint")}</p>
                <Button variant="destructive" className="w-fit rounded-full" disabled={busy} onClick={() => void doDelete(false)}>{t("settings.endSession")}</Button>
              </Card>
            ) : (
              <>
                <Card title={t("settings.logout")}>
                  <Button variant="outline" className="w-fit rounded-full" onClick={() => void doLogout()}>{t("settings.logout")}</Button>
                </Card>
                <Card title={t("settings.danger")}>
                  {me.is_developer ? (
                    <p className="text-sm text-muted-foreground">{t("settings.devNote")}</p>
                  ) : (
                    <Button variant="destructive" className="w-fit rounded-full" onClick={() => { setErrs((s) => ({ ...s, del: undefined })); setDelOpen(true); }}>{t("settings.deleteAccount")}</Button>
                  )}
                </Card>
                {!me.is_developer && me.username && (
                  <DeleteAccountDialog open={delOpen} onOpenChange={setDelOpen} username={me.username} busy={busy} error={errs.del} onConfirm={() => void doDelete(true)} />
                )}
              </>
            )}
        </div>
      </main>
    </div>
  );
}
