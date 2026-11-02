import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { MAX_BODY, runeLength } from "@/components/Composer";

export default function GuestContactDialog({ open, onClose, onSubmit }: {
  open: boolean; onClose: () => void; onSubmit: (answer: string) => void;
}) {
  const { t } = useTranslation();
  const [v, setV] = useState("");
  const ok = v.trim() !== "" && runeLength(v) <= MAX_BODY;
  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("chat.contactTitle")}</DialogTitle>
          <DialogDescription>{t("chat.contactHint")}</DialogDescription>
        </DialogHeader>
        <textarea value={v} onChange={(e) => setV(e.target.value)} rows={3} placeholder={t("chat.contactPlaceholder")}
          aria-label={t("chat.contactTitle")} className="w-full resize-none rounded-md border bg-background p-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring" />
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>{t("chat.skip")}</Button>
          <Button disabled={!ok} onClick={() => { onSubmit(v); onClose(); }}>{t("chat.send")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
