-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE IF NOT EXISTS demo_classes (
    id String,
    name String,
    capacity Int64
) ENGINE = MergeTree
ORDER BY id
SETTINGS fsync_after_insert = 1, fsync_part_directory = 1;

-- +goose Down
DROP TABLE IF EXISTS demo_classes;
