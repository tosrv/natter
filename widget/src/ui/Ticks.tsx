export type TickStatus = "sending" | "sent" | "read";

const line = { fill: "none", "stroke-width": 1.8, "stroke-linecap": "round", "stroke-linejoin": "round" } as const;

// WhatsApp-style status: clock while sending, one tick when sent, two blue ticks when read.
export function Ticks({ status, label }: { status: TickStatus; label: string }) {
  return (
    <svg class="pm-tick" role="img" aria-label={label} viewBox="0 0 18 11" width="18" height="11">
      <title>{label}</title>
      {status === "sending" && (
        <>
          <circle cx="9" cy="5.5" r="4" fill="none" stroke="currentColor" stroke-width="1.5" />
          <path d="M9 3.5v2.2l1.4 1" {...line} stroke="currentColor" stroke-width="1.3" />
        </>
      )}
      {status === "sent" && <path d="M4 6l3 3 6-7" {...line} stroke="currentColor" />}
      {status === "read" && (
        <>
          <path d="M1 6l3 3 6-7" {...line} style={{ stroke: "var(--pm-read)" }} />
          <path d="M6.5 9l1 0 6-7" {...line} style={{ stroke: "var(--pm-read)" }} />
        </>
      )}
    </svg>
  );
}
