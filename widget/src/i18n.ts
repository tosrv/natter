export type Lang = "en" | "id";

const dict = {
  en: {
    title: "Chat",
    placeholder: "Type a message...",
    send: "Send",
    open: "Open chat",
    close: "Close chat",
    connecting: "Connecting...",
    offline: "Offline. Reconnecting...",
    error: "Something went wrong.",
    retry: "Retry",
    ended: "This conversation has ended.",
    seen: "Seen",
  },
  id: {
    title: "Obrolan",
    placeholder: "Ketik pesan...",
    send: "Kirim",
    open: "Buka obrolan",
    close: "Tutup obrolan",
    connecting: "Menghubungkan...",
    offline: "Offline. Menyambung kembali...",
    error: "Terjadi kesalahan.",
    retry: "Coba lagi",
    ended: "Percakapan ini telah berakhir.",
    seen: "Dilihat",
  },
} as const;

export type Key = keyof typeof dict.en;

export function t(lang: Lang, key: Key): string {
  return dict[lang][key];
}

export function pickLang(attr?: string | null): Lang {
  const a = (attr ?? "").toLowerCase();
  if (a === "en" || a === "id") return a;
  const nav = typeof navigator !== "undefined" ? (navigator.language ?? "").toLowerCase() : "";
  return nav.startsWith("id") ? "id" : "en";
}
