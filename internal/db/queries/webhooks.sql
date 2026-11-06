-- name: InsertWebhookDelivery :one
INSERT INTO webhook_deliveries (delivery_id, event, payload)
VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING
RETURNING delivery_id;

-- name: GetWebhookDelivery :one
SELECT * FROM webhook_deliveries WHERE delivery_id = $1;

-- name: DeleteWebhookDelivery :exec
DELETE FROM webhook_deliveries WHERE delivery_id = $1;

-- name: DeleteWebhookDeliveriesBefore :execrows
DELETE FROM webhook_deliveries WHERE received_at < $1;
