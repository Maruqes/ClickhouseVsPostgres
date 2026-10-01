-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE IF NOT EXISTS seed_progress (
    version String,
    target Int64,
    completed Int64
) ENGINE = MergeTree
ORDER BY completed
SETTINGS fsync_after_insert = 1, fsync_part_directory = 1;

-- +goose Down
DROP TABLE IF EXISTS seed_progress;
