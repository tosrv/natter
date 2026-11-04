# Desain: Nama Tamu Wajib + Kontak Opsional (web & widget)

Tanggal: 2026-11-04 · Status: menunggu review

## 1. Tujuan

Tamu wajib mengisi nama tampilan sebelum mulai chat, di web dan widget. Kolom nama terisi otomatis `natter-xxxx` (boleh diubah). Kontak (email/WhatsApp/dll.) opsional, di dialog yang sama. Widget kini juga bisa meninggalkan kontak (sebelumnya tidak ada).

**Kriteria sukses**
- Developer melihat nama yang dipilih tamu (bukan "Guest XXXX") di daftar percakapan.
- Kontak yang diisi sampai ke developer sebagai pesan pertama tamu.
- Satu dialog/form dengan urutan sama di web dan widget: nama → kontak (opsional) → Turnstile → Mulai chat.

**Di luar cakupan:** menyimpan kontak di kolom/tabel tersendiri; mengubah username tamu (`guest-xxxx` tetap); fallback untuk klien lama.

## 2. Backend

`POST /api/auth/guest` body: `{"turnstile_token": string, "display_name": string}`.

Urutan di handler `Guest`:
1. Verifikasi Turnstile (seperti sekarang).
2. `chat.ValidateDisplayName(display_name)` → gagal: 400 `display_name_invalid`. Tanpa memakai rate limit atau slot.
3. Rate limit → `CreateGuest(ctx, name)` → sesi.

Batas body tetap 4 KiB (cukup untuk nama ≤ 40 rune). Saat Turnstile nonaktif (dev/E2E tanpa key), body tetap dibaca untuk `display_name`.

`chat.Service.CreateGuest(ctx, displayName string)` dan `store.CreateGuest(ctx, devID, displayName)` menyimpan nama itu sebagai `display_name`; username tetap `guest-<suffix>` dibuat server. Kontak tidak diproses server.

## 3. Nama acak (klien)

`natter-` + 4 karakter acak dari `[a-z0-9]` (contoh `natter-k7q2`), via `crypto.getRandomValues`. Lolos `ValidateDisplayName`. Dibuat ulang setiap kali form dibuka. Tombol "Mulai chat" nonaktif bila nama (setelah trim) kosong atau > 40 karakter, atau kontak > batas panjang pesan.

## 4. Web

- Komponen baru `GuestStartDialog` (shadcn `Dialog`): input nama (terisi acak), textarea kontak opsional (batas `MAX_BODY`), Turnstile, tombol Batal / Mulai chat. Turnstile pindah dari Landing ke dialog.
- Tombol "Lanjut sebagai tamu" (kedua tempat di Landing) membuka dialog ini.
- Sukses → navigasi ke chat dengan state `{ guestContact?: string }`; `useChat` mengirim kontak sebagai pesan pertama ke percakapan tamu (lewat outbox biasa) lalu membersihkan state.
- Dihapus: `GuestContactDialog`, state `contactOpen`/`setContactOpen`, penanda `newGuest`, dan kunci i18n yang tidak lagi dipakai.
- Error `display_name_invalid` → pesan di bawah input nama; error lain seperti sekarang.

## 5. Widget

- Fase `turnstile` diganti form: input nama, textarea kontak opsional, Turnstile, tombol "Mulai chat" (token Turnstile disimpan, dikirim saat tombol ditekan; bukan auto-submit).
- `controller.startGuest(turnstileToken: string, displayName: string, contact?: string)`. Setelah chat tersambung, kontak yang tidak kosong di-`queue` ke outbox sebagai pesan pertama.
- `api.createGuest(turnstileToken, displayName)`.
- `display_name_invalid` → tetap di form, pesan error di bawah input nama.
- "Mulai chat baru" membuka form dengan nama acak baru.

## 6. Teks UI (EN / ID)

| Kunci | EN | ID |
|---|---|---|
| nama | Your name | Nama kamu |
| kontak | Contact (optional) — email, WhatsApp… | Kontak (opsional) — email, WhatsApp… |
| tombol | Start chat | Mulai chat |
| error nama | Use 1–40 letters, numbers, spaces or . - ' _ | Pakai 1–40 huruf, angka, spasi, atau . - ' _ |

## 7. Pengujian

- **Go:** nama valid tersimpan sebagai `display_name`; nama kosong/terlalu panjang/karakter ilegal → 400 `display_name_invalid`; nama ilegal tidak memakan slot akun maupun token rate limit.
- **Vitest (web & widget):** generator nama acak menghasilkan pola `^natter-[a-z0-9]{4}$`; widget `startGuest` dengan kontak → pesan kontak di outbox, tanpa kontak → outbox kosong; `display_name_invalid` → tetap di form.
- **Playwright:** web dan widget — isi nama + kontak → developer melihat nama tamu di daftar percakapan dan pesan kontak. Spec E2E lama yang membuat tamu disesuaikan dengan dialog baru.

**Verifikasi sendiri:**

```bash
go test -race ./...
cd web && npx vitest run
cd widget && npm test && npm run build && npm run size
```

## 8. Kompatibilitas

`widget.js` lama yang masih ter-cache (maks. 5 menit) gagal membuat tamu karena tidak mengirim `display_name`. Diterima tanpa fallback.
