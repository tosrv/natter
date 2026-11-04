# pmess embeddable chat widget

A small chat bubble (`<pmess-chat>` custom element, Shadow DOM, Preact) that lets visitors of any website message the pmess owner as a guest.

## Install

Add two lines to your page:

```html
<script src="https://chat.rahmattomy.com/widget.js" defer></script>
<pmess-chat></pmess-chat>
```

### Attributes (all optional)

| Attribute | Default | Notes |
|---|---|---|
| `server` | origin of `widget.js` | pmess server for REST and WebSocket calls |
| `lang` | from the browser language | `en` or `id` |
| `color` | built-in accent | primary color, any CSS color |

Light/dark appearance follows the visitor's system setting.

## Operator setup (once per host site)

1. Add the site's origin (`https://<site>`) to `ALLOWED_ORIGINS` in the VPS `.env`, then restart the app (`docker compose up -d`).
2. Add the site's hostname to the Cloudflare Turnstile widget.
3. Verify: `curl -I https://chat.rahmattomy.com/widget.js` should return `Cross-Origin-Resource-Policy: cross-origin`.

Details: [operations guide](../docs/operations.md).

## Host-site CSP

If the host site sends a Content-Security-Policy, allow:

- `script-src https://chat.rahmattomy.com https://challenges.cloudflare.com`
- `connect-src https://chat.rahmattomy.com wss://chat.rahmattomy.com`
- `frame-src https://challenges.cloudflare.com`

## Limitations

- Guest only: no Google login in the widget.
- The session token is kept in the host site's `localStorage` (in-memory fallback when storage is unavailable).
- Single owner: every conversation goes to the one pmess owner. No multi-tenant support, no API key.

## Development

From `widget/`:

```bash
npm test        # Vitest (jsdom)
npm run build   # produces dist/widget.js
npm run size    # fails if widget.js exceeds 40960 bytes gzipped
```
