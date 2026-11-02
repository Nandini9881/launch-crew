-- name: EnqueueJob :one
INSERT INTO jobs (kind, payload, run_at, launch_id)
VALUES (@kind, @payload, COALESCE(sqlc.narg('run_at')::timestamptz, now()), @launch_id)
RETURNING *;

-- name: ClaimJob :one
UPDATE jobs SET locked_at = now(), locked_by = $1
WHERE id = (
    SELECT id FROM jobs
    WHERE locked_at IS NULL AND dead = false AND run_at <= now()
    ORDER BY run_at
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: CompleteJob :exec
DELETE FROM jobs WHERE id = $1;

-- name: FailJob :exec
UPDATE jobs
SET attempts = attempts + 1, last_error = $2, run_at = $3,
    locked_at = NULL, locked_by = NULL, dead = (attempts + 1 >= 5)
WHERE id = $1;

-- name: ListDeadLetters :many
SELECT * FROM jobs
WHERE dead = true AND (sqlc.narg('launch_id')::uuid IS NULL OR launch_id = sqlc.narg('launch_id'))
ORDER BY id;

-- name: RetryJob :exec
UPDATE jobs
SET dead = false, attempts = 0, run_at = now(), locked_at = NULL, locked_by = NULL
WHERE id = $1;

-- name: CountJobsOfKind :one
SELECT count(*) FROM jobs WHERE kind = $1 AND launch_id = $2;

-- name: ReleaseJob :exec
UPDATE jobs SET locked_at = NULL, locked_by = NULL WHERE id = $1;

-- name: ReapStaleLocks :execrows
UPDATE jobs SET locked_at = NULL, locked_by = NULL
WHERE locked_at IS NOT NULL AND locked_at < now() - sqlc.arg('older_than')::interval AND dead = false;

-- name: CountLiveJobsOfKind :one
SELECT count(*) FROM jobs WHERE kind = $1 AND dead = false;

-- name: CountLiveMemberJobs :one
SELECT count(*) FROM jobs
WHERE kind = sqlc.arg(kind) AND dead = false AND payload->>'MemberID' = sqlc.arg(member_id)::text;

-- name: CountLiveForcedMemberJobs :one
SELECT count(*) FROM jobs
WHERE kind = sqlc.arg(kind) AND dead = false AND payload->>'MemberID' = sqlc.arg(member_id)::text
  AND payload->>'Force' = 'true';
