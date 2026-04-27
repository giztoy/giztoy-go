import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  expect: {
    timeout: 10_000,
  },
  fullyParallel: false,
  outputDir: "../.workspace/ui-real-service/test-results",
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  reporter: [["list"], ["html", { open: "never", outputFolder: "../.workspace/ui-real-service/playwright-report" }]],
  testDir: ".",
  testMatch: "**/*.real.spec.ts",
  use: {
    trace: "on-first-retry",
  },
  workers: 1,
});
