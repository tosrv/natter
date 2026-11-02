import { avatarColor } from "@/lib/format";
import { cn } from "@/lib/utils";

export default function Avatar({ id, name, className }: { id: string; name: string; className?: string }) {
  const initial = ([...name.trim()][0] ?? "?").toUpperCase();
  return (
    <span aria-hidden="true" style={{ backgroundColor: avatarColor(id) }}
      className={cn("inline-flex size-10 shrink-0 select-none items-center justify-center rounded-full font-bold text-white", className)}>
      {initial}
    </span>
  );
}
