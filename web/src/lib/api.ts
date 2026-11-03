export class ApiError extends Error {
  code: string;
  status: number;
  constructor(code: string, status: number, message?: string) {
    super(message ?? code);
    this.code = code;
    this.status = status;
  }
}

export interface Me {
  id: string;
  kind: "google" | "guest";
  username: string | null;
  display_name: string;
  is_developer: boolean;
  username_changes_left: number;
  email?: string;
}
export interface UserBrief { id: string; username: string; display_name: string }
export type UserHit = UserBrief
export interface Message {
  id: string; conversation_id: string; sender_id: string; seq: number;
  body: string; created_at: string; client_msg_id?: string;
}
export interface ConversationSummary {
  id: string; peer_id: string; peer_username: string; peer_display_name: string; peer_kind: string;
  last_message: Message | null; unread: number; peer_last_read_seq: number; last_activity_at: string;
  blocked_by_me: boolean;
}
export interface BlockedUser { id: string; username: string; display_name: string; blocked_at: string }
export interface Guest { id: string; username: string; conversation_id: string }

export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch("/api" + path, {
      method,
      credentials: "include",
      headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
  } catch {
    throw new ApiError("internal", 0);
  }
  const text = await res.text();
  let data: unknown = undefined;
  try { data = text ? JSON.parse(text) : undefined; } catch { /* non-JSON body */ }
  if (!res.ok) {
    const e = (data as { error?: { code?: string; message?: string } } | undefined)?.error;
    throw new ApiError(e?.code ?? "internal", res.status, e?.message);
  }
  return data as T;
}

export const getMe = () => request<Me>("GET", "/me");
export const setUsername = (username: string) => request<Me>("PUT", "/me/username", { username });
export const search = (q: string) => request<UserBrief[]>("GET", `/users/search?q=${encodeURIComponent(q)}`);
export const openConversation = (peer_id: string) => request<{ id: string }>("POST", "/conversations", { peer_id });
export const listConversations = () => request<ConversationSummary[]>("GET", "/conversations");
export const listMessages = (id: string, p: { after_seq?: number; before_seq?: number; limit?: number } = {}) => {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(p)) if (v !== undefined) qs.set(k, String(v));
  const s = qs.toString();
  return request<{ messages: Message[] }>("GET", `/conversations/${id}/messages${s ? "?" + s : ""}`);
};
export const createGuest = () => request<Guest>("POST", "/auth/guest");
export const logout = () => request<void>("POST", "/auth/logout");
export const setDisplayName = (display_name: string) => request<Me>("PUT", "/me/display-name", { display_name });
export const deleteAccount = () => request<void>("DELETE", "/me");
export const blockUser = (user_id: string) => request<void>("POST", "/blocks", { user_id });
export const unblockUser = (user_id: string) => request<void>("DELETE", `/blocks/${encodeURIComponent(user_id)}`);
export const listBlocks = () => request<BlockedUser[]>("GET", "/blocks");
