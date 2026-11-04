import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import TurnstileWidget from "@/components/TurnstileWidget";
import { MAX_BODY, runeLength } from "@/components/Composer";
import { ApiError, createGuest } from "@/lib/api";
import { randomGuestName } from "@/lib/guestName";
import { errorMessage } from "@/i18n";

const MAX_NAME = 40;

export default function GuestStartDialog({ open, onClose, siteKey }: {
  open: boolean; onClose: () => void; siteKey: string;
}) {
  const { t } = useTranslation();
  const nav = useNavigate();
  const [name, setName] = useState("");
  const [contact, setContact] = useState("");
  const [token, setToken] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [nameError, setNameError] = useState(false);
  const resetWidget = useRef<(() => void) | null>(null);
  const needsToken = !!siteKey;

  // Fresh random name (and clean form) every time the dialog opens.
  useEffect(() => {
    if (!open) return;
    setName(randomGuestName());
    setContact("");
    setToken(null);
    setBusy(false);
    setNameError(false);
  }, [open]);

  const nameLen = runeLength(name.trim());
  const ok = nameLen >= 1 && nameLen <= MAX_NAME && runeLength(contact) <= MAX_BODY && (!needsToken || !!token) && !busy;

  async function submit() {
    if (!ok) return;
    setBusy(true);
    setNameError(false);
    try {
      await createGuest(name.trim(), needsToken ? token ?? undefined : undefined);
      const c = contact.trim();
      nav("/chat", { state: c ? { guestContact: c } : undefined });
    } catch (e) {
      const code = e instanceof ApiError ? e.code : "internal";
      if (code === "display_name_invalid") setNameError(true);
      else toast.error(errorMessage(code));
      // A Turnstile token is single-use: always get a fresh one after a failure.
      resetWidget.current?.();
      setToken(null);
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("landing.guest")}</DialogTitle>
        </DialogHeader>
        <form className="flex flex-col gap-3" onSubmit={(e) => { e.preventDefault(); void submit(); }}>
          <label className="flex flex-col gap-1 text-sm">
            {t("guestStart.name")}
            <Input value={name} onChange={(e) => setName(e.target.value)} aria-invalid={nameError || undefined} autoComplete="off" />
            {nameError && <span role="alert" className="text-xs text-destructive">{t("guestStart.nameError")}</span>}
          </label>
          <label className="flex flex-col gap-1 text-sm">
            {t("guestStart.contact")}
            <textarea value={contact} onChange={(e) => setContact(e.target.value)} rows={2}
              className="w-full resize-none rounded-md border bg-background p-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
          </label>
          {needsToken && (
            <div className="flex justify-center">
              <TurnstileWidget siteKey={siteKey} onToken={setToken} onReady={(r) => { resetWidget.current = r; }} />
            </div>
          )}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>{t("guestStart.cancel")}</Button>
            <Button type="submit" disabled={!ok}>{t("guestStart.start")}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
