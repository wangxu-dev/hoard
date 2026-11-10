import path from "node:path";
import { fileURLToPath } from "node:url";
import { readFileSync } from "node:fs";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

const root = path.dirname(fileURLToPath(import.meta.url));
const appVersion = JSON.parse(readFileSync(path.resolve(root, "../../mygo.json"), "utf8")).version;
export default defineConfig({
  root,
  base: "/demo/",
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(root, "../../web/src"),
      "@mygo": path.resolve(root, "backend.ts"),
    },
  },
  define: {
    "import.meta.env.HOARD_VERSION": JSON.stringify(appVersion),
    "import.meta.env.HOARD_RELEASE_NOTES": JSON.stringify(""),
  },
  build: { outDir: path.resolve(root, "../public/demo"), emptyOutDir: true },
});
