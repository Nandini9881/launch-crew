-- name: UpsertBufferConnection :one
INSERT INTO buffer_connections
    (user_id, buffer_account_id, org_id, access_token_enc, refresh_token_enc, key_id, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (buffer_account_id) DO UPDATE SET
    org_id = EXCLUDED.org_id,
    access_token_enc = EXCLUDED.access_token_enc,
    refresh_token_enc = EXCLUDED.refresh_token_enc,
    key_id = EXCLUDED.key_id,
    expires_at = EXCLUDED.expires_at,
    status = 'ok'
RETURNING *;

-- name: GetBufferConnection :one
SELECT * FROM buffer_connections WHERE id = $1;

-- name: GetBufferConnectionByAccount :one
SELECT * FROM buffer_connections WHERE buffer_account_id = $1;

-- name: UpdateConnectionTokens :exec
UPDATE buffer_connections
SET access_token_enc = $2, refresh_token_enc = $3, key_id = $4, expires_at = $5
WHERE id = $1;

-- name: UpdateConnectionStatus :exec
UPDATE buffer_connections SET status = $2 WHERE id = $1;

-- name: UpdateConnectionChannelsCache :exec
UPDATE buffer_connections SET channels_json = $2, channels_cached_at = $3 WHERE id = $1;

-- name: UpdateConnectionConfigCache :exec
UPDATE buffer_connections SET config_json = $2, config_cached_at = $3 WHERE id = $1;

-- name: UpdateConnectionRate :exec
UPDATE buffer_connections SET rate_remaining = $2, rate_reset_at = $3 WHERE id = $1;

-- name: ListConnectionsByUser :many
SELECT * FROM buffer_connections WHERE user_id = $1 ORDER BY buffer_account_id;

-- name: ListConnectionsNotOnKey :many
-- Cleared (disconnected) rows hold no ciphertext and are skipped.
SELECT * FROM buffer_connections
WHERE key_id <> sqlc.arg(key_id) AND access_token_enc <> ''::bytea AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);

-- name: RotateConnectionTokens :execrows
-- Guarded on the old access token so a concurrent refresh is not clobbered.
UPDATE buffer_connections
SET access_token_enc = sqlc.arg(access_new), refresh_token_enc = sqlc.arg(refresh_new), key_id = sqlc.arg(key_id)
WHERE id = sqlc.arg(id) AND access_token_enc = sqlc.arg(access_old);

-- name: DeleteConnectionsByUser :exec
DELETE FROM buffer_connections WHERE user_id = $1;

-- name: ClearConnectionTokens :exec
-- Disconnect: drop the stored tokens; a new OAuth sign-in restores them.
UPDATE buffer_connections
SET access_token_enc = ''::bytea, refresh_token_enc = NULL, expires_at = NULL,
    status = 'needs_reconnect', channels_json = NULL, channels_cached_at = NULL
WHERE id = $1;
