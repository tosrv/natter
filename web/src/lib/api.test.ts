import { describe, it, expect, vi, afterEach } from "vitest";
import { request, ApiError } from "./api";

afterEach(() => vi.unstubAllGlobals());

describe("request", () => {
  it("throws ApiError with code from error body", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: { code: "user_limit_reached", message: "x" } }), { status: 403 })));
    const err = await request("POST", "/auth/guest").catch((e) => e as ApiError);
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
