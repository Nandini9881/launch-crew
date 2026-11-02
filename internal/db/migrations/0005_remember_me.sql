-- +goose Up
ALTER TABLE users ADD COLUMN remember_me boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE users DROP COLUMN remember_me;
