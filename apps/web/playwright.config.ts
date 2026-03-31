import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  reporter: "list",
  use: {
    baseURL: "http://127.0.0.1:4322",
    trace: "on-first-retry",
  },
  webServer: {
    command: "bun run build && bun run preview",
    url: "http://127.0.0.1:4322",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
  projects: [
    {
      name: "desktop-chromium",
      use: {
        ...devices["Desktop Chrome"],
        viewport: {
          width: 1440,
          height: 900,
        },
      },
    },
    {
      name: "mobile-chromium",
      use: {
        ...devices["Pixel 5"],
      },
    },
  ],
});
