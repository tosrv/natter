export function Launcher({ open, label, onClick }: { open: boolean; label: string; onClick: () => void }) {
  return (
    <button class="pm-launch" type="button" aria-label={label} aria-expanded={open} onClick={onClick}>
      <svg viewBox="0 0 24 24" width="26" height="26" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        {open
          ? <path d="M18 6 6 18M6 6l12 12" />
          : <path d="M7.9 20A9 9 0 1 0 4 16.1L2 22Z" />}
      </svg>
    </button>
  );
}
