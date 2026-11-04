import { describe, it, expect } from "vitest";
import { createSession } from "./session";

describe("session", () => {
  it("set/get/clear uses namespaced key", () => {
    const st = window.localStorage; st.clear();
    const s = createSession("https://chat.example", st);
    expect(s.get()).toBeNull();
    s.set("tok");
    expect(st.getItem("natter:https://chat.example")).toBe("tok");
    expect(s.get()).toBe("tok");
    s.clear();
    expect(s.get()).toBeNull();
    expect(st.getItem("natter:https://chat.example")).toBeNull();
  });
  it("falls back to memory when storage throws", () => {
    const bad = { getItem() { throw new Error("x"); }, setItem() { throw new Error("x"); }, removeItem() { throw new Error("x"); } } as unknown as Storage;
    const s = createSession("https://chat.example", bad);
    s.set("tok");
    expect(s.get()).toBe("tok");
    s.clear();
    expect(s.get()).toBeNull();
  });
});
