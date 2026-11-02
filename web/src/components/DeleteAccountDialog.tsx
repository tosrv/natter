import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";

interface Props {
  open: boolean;
  onOpenChange: (o: boolean) => void;
  username: string;
  busy: boolean;
  error?: string;
  onConfirm: () => void;
}

export default function DeleteAccountDialog({ open, onOpenChange, username, busy, error, onConfirm }: Props) {
  const { t } = useTranslation();
  const [typed, setTyped] = useState("");
  useEffect(() => { if (open) setTyped(""); }, [open]);
  const close = (o: boolean) => { if (!o) setTyped(""); onOpenChange(o); };
  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("settings.deleteTitle")}</DialogTitle>
          <DialogDescription>{t("settings.deleteBody")}</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="delete-confirm" className="text-sm font-semibold">{t("settings.deleteConfirm", { username })}</label>
          <Input id="delete-confirm" value={typed} onChange={(e) => setTyped(e.target.value)} autoComplete="off" autoCapitalize="none"
            aria-describedby="delete-confirm-msg" className="rounded-full" />
          <p id="delete-confirm-msg" role={error ? "alert" : undefined} className="min-h-4 text-xs text-destructive">{error}</p>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={() => close(false)}>{t("settings.cancel")}</Button>
          <Button variant="destructive" disabled={busy || typed !== username} onClick={onConfirm}>{t("settings.deleteAction")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
