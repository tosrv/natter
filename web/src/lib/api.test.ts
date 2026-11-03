import { describe, it, expect, vi, afterEach } from "vitest";
import { request, ApiError, blockUser, unblockUser, listBlocks } from "./api";

afterEach(() => vi.unstubAllGlobals());

describe("request", () => {
  it("throws ApiError with code from error body", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: { code: "user_limit_reached", message: "x" } }), { status: 403 })));
    const err: ApiError = await request("POST", "/auth/guest").then(() => { throw new Error("no throw"); }, (e: ApiError) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe("user_limit_reached");
    expect(err.status).toBe(403);
  });
  it("sends credentials include", async () => {
    const f = vi.fn().mockResolvedValue(new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", f);
    await request("GET", "/me");
    expect(f.mock.calls[0][1].credentials).toBe("include");
  });
});

describe("blocks", () => {
  it("blockUser POSTs /api/blocks with user_id", async () => {
    const f = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", f);
    await blockUser("u1");
    expect(f.mock.calls[0][0]).toBe("/api/blocks");
    expect(f.mock.calls[0][1].method).toBe("POST");
    expect(JSON.parse(f.mock.calls[0][1].body)).toEqual({ user_id: "u1" });
  });
  it("unblockUser DELETEs /api/blocks/:id", async () => {
    const f = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", f);
    await unblockUser("u 1");
    expect(f.mock.calls[0][0]).toBe("/api/blocks/u%201");
    expect(f.mock.calls[0][1].method).toBe("DELETE");
  });
  it("listBlocks GETs /api/blocks", async () => {
    const f = vi.fn().mockResolvedValue(new Response("[]", { status: 200 }));
    vi.stubGlobal("fetch", f);
    expect(await listBlocks()).toEqual([]);
    expect(f.mock.calls[0][0]).toBe("/api/blocks");
  });
});
