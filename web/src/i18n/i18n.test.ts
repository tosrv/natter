import { describe, it, expect } from "vitest";
import en from "./en.json";
import id from "./id.json";

const ERROR_CODES = [
  "user_limit_reached", "developer_unavailable", "not_found", "forbidden", "message_empty",
  "message_too_long", "username_invalid", "username_taken", "username_change_limit",
  "username_required", "query_too_short", "rate_limited", "unauthorized",
  "invalid_state", "bad_request", "internal", "oauth_failed",
];

function keys(o: Record<string, unknown>, p = ""): string[] {
  return Object.entries(o).flatMap(([k, v]) =>
    v && typeof v === "object" ? keys(v as Record<string, unknown>, p + k + ".") : [p + k]);
}

describe("i18n", () => {
  it("en and id have identical key sets", () => {
    expect(keys(id).sort()).toEqual(keys(en).sort());
  });
  it("both languages contain every error code", () => {
    for (const lang of [en, id]) {
      const errs = (lang as { errors: Record<string, string> }).errors;
      for (const c of ERROR_CODES) expect(errs[c], c).toBeTruthy();
    }
  });
});
