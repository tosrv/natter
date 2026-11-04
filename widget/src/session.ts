export function createSession(server: string, storage?: Storage) {
  const key = `natter:${server}`;
  let mem: string | null = null;
  let st: Storage | undefined = storage;
  if (!st) {
    try { st = window.localStorage; } catch { st = undefined; }
  }
  return {
    get(): string | null {
      try { if (st) return st.getItem(key) ?? mem; } catch { /* fall through */ }
      return mem;
    },
    set(t: string): void {
      mem = t;
      try { st?.setItem(key, t); } catch { /* memory only */ }
    },
    clear(): void {
      mem = null;
      try { st?.removeItem(key); } catch { /* ignore */ }
    },
  };
}
