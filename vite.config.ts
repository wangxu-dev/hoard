import path from "node:path";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { releaseNotes } from "./scripts/release-notes.ts";
import { defineConfig } from "vite";

const root = path.dirname(fileURLToPath(import.meta.url));

const buildInfo = JSON.parse(readFileSync(path.join(root, "mygo.json"), "utf8")) as { version?: string };

export default defineConfig({
  define: {
    "import.meta.env.HOARD_VERSION": JSON.stringify(buildInfo.version || "dev"),
    "import.meta.env.HOARD_RELEASE_NOTES": JSON.stringify(releaseNotes(readFileSync(path.join(root, "CHANGELOG.md"), "utf8"), buildInfo.version || "dev")),
  },
  envPrefix: ["VITE_", "HOARD_"],
  root: path.resolve(root, "web"),
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(root, "web/src"),
      "@mygo": path.resolve(root, ".mygo/bindings.gen.ts"),
    },
  },
  server: {
    port: 5173,
    strictPort: true,
    fs: { allow: [root] },
    watch: { ignored: ["**/.mygo/**", "**/build/**", "**/dist/**"] },
  },
  build: { outDir: path.resolve(root, "dist"), emptyOutDir: true },
});
