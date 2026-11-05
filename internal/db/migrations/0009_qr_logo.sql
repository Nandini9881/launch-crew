-- +goose Up
-- What sits in the middle of the QR code on the show slide: the Launch Crew
-- plane, nothing, or an image the captain supplied (stored as a launch asset).
ALTER TABLE launches ADD COLUMN qr_logo text NOT NULL DEFAULT 'plane';

-- +goose Down
ALTER TABLE launches DROP COLUMN qr_logo;
