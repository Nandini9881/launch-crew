import { execFileSync } from 'node:child_process'
import { defineConfig, devices } from '@playwright/test'

// Pick a free port once; workers inherit it through the environment.
if (!process.env.E2E_PORT) {
  const script = "const s=require('net').createServer();s.listen(0,'127.0.0.1',()=>{console.log(s.address().port);s.close()})"
  process.env.E2E_PORT = execFileSync(process.execPath, ['-e', script]).toString().trim()
}
const baseURL = `http://localhost:${process.env.E2E_PORT}`

export default defineConfig({
  testDir: '.',
  // Screenshots are opt-in: SCREENSHOTS=1 runs only that spec.
  testMatch: process.env.SCREENSHOTS ? 'screenshots.spec.ts' : '*.spec.ts',
  testIgnore: process.env.SCREENSHOTS ? [] : ['screenshots.spec.ts'],
  // One server, one database: specs reset shared state, so run them one at a time.
  fullyParallel: false,
  workers: 1,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  retries: 0, // a flaky flow is a bug, in CI too
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: { baseURL, trace: 'retain-on-failure' },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'webkit', use: { ...devices['Desktop Safari'] } },
  ],
  webServer: {
    command: 'node fixtures/server.ts',
    url: `${baseURL}/healthz`,
    timeout: 180_000,
    reuseExistingServer: false,
    stdout: 'pipe',
    stderr: 'pipe',
  },
})
