# Buffer API notes

## OAuth endpoints

Checked 2026-11-01 against https://developers.buffer.com/guides/building-apps.html and https://developers.buffer.com/guides/authentication.html.

| Purpose | URL |
| --- | --- |
| Authorize (GET, browser) | `https://auth.buffer.com/auth` |
| Token (POST, form-encoded) | `https://auth.buffer.com/token` |
| API (POST, GraphQL, `Authorization: Bearer`) | `https://api.buffer.com` |
| App registration | `https://publish.buffer.com/settings/api` |

- Flow: authorization code with PKCE (`code_challenge_method=S256`), fresh `state` per request.
- Authorize params: `client_id`, `redirect_uri`, `response_type=code`, `scope`, `state`, `code_challenge`, `code_challenge_method`.
- Scopes used: `account:read posts:read posts:write offline_access` (`offline_access` is required to receive a refresh token).
- Client credentials go in the token request body, not a Basic header. Confidential clients send `client_id` and `client_secret` plus `code_verifier`; public clients omit the secret.
- Redirect URIs must be HTTPS (localhost included) and match character for character on authorize and exchange.
- Access tokens last about an hour. Refresh tokens rotate: each refresh returns a new one and invalidates the old; reusing an old one revokes the grant. Save the replacement before using it, and let one component own refreshing.

## GraphQL operations

Checked 2026-11-01 against https://developers.buffer.com/reference.md (the GraphQL reference as Markdown), the guides `posts-and-scheduling`, `error-handling`, `character-limits`, `api-limits`, `authentication`, `pagination`, `efficient-api-usage`, and the examples `get-channels`, `create-scheduled-post`, `create-draft-post`, `create-threaded-post`. The strings live in `queries.go`; every operation is named so the audit and the fake can key on it.

| Op name | Root field | Arguments | Selected fields |
| --- | --- | --- | --- |
| `Account` (auth package) | `account` | none | `id organizations { id }` |
| `GetChannels` | `channels(input: ChannelsInput!)` | `{ organizationId }` | `id name displayName service avatar isLocked isQueuePaused isDisconnected` |
| `GetConfiguration` | `configuration(input: ConfigurationInput!)` (Experimental) | `{ organizationId }` | `services { service channelType content { configurationContentTypes supportedProperties rules { __typename property ... on LengthRule { maxLength } } } }` |
| `GetSentPosts` | `posts(first: Int, after: String, input: PostsInput!)` | `first: 100`, `filter: { status: [sent] }`, `sort: [{ field: createdAt, direction: desc }]` | `edges { node { id channelId text status sentAt createdAt } }` |
| `GetPostsCreatedBetween` | `posts` | `first: 100`, `filter: { channelIds: [id], createdAt: { start, end } }`, `sort: createdAt asc` | as above |
| `CreatePost` | `createPost(input: CreatePostInput!)` returns union `PostActionPayload` | see below | `__typename ... on PostActionSuccess { post { id status } } ... on MutationError { message }` |
| `GetDailyPostingLimits` | `dailyPostingLimits(input: DailyPostingLimitsInput!)` | `{ channelIds }` (required; `date` defaults to now) | `channelId isAtLimit limit scheduled sent` (`limit` null = unlimited) |

`CreatePostInput` fields used: `channelId`, `text`, `schedulingType: automatic` (required), `mode` (`ShareMode`: `addToQueue` or `customScheduled`), `dueAt` (ISO 8601, with `customScheduled`), `saveToDraft` (drafts skip posting limits), `assets: [{ image: { url } } | { video: { url } }]` (`AssetInput` wants exactly one variant; the client picks `video` by file extension .mp4/.mov/.m4v/.webm, else `image`), `metadata`.
- Threads: `metadata.{twitter|bluesky|threads|mastodon}.thread: [{ text }]`. The array is the whole thread including the root, and top-level `text` must equal the first item.
- First comment: `metadata.{linkedin|facebook|instagram}.firstComment`.
- The metadata key matches the `Service` enum except Google Business, whose key is `google`.

`Service` enum values (lowercase, used as `Config` keys): `bluesky facebook googlebusiness instagram linkedin mastodon pinterest startPage substack threads tiktok twitter whatsapp youtube`. X is `twitter`.

`PostStatus` values: `draft error needs_approval scheduled sending sent`. `PostSortableKey` is only `dueAt` or `createdAt`, so "sent posts" are sorted by `createdAt`.

### Errors

- Typed mutation errors come back in `data` with HTTP 200. `createPost`'s union is `PostActionSuccess | NotFoundError | UnauthorizedError | UnexpectedError | RestProxyError | LimitReachedError | InvalidInputError`. A full queue is `__typename: "LimitReachedError"` with a `message`, and no code. The client selects `__typename` to tell them apart, and on `LimitReachedError` retries once with `saveToDraft: true`.
- Non-recoverable errors go in `errors[]` with `extensions.code`: `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`, `UNEXPECTED`, `RATE_LIMIT_EXCEEDED`.
- Character-limit violations are mutation errors with messages like `LinkedIn posts cannot exceed 3000 characters.`

### Configuration: which fields come from where

From the API, the `content` entry whose `configurationContentTypes` includes `post`:
- `MaxChars` is the `LengthRule.maxLength` whose `property` is `text`.
- `Threads`, `ImageOK` and `FirstComment` are true when `thread`, `image` and `firstComment` appear in `supportedProperties`.

When a service has several channel types (Facebook page and group, LinkedIn page and profile), the client keeps the lowest `MaxChars` and sets a capability only if every channel type has it.

Built-in table in `operations.go`, taken from the character-limits and posts guides, used for services missing from the response and for a missing `MaxChars`:

| Service | MaxChars | Threads | Image | First comment |
| --- | --- | --- | --- | --- |
| twitter | 280 (free tier; paid is 25,000) | yes | yes | no |
| bluesky | 300 | yes | yes | no |
| threads | 500 | yes | yes | no |
| mastodon | 500 (server default; real value is `MastodonMetadata.maxCharacters`) | yes | yes | no |
| linkedin | 3000 | no | yes | yes |
| facebook | 5000 | no | yes | yes |
| instagram | 2196 | no | yes | yes |
| youtube | 5000 | no | no | no |
| tiktok | 2200 (with a video; 4000 without) | no | yes | no |
| pinterest | 500 | no | yes | no |
| googlebusiness | 4000 | no | yes | no |

If the `configuration` query returns a GraphQL error (it is Experimental), the client returns the built-in table and does not cache it.

### Rate limits

From https://developers.buffer.com/guides/api-limits.html, checked 2026-11-01.
- There are no `RateLimit-Remaining` or `RateLimit-Reset` headers. Buffer sends structured `RateLimit` and `RateLimit-Policy` headers, one entry per window (15 min, 1 day, 30 days), repeated or comma-joined:
  ```
  RateLimit:        "150-in-15min";r=98;t=897
  RateLimit-Policy: "150-in-15min";q=150;w=900;pk=:ZjJj:
  ```
  `r` is the requests remaining, `t` the seconds to reset, `q` the quota and `w` the window in seconds.
- Policy names change with the plan, so the docs say to match on `w`. The client tracks the 15-minute window (`w=900`) and falls back to the first `RateLimit` entry when no policy matches. It stores `r` in `rate_remaining` and `now + t` in `rate_reset_at`.
- Before each call, if `rate_remaining` is below 20% of the window, the client sleeps until `rate_reset_at`. The window is the largest `q` (or `r`) this process has seen for the connection, and 100 when unknown.
- App client limits: 100 per 15 min base plus 10 per authorized user; 500 per day plus 25 per user; 15,000 per 30 days plus 100 per user.
- A 429 carries `Retry-After` (seconds; it matches `t`) and `errors[0].extensions.code = RATE_LIMIT_EXCEEDED` with `extensions.window`. A 429 does not consume quota. The client waits `Retry-After` once (60 s if the header is missing), then gives up with an error naming both waits. Both this wait and the budget wait are capped at 15 minutes (one window); a longer wait returns an error naming it instead of sleeping.
- An expired or revoked token gets `401 Unauthorized` (authentication guide). The error-handling guide also lists `UNAUTHORIZED` in `errors[]` with HTTP 200, so the client treats either as the refresh trigger.

### Ambiguities and unverified points

- **Request id header**: the docs do not mention one. The client records `X-Request-Id` and stores NULL when it is absent. Confirm with the first real call (deferred manual check).
- **`posts` page size**: the docs give no maximum for `first`; the pagination guide suggests 20 to 50. The client asks for 100, as the plan says. `PostsCreatedBetween` reads one page only.
- **`configuration`** is Experimental, and the type notes say a field is public only when tagged `@public`. The reference lists `services`, but it is not confirmed that third-party tokens can read it, hence the built-in fallback.
- **Channel `Name`** is `displayName` when set, otherwise `name` (the handle).
- **`DailyPostingLimits`** needs channel ids, so it reads the cached channel list and asks about every unlocked channel.
- Refresh calls to the token endpoint are not written to `buffer_calls`, which audits API (GraphQL) calls only.
