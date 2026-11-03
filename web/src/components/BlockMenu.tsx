import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Ban, MoreVertical } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";

export default function BlockMenu({ name, blocked, onBlock, onUnblock }: {
  name: string; blocked: boolean; onBlock: () => Promise<void> | void; onUnblock: () => Promise<void> | void;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  async function confirm() {
    setBusy(true);
    try { await onBlock(); } finally { setBusy(false); setOpen(false); }
  }
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon" className="ml-auto shrink-0" aria-label={t("chat.menu")}><MoreVertical /></Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          {blocked
            ? <DropdownMenuItem onSelect={() => void onUnblock()}>{t("chat.unblock")}</DropdownMenuItem>
            : <DropdownMenuItem onSelect={() => setOpen(true)}><Ban className="mr-2 size-4" aria-hidden />{t("chat.block")}</DropdownMenuItem>}
        </DropdownMenuContent>
      </DropdownMenu>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("chat.blockTitle", { name })}</DialogTitle>
            <DialogDescription>{t("chat.blockBody")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setOpen(false)}>{t("settings.cancel")}</Button>
            <Button variant="destructive" disabled={busy} onClick={() => void confirm()}>{t("chat.block")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
