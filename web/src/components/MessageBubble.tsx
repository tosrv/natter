import StatusTicks, { type TickStatus } from "@/components/StatusTicks";
import { cn } from "@/lib/utils";

const hhmm = (iso: string | undefined) => {
  if (!iso) return "";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
};

export default function MessageBubble({ mine, body, createdAt, status }: {
  mine: boolean; body: string; createdAt?: string; status: TickStatus | null;
}) {
  return (
    <div className={cn("flex", mine ? "justify-end" : "justify-start")}>
      <div className={cn("max-w-[80%] rounded-2xl px-3 py-2 shadow-sm",
        mine ? "rounded-br-sm bg-primary-solid text-primary-foreground" : "rounded-bl-sm bg-card text-card-foreground border")}>
        {/* Rendered as plain text; never HTML. */}
        <p className="whitespace-pre-wrap break-words">{body}</p>
        <p className={cn("mt-0.5 flex items-center justify-end gap-1 text-[11px]", mine ? "" : "text-muted-foreground")}>
          <span>{hhmm(createdAt)}</span>
          {status && <StatusTicks status={status} />}
        </p>
      </div>
    </div>
  );
}
