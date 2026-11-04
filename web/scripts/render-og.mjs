// Renders scripts/og.html to shots-raw/og.png; optimize-images.mjs then writes public/og.png.
import { chromium } from "@playwright/test";
import path from "node:path";
import fs from "node:fs";

const root = path.resolve(path.dirname(new URL(import.meta.url).pathname), "..");
fs.mkdirSync(path.join(root, "shots-raw"), { recursive: true });
const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1200, height: 630 } });
await page.goto("file://" + path.join(root, "scripts/og.html"));
const geist = await page.evaluate(async () => { await document.fonts.ready; return document.fonts.check('600 60px "Geist"'); });
if (!geist) throw new Error("Geist did not load");
await page.screenshot({ path: path.join(root, "shots-raw/og.png") });
await browser.close();
