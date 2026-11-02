-- name: ListLaunchOpenTokensNotOnKey :many
SELECT id, open_token_enc FROM launches
WHERE open_token_enc IS NOT NULL
  AND substring(open_token_enc FROM 1 FOR length(sqlc.arg(prefix)::bytea)) <> sqlc.arg(prefix)::bytea
  AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);

-- name: RotateLaunchOpenToken :execrows
UPDATE launches SET open_token_enc = sqlc.arg(enc_new)
WHERE id = sqlc.arg(id) AND open_token_enc = sqlc.arg(enc_old);

-- name: ListInviteTokensNotOnKey :many
SELECT id, invite_token_enc FROM crew_members
WHERE invite_token_enc IS NOT NULL
  AND substring(invite_token_enc FROM 1 FOR length(sqlc.arg(prefix)::bytea)) <> sqlc.arg(prefix)::bytea
  AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);

-- name: RotateInviteToken :execrows
UPDATE crew_members SET invite_token_enc = sqlc.arg(enc_new)
WHERE id = sqlc.arg(id) AND invite_token_enc = sqlc.arg(enc_old);

-- name: AnyTokenNotOnKey :one
-- True when some stored ciphertext is off the current key (the boot check).
SELECT
    EXISTS (SELECT 1 FROM buffer_connections WHERE key_id <> sqlc.arg(key_id) AND access_token_enc <> ''::bytea)
 OR EXISTS (SELECT 1 FROM launches WHERE open_token_enc IS NOT NULL
            AND substring(open_token_enc FROM 1 FOR length(sqlc.arg(prefix)::bytea)) <> sqlc.arg(prefix)::bytea)
 OR EXISTS (SELECT 1 FROM crew_members WHERE invite_token_enc IS NOT NULL
            AND substring(invite_token_enc FROM 1 FOR length(sqlc.arg(prefix)::bytea)) <> sqlc.arg(prefix)::bytea)
 AS pending;
