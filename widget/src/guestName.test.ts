import { it, expect } from "vitest";
import { randomGuestName } from "./guestName";

it("randomGuestName matches pattern", () => {
  for (let i = 0; i < 20; i++) expect(randomGuestName()).toMatch(/^natter-[a-z0-9]{4}$/);
});
