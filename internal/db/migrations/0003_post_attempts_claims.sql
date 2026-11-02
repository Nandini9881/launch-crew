-- +goose Up
ALTER TABLE post_attempts
    ADD COLUMN claimed_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN unclear boolean NOT NULL DEFAULT false,
    ADD COLUMN sent_text text,
    ADD COLUMN sent_at timestamptz,
    ADD COLUMN sent_due_at timestamptz,
    ADD COLUMN note text;

-- +goose Down
ALTER TABLE post_attempts
    DROP COLUMN note,
    DROP COLUMN sent_due_at,
    DROP COLUMN sent_at,
    DROP COLUMN sent_text,
    DROP COLUMN unclear,
    DROP COLUMN claimed_at;
