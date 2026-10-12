// Runs inside the visual runner container (see run.mjs): Storybook's static build is mounted at
// /storybook and this directory at /runner/suite.
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: '.',
  testMatch: 'storybook.visual.ts',
  snapshotPathTemplate: '{testDir}/baseline/{arg}{ext}',
  outputDir: './.results/output',
  reporter: [['list'], ['html', { outputFolder: './.results/report', open: 'never' }]],
  fullyParallel: true,
  workers: 4,
  retries: 0,
  // The first story per worker includes browser start-up.
  timeout: 60_000,
  expect: {
    timeout: 20_000,
    toHaveScreenshot: { maxDiffPixelRatio: 0.002, animations: 'disabled', caret: 'hide', scale: 'css' },
  },
  use: {
    baseURL: 'http://127.0.0.1:6007',
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 1,
    colorScheme: 'light',
    reducedMotion: 'reduce',
    timezoneId: 'UTC',
    locale: 'en-GB',
  },
  webServer: {
    command: 'node serve.mjs /storybook 6007',
    url: 'http://127.0.0.1:6007/index.json',
    reuseExistingServer: false,
  },
});
