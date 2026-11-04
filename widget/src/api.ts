import type { Conversation, Message } from "./types";

export class ApiError extends Error {
  status: number;
  code: string;
  constructor(code: string, status: number, message?: string) {
    super(message ?? code);
    this.code = code;
    this.status = status;
  }
}

export function createApi(server: string, getToken: () => string | null) {
  async function req<T>(method: string, path: string, body?: unknown): Promise<T> {
    const headers: Record<string, string> = {};
    const tok = getToken();
    if (tok) headers.Authorization = `Bearer ${tok}`;
    if (body !== undefined) headers["Content-Type"] = "application/json";
    let res: Response;
    try {
      res = await fetch(`${server}/api${path}`, {
        method, credentials: "omit", headers,
        body: body !== undefined ? JSON.stringify(body) : undefined,
      });
    } catch {
      throw new ApiError("internal", 0);
    }
    const text = await res.text();
    let data: unknown;
    try { data = text ? JSON.parse(text) : undefined; } catch { /* non-JSON */ }
    if (!res.ok) {
      const e = (data as { error?: { code?: string; message?: string } } | undefined)?.error;
      throw new ApiError(e?.code ?? "internal", res.status, e?.message);
    }
    return data as T;
  }
  return {
    config: () => req<{ turnstile_site_key: string }>("GET", "/config"),
    createGuest: (turnstileToken: string) =>
      req<{ token: string; conversation_id: string; id: string }>("POST", "/auth/guest", { turnstile_token: turnstileToken }),
    conversations: () => req<Conversation[]>("GET", "/conversations"),
    messagesAfter: async (convId: string, afterSeq: number, limit?: number): Promise<Message[]> => {
      const qs = new URLSearchParams({ after_seq: String(afterSeq) });
      if (limit !== undefined) qs.set("limit", String(limit));
      const r = await req<{ messages: Message[] }>("GET", `/conversations/${encodeURIComponent(convId)}/messages?${qs}`);
      return r.messages ?? [];
    },
  };
}
