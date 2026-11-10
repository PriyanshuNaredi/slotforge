import { defineConfig } from "@playwright/test";

/**
 * Playwright runs against a fully local stack:
 *   - API:   http://localhost:8081 (DATABASE_URL points at slotforge_e2e)
 *   - Web:   http://localhost:3100 (dev server, API_PROXY_ORIGIN=8081)
 * Start both before running; this config does not manage them.
 */
export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [["list"]],
  use: {
    baseURL: "http://localhost:3100",
    trace: "retain-on-failure",
  },
});
