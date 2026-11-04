import { describe, it, expect, vi, afterEach } from "vitest";
import { createApi, ApiError } from "./api";

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status });
afterEach(() => vi.unstubAllGlobals());

describe("api", () => {
  it("createGuest posts turnstile_token with credentials omit", async () => {
    const f = vi.fn().mockResolvedValue(json(200, { token: "t", conversation_id: "c", id: "u" }));
    vi.stubGlobal("fetch", f);
    const r = await createApi("https://s.example", () => null).createGuest("ts1");
    expect(r.token).toBe("t");
    const [url, init] = f.mock.calls[0];
    expect(url).toBe("https://s.example/api/auth/guest");
    expect(init.method).toBe("POST");
    expect(init.credentials).toBe("omit");
    expect(JSON.parse(init.body)).toEqual({ turnstile_token: "ts1" });
  });
  it("conversations sends bearer token", async () => {
    const f = vi.fn().mockResolvedValue(json(200, []));
    vi.stubGlobal("fetch", f);
    await createApi("https://s.example", () => "abc").conversations();
    expect(f.mock.calls[0][1].headers.Authorization).toBe("Bearer abc");
    expect(f.mock.calls[0][1].credentials).toBe("omit");
  });
  it("messagesAfter unwraps messages and builds query", async () => {
    const f = vi.fn().mockResolvedValue(json(200, { messages: [{ seq: 4 }] }));
    vi.stubGlobal("fetch", f);
    const r = await createApi("https://s.example", () => "abc").messagesAfter("c1", 3, 50);
    expect(r).toEqual([{ seq: 4 }]);
    expect(f.mock.calls[0][0]).toBe("https://s.example/api/conversations/c1/messages?after_seq=3&limit=50");
  });
  it("401 rejects with ApiError", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json(401, { error: { code: "unauthorized" } })));
    const p = createApi("https://s.example", () => null).conversations();
    await expect(p).rejects.toBeInstanceOf(ApiError);
    await expect(p).rejects.toMatchObject({ status: 401, code: "unauthorized" });
  });
});
