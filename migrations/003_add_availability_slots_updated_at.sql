-- +goose Up

ALTER TABLE availability_slots
ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();


-- +goose Down

ALTER TABLE availability_slots
DROP COLUMN updated_at;