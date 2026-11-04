import { describe, it, expect, vi } from "vitest";
import { createController } from "./controller";
import { ApiError } from "./api";

function setup(over: Record<string, unknown> = {}, token: string | null = "tok") {
  let tok = token;
  const session = { get: () => tok, set: vi.fn((t: string) => { tok = t; }), clear: vi.fn(() => { tok = null; }) };
  const api = {
    config: vi.fn(async () => ({ turnstile_site_key: "k" })),
    createGuest: vi.fn(async () => ({ token: "new", conversation_id: "c1", id: "u1" })),
    me: vi.fn(async () => ({ id: "u1" })),
    conversations: vi.fn(async () => [{ id: "c1", peer_id: "p1", peer_last_read_seq: 0, peer_ended: false }]),
    messagesAfter: vi.fn(async () => []),
    ...over,
  };
  const sockets: { opts: any; send: any; close: any }[] = [];
  const socketFactory = vi.fn((opts: any) => {
    const s = { opts, send: vi.fn(() => true), close: vi.fn() };
    sockets.push(s);
    return s;
  });
  const c = createController({ api: api as any, session, socketFactory });
  return { c, api, session, sockets, tok: () => tok };
}

describe("controller", () => {
  it("stale token → turnstile", async () => {
    const t = setup({ conversations: vi.fn(async () => { throw new ApiError("unauthorized", 401); }) });
    await t.c.open();
    expect(t.c.state().phase).toBe("turnstile");
    expect(t.session.clear).toHaveBeenCalled();
  });
  it("valid token → chat and socket created", async () => {
    const t = setup();
    await t.c.open();
    expect(t.c.state().phase).toBe("chat");
    expect(t.sockets.length).toBe(1);
  });
  it("no token → turnstile", async () => {
    const t = setup({}, null);
    await t.c.open();
    expect(t.c.state().phase).toBe("turnstile");
    expect(t.c.state().siteKey).toBe("k");
  });
  it("startGuest stores token then chat", async () => {
    const t = setup({}, null);
    await t.c.open();
    await t.c.startGuest("ts");
    expect(t.session.set).toHaveBeenCalledWith("new");
    expect(t.c.state().phase).toBe("chat");
  });
  it("conversation_ended → ended and token cleared", async () => {
    const t = setup();
    await t.c.open();
    t.sockets[0].opts.onFrame({ type: "conversation_ended", conversation_id: "c1" });
    expect(t.c.state().phase).toBe("ended");
    expect(t.tok()).toBeNull();
    expect(t.sockets[0].close).toHaveBeenCalled();
  });
  it("error peer_ended → ended", async () => {
    const t = setup();
    await t.c.open();
    t.sockets[0].opts.onFrame({ type: "error", code: "peer_ended" });
    expect(t.c.state().phase).toBe("ended");
  });
  it("limit_reached → full", async () => {
    const t = setup({ createGuest: vi.fn(async () => { throw new ApiError("limit_reached", 429); }) }, null);
    await t.c.open();
    await t.c.startGuest("ts");
    expect(t.c.state().phase).toBe("full");
  });
  it("fetch TypeError → unavailable", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const t = setup({ conversations: vi.fn(async () => { throw new TypeError("Failed to fetch"); }) });
    await t.c.open();
    expect(t.c.state().phase).toBe("unavailable");
    expect(warn).toHaveBeenCalled();
  });
  it("restart from ended → turnstile", async () => {
    const t = setup();
    await t.c.open();
    t.sockets[0].opts.onFrame({ type: "conversation_ended" });
    t.c.restart();
    await new Promise((r) => setTimeout(r, 0));
    expect(t.c.state().phase).toBe("turnstile");
  });
  it("send queues and sends frame; on open resends outbox after catch-up", async () => {
    const t = setup();
    await t.c.open();
    t.c.send("hi");
    expect(t.c.state().chat.outbox.length).toBe(1);
    expect(t.sockets[0].send).toHaveBeenCalledWith(expect.objectContaining({ type: "send", conversation_id: "c1", body: "hi" }));
    t.sockets[0].opts.onOpen();
    await new Promise((r) => setTimeout(r, 0));
    expect(t.sockets[0].send).toHaveBeenCalledTimes(2);
  });
  it("catch-up 401 → ended", async () => {
    const t = setup();
    await t.c.open();
    t.api.messagesAfter.mockImplementation(async () => { throw new ApiError("unauthorized", 401); });
    t.sockets[0].opts.onOpen();
    await new Promise((r) => setTimeout(r, 0));
    expect(t.c.state().phase).toBe("ended");
  });
  it("turnstile load failure (fail) → unavailable, reopen retries", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    const t = setup({}, null);
    await t.c.open();
    t.c.fail(new Error("turnstile load failed"));
    expect(t.c.state().phase).toBe("unavailable");
    await t.c.open();
    expect(t.c.state().phase).toBe("turnstile");
  });
  it("own id from /me; peer message with client_msg_id is not mine and gets read", async () => {
    const t = setup();
    await t.c.open();
    expect(t.c.state().meId).toBe("u1");
    t.sockets[0].opts.onFrame({ type: "message", message: { id: "m1", conversation_id: "c1", sender_id: "p1", seq: 1, body: "x", client_msg_id: "zz", created_at: "" } });
    const m = t.c.state().chat.messages[0];
    expect(m.sender_id === t.c.state().meId).toBe(false);
    expect(t.sockets[0].send).toHaveBeenCalledWith({ type: "read", conversation_id: "c1", seq: 1 });
  });
  it("startGuest uses createGuest id as meId", async () => {
    const t = setup({ createGuest: vi.fn(async () => ({ token: "n", conversation_id: "c1", id: "g9" })) }, null);
    await t.c.open();
    await t.c.startGuest("ts");
    expect(t.c.state().meId).toBe("g9");
  });
  it("non-rate-limited error with client_msg_id drops outbox item", async () => {
    const t = setup();
    await t.c.open();
    t.c.send("hi");
    const id = t.c.state().chat.outbox[0].client_msg_id;
    t.sockets[0].opts.onFrame({ type: "error", code: "bad_request", client_msg_id: id });
    expect(t.c.state().chat.outbox.length).toBe(0);
  });
  it("rate_limited keeps outbox item and retries", async () => {
    vi.useFakeTimers();
    const t = setup();
    await t.c.open();
    t.c.send("hi");
    const id = t.c.state().chat.outbox[0].client_msg_id;
    t.sockets[0].opts.onFrame({ type: "error", code: "rate_limited", client_msg_id: id });
    expect(t.c.state().chat.outbox.length).toBe(1);
    vi.advanceTimersByTime(10000);
    expect(t.sockets[0].send).toHaveBeenCalledTimes(2);
    vi.useRealTimers();
  });
});
