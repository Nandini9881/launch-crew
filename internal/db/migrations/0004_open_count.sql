-- +goose Up
ALTER TABLE launches ADD COLUMN open_count int NOT NULL DEFAULT 0;
ALTER TABLE crew_members ADD COLUMN voiced_at timestamptz;
ALTER TABLE crew_drafts ADD COLUMN from_model bool NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE crew_drafts DROP COLUMN from_model;
ALTER TABLE crew_members DROP COLUMN voiced_at;
ALTER TABLE launches DROP COLUMN open_count;
