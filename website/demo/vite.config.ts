import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

const root = path.dirname(fileURLToPath(import.meta.url));
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
    "import.meta.env.HOARD_VERSION": JSON.stringify("0.1.5"),
    "import.meta.env.HOARD_RELEASE_NOTES": JSON.stringify(""),
  },
  build: { outDir: path.resolve(root, "../public/demo"), emptyOutDir: true },
});
