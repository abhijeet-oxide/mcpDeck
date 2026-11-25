import { writeFileSync } from "node:fs";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    // go:embed needs dist/ to exist on a fresh checkout; keep its placeholder.
    { name: "keep-dist", closeBundle: () => writeFileSync("dist/.gitkeep", "") },
  ],
  server: {
    port: 5173,
    proxy: {
      "/api": { target: "http://127.0.0.1:47800", changeOrigin: true },
      "/launch": { target: "http://127.0.0.1:47800", changeOrigin: true },
    },
  },
  build: { outDir: "dist", emptyOutDir: true, chunkSizeWarningLimit: 6000 },
  test: { globals: true, environment: "jsdom", setupFiles: ["./src/test-setup.ts"], css: false },
});
