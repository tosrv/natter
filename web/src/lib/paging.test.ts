import { describe, it, expect, vi } from "vitest";
import { nextFetch, fetchMissing } from "./paging";
import type { Message } from "./api";

const m = (seq: number): Message => ({ id: "m" + seq, conversation_id: "c", sender_id: "u", seq, body: "", created_at: "" });
const page = (from: number, n: number) => Array.from({ length: n }, (_, i) => m(from + i));

describe("nextFetch", () => {
  it("first open loads latest page even if live messages already seeded", () => {
    expect(nextFetch({ historyLoaded: false, messages: [m(500)] })).toEqual({});
  });
  it("after history loaded, fetches after last seq", () => {
    expect(nextFetch({ historyLoaded: true, messages: [m(1), m(7)] })).toEqual({ after_seq: 7 });
    expect(nextFetch({ historyLoaded: true, messages: [] })).toEqual({ after_seq: 0 });
  });
});

describe("fetchMissing", () => {
  it("loops while full pages are returned (catch-up >100)", async () => {
    const get = vi.fn()
      .mockResolvedValueOnce(page(11, 100))
      .mockResolvedValueOnce(page(111, 100))
      .mockResolvedValueOnce(page(211, 5));
    const out = await fetchMissing(get, { historyLoaded: true, messages: [m(10)] }, 100);
    expect(out.length).toBe(205);
    expect(get.mock.calls.map((c) => c[0])).toEqual([{ after_seq: 10 }, { after_seq: 110 }, { after_seq: 210 }]);
  });
  it("first open makes a single latest-page request", async () => {
    const get = vi.fn().mockResolvedValueOnce(page(401, 100));
    const out = await fetchMissing(get, { historyLoaded: false, messages: [m(500)] }, 100);
    expect(out.length).toBe(100);
    expect(get).toHaveBeenCalledTimes(1);
    expect(get.mock.calls[0][0]).toEqual({});
  });
});
