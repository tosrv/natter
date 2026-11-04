const ALPHABET = "abcdefghijklmnopqrstuvwxyz0123456789";

/** `natter-` + 4 random [a-z0-9] chars, via crypto.getRandomValues. */
export function randomGuestName(): string {
  const bytes = new Uint8Array(4);
  crypto.getRandomValues(bytes);
  let s = "natter-";
  for (const b of bytes) s += ALPHABET[b % ALPHABET.length];
  return s;
}
