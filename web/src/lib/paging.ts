import type { Message } from "./api";

export interface ConvHistory { historyLoaded: boolean; messages: Message[] }
export type FetchParams = { after_seq?: number };

/**
 * First open: no params → API returns the newest page (ascending), so live
 * messages seeded before opening do not hide earlier history.
 * Afterwards: catch up from the last known seq.
 */
export function nextFetch(s: ConvHistory): FetchParams {
  if (!s.historyLoaded) return {};
  const last = s.messages.length ? s.messages[s.messages.length - 1].seq : 0;
  return { after_seq: last };
}

/** Fetch what's missing; when catching up, keep paging while full pages come back. */
export async function fetchMissing(
  get: (p: FetchParams) => Promise<Message[]>, s: ConvHistory, limit: number,
): Promise<Message[]> {
  let p = nextFetch(s);
  const first = await get(p);
  if (p.after_seq === undefined) return first;
  const out = [...first];
  let batch = first;
  while (batch.length >= limit) {
    p = { after_seq: batch[batch.length - 1].seq };
    batch = await get(p);
    out.push(...batch);
  }
  return out;
}
