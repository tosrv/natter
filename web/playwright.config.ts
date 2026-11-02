import { defineConfig, devices } from "@playwright/test";

// Run against a server started with E2E_FAKE_LOGIN=1 and DEVELOPER_EMAIL=dev@e2e.test:
//   E2E_FAKE_LOGIN=1 DEVELOPER_EMAIL=dev@e2e.test docker compose up --build -d
//   E2E_BASE_URL=http://localhost:8080 npm run e2e
export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  workers: 1,
  reporter: "list",
  use: {
    ...devices["Desktop Chrome"],
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:8080",
    locale: "en-US",
    trace: "retain-on-failure",
  },
});
