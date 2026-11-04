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
    sent: "Sent",
    sending: "Sending",
    newChat: "Start a new chat",
    full: "We are busy right now, please try again later.",
    unavailable: "Chat is unavailable.",
    typing: "typing...",
    guestName: "Your name",
    guestNameError: "Use 1–40 letters, numbers, spaces or . - ' _",
    guestContact: "Contact (optional) — email, WhatsApp…",
    startChat: "Start chat",
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
    sent: "Terkirim",
    sending: "Mengirim",
    newChat: "Mulai chat baru",
    full: "Sedang penuh, coba lagi nanti.",
    unavailable: "Chat tidak tersedia.",
    typing: "mengetik...",
    guestName: "Nama kamu",
    guestNameError: "Pakai 1–40 huruf, angka, spasi, atau . - ' _",
    guestContact: "Kontak (opsional) — email, WhatsApp…",
    startChat: "Mulai chat",
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
