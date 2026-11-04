import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { ApiError, listBlocks, unblockUser, type BlockedUser } from "@/lib/api";
import { errorMessage } from "@/i18n";

export default function BlockedList() {
  const { t } = useTranslation();
  const nav = useNavigate();
  const [items, setItems] = useState<BlockedUser[] | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const fail = (e: unknown) => {
    if (e instanceof ApiError && e.status === 401) { nav("/", { replace: true }); return; }
    toast.error(errorMessage(e instanceof ApiError ? e.code : "internal"));
  };
  useEffect(() => { listBlocks().then(setItems).catch((e) => { setItems([]); fail(e); });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function unblock(id: string) {
    setBusyId(id);
    try { await unblockUser(id); setItems((xs) => (xs ?? []).filter((x) => x.id !== id)); }
    catch (e) { fail(e); }
    setBusyId(null);
  }

  if (items === null) return <p className="text-sm text-muted-foreground">{t("common.loading")}</p>;
  if (items.length === 0) return <p className="text-sm text-muted-foreground">{t("settings.blockedEmpty")}</p>;
  return (
    <ul className="flex flex-col gap-2">
      {items.map((u) => (
        <li key={u.id} className="flex items-center justify-between gap-2">
          <span className="min-w-0 truncate"><span className="font-semibold">{u.display_name || u.username}</span> <span className="text-xs text-muted-foreground">@{u.username}</span></span>
          <Button size="sm" variant="outline" className="shrink-0" disabled={busyId === u.id} onClick={() => void unblock(u.id)}>{t("chat.unblock")}</Button>
        </li>
      ))}
    </ul>
  );
}
