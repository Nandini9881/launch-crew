# GitHub App setup

Settings Launch Crew needs on its GitHub App (for the Task 29 runbook).

- Webhook URL: `<BASE_URL>/webhooks/github`; secret in `GITHUB_WEBHOOK_SECRET`. Subscribe to Release events.
- Repository permissions: Contents read & write, Pull requests read, Issues write (PR comments), Metadata read.
- Setup URL (post-installation): `<BASE_URL>/github/install/callback`.
- Callback URL (user authorization): `<BASE_URL>/github/verify/callback` and `<BASE_URL>/github/install/callback`.
- Enable **Request user authorization (OAuth) during installation**. The install callback then receives a `code`, which is exchanged for a user token and checked against `GET /user/installations` so a captain can only bind installations they can access. Without the setting the callback sends the captain through user authorization itself, which also works.
- Leave "Expire user authorization tokens" on; the tokens are used once and never stored.
- Env: `GITHUB_APP_ID`, `GITHUB_APP_SLUG`, `GITHUB_PRIVATE_KEY`, `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`.
