-- +goose Up
-- Why a captain post ended as a Buffer draft or failed.
ALTER TABLE captain_drafts ADD COLUMN note text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE captain_drafts DROP COLUMN note;
