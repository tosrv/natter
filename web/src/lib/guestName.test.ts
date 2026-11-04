import { describe, it, expect } from "vitest";
import { randomGuestName } from "./guestName";

describe("randomGuestName", () => {
  it("matches natter-xxxx 100 times", () => {
    for (let i = 0; i < 100; i++) expect(randomGuestName()).toMatch(/^natter-[a-z0-9]{4}$/);
  });
});
