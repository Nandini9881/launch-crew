-- +goose Up
-- Every PR a contributor has in the release, newest first, as
-- [{"number": 12, "title": "..."}]. NULL means a single PR, held in
-- pr_number and pr_title (which stay the most recent PR).
ALTER TABLE crew_members ADD COLUMN prs jsonb;

-- +goose Down
ALTER TABLE crew_members DROP COLUMN prs;
