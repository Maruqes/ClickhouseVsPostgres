-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE IF NOT EXISTS demo_reservations (
    id String,
    class_id String,
    attempt_id Int64,
    created_at DateTime64(3, 'UTC')
) ENGINE = MergeTree
ORDER BY (class_id, attempt_id, id)
SETTINGS fsync_after_insert = 1, fsync_part_directory = 1;

-- +goose Down
DROP TABLE IF EXISTS demo_reservations;
