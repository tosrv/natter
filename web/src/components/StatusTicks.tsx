import { useTranslation } from "react-i18next";

export type TickStatus = "sending" | "sent" | "read";

export default function StatusTicks({ status }: { status: TickStatus }) {
  const { t } = useTranslation();
  const label = t(`status.${status}`);
  return (
    <svg role="img" aria-label={label} viewBox="0 0 18 11" className="inline-block h-[11px] w-[18px]">
      <title>{label}</title>
      {status === "sending" && (
        <>
          <circle cx="9" cy="5.5" r="4" fill="none" stroke="currentColor" strokeOpacity=".8" strokeWidth="1.5" />
          <path d="M9 3.5v2.2l1.4 1" fill="none" stroke="currentColor" strokeOpacity=".8" strokeWidth="1.3" strokeLinecap="round" />
        </>
      )}
      {status === "sent" && (
        <path d="M4 6l3 3 6-7" fill="none" stroke="currentColor" strokeOpacity=".8" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
      )}
      {status === "read" && (
        <>
          <path d="M1 6l3 3 6-7" fill="none" stroke="#bfe9ff" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
          <path d="M6.5 9l1 0 6-7" fill="none" stroke="#bfe9ff" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
        </>
      )}
    </svg>
  );
}
