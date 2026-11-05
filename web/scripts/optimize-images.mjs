// Turns raw @2x captures (web/shots-raw, from `PRODUCT_SHOTS=1 … playwright test screenshots`)
// into public/product/<variant>-<theme>[@2x].{avif,webp,png}, and og.png into public/og.png.
//   node scripts/optimize-images.mjs
import sharp from "sharp";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const RAW = path.join(root, "shots-raw");
const OUT = path.join(root, "public/product");
fs.mkdirSync(OUT, { recursive: true });

// width = 1x CSS width; crop (in raw 2x pixels) optional.
const VARIANTS = {
  desktop: { width: 1440 },
  mobile: { width: 390 },
  // Header (typing…) + all bubbles with ✓/✓✓ from the phone capture.
  receipts: { width: 390, from: "mobile", crop: { left: 0, top: 0, width: 780, height: 1120 } },
};

async function emit(src, name, w) {
  const base = sharp(src).resize({ width: w });
  await base.clone().avif({ quality: 50, effort: 6 }).toFile(path.join(OUT, `${name}.avif`));
  await base.clone().webp({ quality: 75 }).toFile(path.join(OUT, `${name}.webp`));
  await base.clone().png({ compressionLevel: 9, palette: true, quality: 90 }).toFile(path.join(OUT, `${name}.png`));
}

for (const [v, { width, crop, from }] of Object.entries(VARIANTS)) {
  for (const theme of ["light", "dark"]) {
    let buf = fs.readFileSync(path.join(RAW, `${from ?? v}-${theme}.png`));
    if (crop) buf = await sharp(buf).extract(crop).toBuffer();
    await emit(buf, `${v}-${theme}`, width);
    await emit(buf, `${v}-${theme}@2x`, width * 2);
    const m = await sharp(path.join(OUT, `${v}-${theme}.png`)).metadata();
    console.log(v, theme, `${m.width}x${m.height}`);
  }
}

const og = path.join(RAW, "og.png");
if (fs.existsSync(og)) {
  await sharp(og).resize(1200, 630).png({ compressionLevel: 9, palette: true, quality: 90 }).toFile(path.join(root, "public/og.png"));
  console.log("og.png written");
}
