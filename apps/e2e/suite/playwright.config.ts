import { defineConfig } from '@playwright/test';

// End-to-end suite (see ../run.mjs). Flows share seeded organisations and build on each other, so
// they run serially in one worker. Visual checks run only in the pinned container (E2E_VISUAL=1).
export default defineConfig({
  testDir: './tests',
  snapshotPathTemplate: '{testDir}/../baseline/{arg}{ext}',
  outputDir: './.results/output',
  reporter: [['list'], ['html', { outputFolder: './.results/report', open: 'never' }]],
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 90_000,
  expect: {
    timeout: 15_000,
    // An absolute budget: on a full page a ratio hides real changes (one sidebar row ≈ 2,000 px).
    toHaveScreenshot: { maxDiffPixels: 500, animations: 'disabled', caret: 'hide', scale: 'css' },
  },
  use: {
    baseURL: process.env.E2E_BASE_URL ?? 'http://localhost:8080',
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 1,
    colorScheme: 'light',
    reducedMotion: 'reduce',
    timezoneId: 'UTC',
    locale: 'en-GB',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
});
