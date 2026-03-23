import { defineConfig } from "astro/config";

export default defineConfig({
  output: "static",
  outDir: "../services/relay/internal/webui/dist",
  trailingSlash: "always",
});
