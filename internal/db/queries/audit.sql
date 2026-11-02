-- name: InsertBufferCall :exec
INSERT INTO buffer_calls (launch_id, connection_id, op, status, request_id, rate_remaining)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: CountBufferCallsSince :one
SELECT count(*) FROM buffer_calls WHERE connection_id = $1 AND at >= $2;

-- name: UpsertOpenRate :one
INSERT INTO open_rate (launch_id, ip, minute) VALUES ($1, $2, $3)
ON CONFLICT (launch_id, ip, minute) DO UPDATE SET n = open_rate.n + 1
RETURNING n;
