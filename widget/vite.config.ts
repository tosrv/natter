import { defineConfig } from "vitest/config";
import preact from "@preact/preset-vite";

export default defineConfig({
  plugins: [preact()],
  define: { "process.env.NODE_ENV": JSON.stringify("production") },
  build: {
    lib: { entry: "src/main.ts", name: "PmessWidget", formats: ["iife"], fileName: () => "widget.js" },
    cssCodeSplit: false,
    minify: "esbuild",
  },
  test: { environment: "jsdom" },
});
