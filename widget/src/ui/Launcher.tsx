export function Launcher({ open, label, onClick }: { open: boolean; label: string; onClick: () => void }) {
  return (
    <button class="pm-launch" type="button" aria-label={label} aria-expanded={open} onClick={onClick}>
      <svg viewBox="0 0 24 24" width="26" height="26" aria-hidden="true" fill="currentColor">
        {open
          ? <path d="M18.3 5.7 12 12l6.3 6.3-1.4 1.4L10.6 13.4 4.3 19.7 2.9 18.3 9.2 12 2.9 5.7 4.3 4.3l6.3 6.3 6.3-6.3z" />
          : <path d="M4 4h16a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H8l-4 4V6a2 2 0 0 1 2-2z" />}
      </svg>
    </button>
  );
}
