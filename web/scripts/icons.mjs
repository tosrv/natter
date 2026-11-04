// Generates PNG icons from the committed SVGs. Run: node scripts/icons.mjs
import sharp from "sharp";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const pub = join(dirname(fileURLToPath(import.meta.url)), "..", "public");
const favicon = readFileSync(join(pub, "favicon.svg"));
const bubble = /d="([^"]+)"/.exec(favicon.toString())[1];

// Opaque tile: white bubble on natter orange, with safe-zone padding.
const tile = (size) => Buffer.from(
  `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="-8 -8 48 48">` +
  `<rect x="-8" y="-8" width="48" height="48" fill="#E25C2F"/><path fill="#fff" d="${bubble}"/></svg>`);

await sharp(favicon, { density: 384 }).resize(32, 32).png({ compressionLevel: 9 }).toFile(join(pub, "favicon-32.png"));
await sharp(tile(180)).png({ compressionLevel: 9 }).toFile(join(pub, "apple-touch-icon.png"));
await sharp(tile(512)).png({ compressionLevel: 9 }).toFile(join(pub, "icon-512.png"));
