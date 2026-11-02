import { defineConfig, devices } from "@playwright/test";

// Run against a server started with E2E_FAKE_LOGIN=1 and DEVELOPER_EMAIL=dev@e2e.test:
//   E2E_FAKE_LOGIN=1 DEVELOPER_EMAIL=dev@e2e.test docker compose up --build -d
//   E2E_BASE_URL=http://localhost:8080 npm run e2e
// Note: guest creation is limited to 5/hour per IP in production; with
// E2E_FAKE_LOGIN=1 the server raises it to 1000/hour so the suite can be rerun.
// Screenshots: SCREENSHOTS=1 npx playwright test screenshots (writes docs/screenshots/).
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
