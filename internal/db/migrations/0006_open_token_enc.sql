-- +goose Up
-- Keyring-encrypted copies of the open and invite tokens, so the captain can
-- copy the links again; the hashes stay the lookup keys.
ALTER TABLE launches
    ADD COLUMN open_token_enc bytea,
    ADD COLUMN release_id bigint;
ALTER TABLE crew_members
    ADD COLUMN invite_token_enc bytea,
    ADD COLUMN comment_failed bool NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE crew_members DROP COLUMN comment_failed, DROP COLUMN invite_token_enc;
ALTER TABLE launches DROP COLUMN release_id, DROP COLUMN open_token_enc;
