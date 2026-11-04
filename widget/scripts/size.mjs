import { readFileSync } from "node:fs";
import { gzipSync } from "node:zlib";
const LIMIT = 40960;
const size = gzipSync(readFileSync(new URL("../dist/widget.js", import.meta.url))).length;
console.log(`widget.js gzip: ${size} bytes (limit ${LIMIT})`);
process.exit(size > LIMIT ? 1 : 0);
