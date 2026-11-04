import { describe, it, expect } from "vitest";
import i18n, { setLang } from "./index";

describe("document lang sync", () => {
  it("follows language changes", async () => {
    setLang("id");
    expect(document.documentElement.lang).toBe("id");
    await i18n.changeLanguage("en");
    expect(document.documentElement.lang).toBe("en");
  });
});
