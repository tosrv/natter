import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Ban, MoreVertical, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";

type Action = () => Promise<void> | void;

export default function BlockMenu({ name, blocked, onBlock, onUnblock, onDelete }: {
  name: string; blocked: boolean; onBlock?: Action; onUnblock?: Action; onDelete?: Action;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState<"block" | "delete" | null>(null);
  const [busy, setBusy] = useState(false);
  const canBlock = !!(onBlock && onUnblock);
  async function confirm(fn?: Action) {
    setBusy(true);
    try { await fn?.(); } finally { setBusy(false); setOpen(null); }
  }
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon" className="ml-auto shrink-0" aria-label={t("chat.menu")}><MoreVertical /></Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          {canBlock && (blocked
            ? <DropdownMenuItem onSelect={() => void onUnblock?.()}>{t("chat.unblock")}</DropdownMenuItem>
            : <DropdownMenuItem onSelect={() => setOpen("block")}><Ban className="mr-2 size-4" aria-hidden />{t("chat.block")}</DropdownMenuItem>)}
          {onDelete && <DropdownMenuItem onSelect={() => setOpen("delete")}><Trash2 className="mr-2 size-4" aria-hidden />{t("chat.deleteChat")}</DropdownMenuItem>}
        </DropdownMenuContent>
      </DropdownMenu>
      <Dialog open={open !== null} onOpenChange={(o) => { if (!o) setOpen(null); }}>
        <DialogContent>
          {open === "delete" ? (
            <>
              <DialogHeader>
                <DialogTitle>{t("chat.deleteTitle")}</DialogTitle>
                <DialogDescription>{t("chat.deleteBody")}</DialogDescription>
              </DialogHeader>
              <DialogFooter>
                <Button variant="ghost" onClick={() => setOpen(null)}>{t("settings.cancel")}</Button>
                <Button variant="destructive" disabled={busy} onClick={() => void confirm(onDelete)}>{t("chat.deleteConfirm")}</Button>
              </DialogFooter>
            </>
          ) : (
            <>
              <DialogHeader>
                <DialogTitle>{t("chat.blockTitle", { name })}</DialogTitle>
                <DialogDescription>{t("chat.blockBody")}</DialogDescription>
              </DialogHeader>
              <DialogFooter>
                <Button variant="ghost" onClick={() => setOpen(null)}>{t("settings.cancel")}</Button>
                <Button variant="destructive" disabled={busy} onClick={() => void confirm(onBlock)}>{t("chat.block")}</Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
