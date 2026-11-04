import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { ApiError, search, type UserHit } from "@/lib/api";
import { errorMessage } from "@/i18n";
import Avatar from "@/components/Avatar";

export default function UserSearch({ onPick, onUnauthorized }: {
  onPick: (u: UserHit) => void; onUnauthorized: () => void;
}) {
  const { t } = useTranslation();
  const [q, setQ] = useState("");
  const [hits, setHits] = useState<UserHit[] | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    const term = q.trim().toLowerCase();
    if (term.length < 2) { setHits(null); setErr(null); return; }
    let alive = true;
    const h = setTimeout(() => {
      search(term).then((r) => { if (alive) { setHits(r); setErr(null); } })
        .catch((e) => {
          if (e instanceof ApiError && e.status === 401) return onUnauthorized();
          if (alive) setErr(e instanceof ApiError ? e.code : "internal");
        });
    }, 300);
    return () => { alive = false; clearTimeout(h); };
  }, [q, onUnauthorized]);

  return (
    <div className="space-y-1 p-3">
      <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder={t("chat.searchPlaceholder")} aria-label={t("chat.searchPlaceholder")} className="border border-input bg-transparent focus-visible:ring-0 focus-visible:border-ring" />
      {err && <p className="text-sm text-destructive">{errorMessage(err)}</p>}
      {hits && hits.length === 0 && <p className="text-sm text-muted-foreground">{t("chat.noResults")}</p>}
      {hits && hits.length > 0 && (
        <ul className="rounded-md border bg-card">
          {hits.map((u) => (
            <li key={u.id}>
              <button type="button" className="flex w-full items-center gap-2 p-2 text-left text-sm hover:bg-muted"
                onClick={() => { setQ(""); setHits(null); onPick(u); }}>
                <Avatar id={u.id} name={u.display_name || u.username} className="size-7 text-xs" />
                <span className="min-w-0">
                  <span className="block truncate font-medium">{u.display_name || u.username}</span>
                  <span className="block truncate text-xs text-muted-foreground">@{u.username}</span>
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
