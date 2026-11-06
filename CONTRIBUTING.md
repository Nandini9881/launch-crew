# Contributing

## Setup

See "Run locally" in the [README](README.md) for Postgres, environment and starting the server.

## Tests

Run these before opening a PR. CI runs the same checks.

- API: `go test ./...` from the repo root. The DB-backed tests need `DATABASE_URL` (`make db-up` starts a matching Postgres) and skip without it. CI runs `go test -p 1 ./...` because those tests share one database and each migrates it, so packages must not run in parallel.
- Web, from `web/`: `npm test` (Vitest), `npm run lint` (stylelint) and `npx tsc -b` (type check; `npm run build` runs it too).
- End to end, from `e2e/`: `npm ci`, `npx playwright install chromium webkit`, then `npx playwright test`. CI runs the `chromium` and `webkit` projects. Playwright starts its own server from `e2e/fixtures/server.ts`.

`make test` runs the Go and web suites together.

## Branches and pull requests

- Branch from `main`. Use hyphenated names, with no slash (for example `fix-invite-link`).
- Open a pull request against `main` and fill in the template.
- Merging to `main` deploys to Fly.io once CI's test job passes.
