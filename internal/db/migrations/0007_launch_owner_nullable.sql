-- +goose Up
-- Deleting a user must not cascade into launches: other people's crew rows and
-- invite links hang off them. The owner is nulled instead.
ALTER TABLE launches ALTER COLUMN owner_user_id DROP NOT NULL;
ALTER TABLE launches DROP CONSTRAINT launches_owner_user_id_fkey;
ALTER TABLE launches ADD CONSTRAINT launches_owner_user_id_fkey
    FOREIGN KEY (owner_user_id) REFERENCES users (id) ON DELETE SET NULL;

-- +goose Down
DELETE FROM launches WHERE owner_user_id IS NULL;
ALTER TABLE launches DROP CONSTRAINT launches_owner_user_id_fkey;
ALTER TABLE launches ADD CONSTRAINT launches_owner_user_id_fkey
    FOREIGN KEY (owner_user_id) REFERENCES users (id) ON DELETE CASCADE;
ALTER TABLE launches ALTER COLUMN owner_user_id SET NOT NULL;
