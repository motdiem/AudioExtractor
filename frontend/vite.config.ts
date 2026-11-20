import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import wails from "@wailsio/runtime/plugins/vite";

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
    fs: {
      // The dev-only screen previews (src/preview.ts) read the Go catalogs.
      allow: [".", fileURLToPath(new URL("../internal/i18n/locales", import.meta.url))],
    },
  },
  plugins: [wails("./bindings")],
});
