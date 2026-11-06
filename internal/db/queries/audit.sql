-- name: InsertBufferCall :exec
INSERT INTO buffer_calls (launch_id, connection_id, op, status, request_id, rate_remaining)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: CountBufferCallsSince :one
SELECT count(*) FROM buffer_calls WHERE connection_id = $1 AND at >= $2;

-- name: UpsertOpenRate :one
INSERT INTO open_rate (launch_id, ip, minute) VALUES ($1, $2, $3)
ON CONFLICT (launch_id, ip, minute) DO UPDATE SET n = open_rate.n + 1
RETURNING n;

-- name: DeleteBufferCallsBefore :execrows
DELETE FROM buffer_calls WHERE at < $1;

-- name: DeleteOpenRateBefore :execrows
DELETE FROM open_rate WHERE minute < $1;
