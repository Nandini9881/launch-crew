-- name: ListGitHubIdentitiesByUser :many
SELECT * FROM github_identities WHERE user_id = $1 ORDER BY github_user_id;

-- name: OptOutUser :exec
INSERT INTO opt_outs (github_user_id)
SELECT gi.github_user_id FROM github_identities gi WHERE gi.user_id = sqlc.arg(uid)
UNION
SELECT cm.github_user_id FROM crew_members cm WHERE cm.user_id = sqlc.arg(uid) AND cm.github_user_id IS NOT NULL
ON CONFLICT DO NOTHING;

-- name: MarkUserCrewOptedOut :exec
UPDATE crew_members cm SET state = 'opted_out'
WHERE (cm.user_id = sqlc.arg(uid)
    OR cm.github_user_id IN (SELECT gi.github_user_id FROM github_identities gi WHERE gi.user_id = sqlc.arg(uid)))
  AND cm.github_user_id IS NOT NULL
  AND cm.state NOT IN ('queued', 'posted');

-- name: MarkGitHubCrewOptedOut :exec
UPDATE crew_members SET state = 'opted_out'
WHERE github_user_id = $1 AND state NOT IN ('queued', 'posted');

-- name: CountUserGitHubIDs :one
SELECT count(*) FROM (
    SELECT gi.github_user_id FROM github_identities gi WHERE gi.user_id = sqlc.arg(uid)
    UNION
    SELECT cm.github_user_id FROM crew_members cm WHERE cm.user_id = sqlc.arg(uid) AND cm.github_user_id IS NOT NULL
) ids;

-- name: DeleteUserVoiceProfiles :exec
DELETE FROM voice_profiles vp
WHERE vp.buffer_account_id IN (SELECT bc.buffer_account_id FROM buffer_connections bc WHERE bc.user_id = sqlc.arg(uid))
   OR vp.buffer_account_id IN (
        SELECT cm.buffer_account_id FROM crew_members cm
        WHERE cm.buffer_account_id IS NOT NULL
          AND (cm.user_id = sqlc.arg(uid)
               OR cm.github_user_id IN (SELECT gi.github_user_id FROM github_identities gi WHERE gi.user_id = sqlc.arg(uid))));

-- name: MarkCrewMemberOptedOut :exec
UPDATE crew_members SET state = 'opted_out' WHERE id = $1 AND state NOT IN ('queued', 'posted');

-- name: UserIsOptedOut :one
SELECT EXISTS (
    SELECT 1 FROM opt_outs o
    WHERE o.github_user_id IN (SELECT gi.github_user_id FROM github_identities gi WHERE gi.user_id = sqlc.arg(uid))
       OR o.github_user_id IN (SELECT cm.github_user_id FROM crew_members cm WHERE cm.user_id = sqlc.arg(uid))
);

-- name: CountUserLaunches :one
SELECT count(*) FROM launches WHERE owner_user_id = $1 AND status <> 'deleted';

-- name: CountUserCrewRows :one
SELECT count(*) FROM crew_members cm
WHERE cm.user_id = sqlc.arg(uid)
   OR cm.github_user_id IN (SELECT gi.github_user_id FROM github_identities gi WHERE gi.user_id = sqlc.arg(uid));

-- name: UserHasBufferConnection :one
SELECT EXISTS (SELECT 1 FROM buffer_connections WHERE user_id = $1);

-- name: DeleteUserCrewDrafts :exec
DELETE FROM crew_drafts d WHERE d.crew_member_id IN (
    SELECT cm.id FROM crew_members cm
    WHERE cm.user_id = sqlc.arg(uid)
       OR cm.github_user_id IN (SELECT gi.github_user_id FROM github_identities gi WHERE gi.user_id = sqlc.arg(uid)));

-- name: DeleteUserCrewMembers :exec
DELETE FROM crew_members cm
WHERE cm.user_id = sqlc.arg(uid)
   OR cm.github_user_id IN (SELECT gi.github_user_id FROM github_identities gi WHERE gi.user_id = sqlc.arg(uid));

-- name: DeleteUserBufferConnections :exec
DELETE FROM buffer_connections WHERE user_id = $1;

-- name: DeleteUserCaptainDrafts :exec
DELETE FROM captain_drafts WHERE launch_id IN (SELECT id FROM launches WHERE owner_user_id = $1);

-- name: MarkUserLaunchesDeleted :exec
UPDATE launches SET status = 'deleted' WHERE owner_user_id = $1;

-- name: DeleteUserIdentities :exec
DELETE FROM github_identities WHERE user_id = $1;

-- name: DeleteUserOptOuts :exec
DELETE FROM opt_outs o
WHERE o.github_user_id IN (SELECT gi.github_user_id FROM github_identities gi WHERE gi.user_id = sqlc.arg(uid))
   OR o.github_user_id IN (SELECT cm.github_user_id FROM crew_members cm WHERE cm.user_id = sqlc.arg(uid) AND cm.github_user_id IS NOT NULL);
