// Builds the web app and the Go binary, then runs it with LAUNCHCREW_E2E=1 against the
// in-process fakes. Playwright's webServer starts this; it stops on SIGTERM.
// E2E_SKIP_BUILD=1 reuses the last build.
import { execFileSync, spawn } from 'node:child_process'
import { mkdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..', '..')
const bin = join(root, 'e2e', '.bin', 'launchcrew')
const port = process.env.E2E_PORT ?? '4517'

if (process.env.E2E_SKIP_BUILD !== '1') {
  execFileSync('npm', ['run', 'build'], { cwd: join(root, 'web'), stdio: 'inherit' })
  mkdirSync(dirname(bin), { recursive: true })
  execFileSync('go', ['build', '-o', bin, './cmd/launchcrew'], { cwd: root, stdio: 'inherit' })
}

const child = spawn(bin, [], {
  cwd: root,
  stdio: 'inherit',
  env: {
    ...process.env,
    LAUNCHCREW_E2E: '1',
    PORT: port,
    DATABASE_URL: process.env.DATABASE_URL ?? 'postgres://postgres:postgres@localhost:5432/launchcrew?sslmode=disable',
  },
})
for (const sig of ['SIGINT', 'SIGTERM'] as const) process.on(sig, () => child.kill(sig))
child.on('exit', (code) => process.exit(code ?? 0))
