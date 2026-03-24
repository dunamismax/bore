import path from "node:path";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  build: {
    outDir: "../services/relay/internal/webui/dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/status": "http://127.0.0.1:8080",
    },
  },
});
