-- +goose Up
-- Captain-uploaded images sit beside the cards in launch_assets as kind
-- 'image', ordered by position (NULL for cards and the hero). include_card
-- says whether a launch's posts lead with the card slot.
ALTER TABLE launch_assets ADD COLUMN position smallint;
ALTER TABLE launches ADD COLUMN include_card boolean NOT NULL DEFAULT true;

-- +goose Down
ALTER TABLE launches DROP COLUMN include_card;
ALTER TABLE launch_assets DROP COLUMN position;
