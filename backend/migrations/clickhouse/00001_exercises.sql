-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE IF NOT EXISTS exercises (
    id Int64,
    athlete_id Int64,
    exercise_id Int64,
    muscle_area LowCardinality(String),
    reps Int64,
    kg Decimal(12, 2),
    country_code LowCardinality(String),
    created_at DateTime('UTC')
) ENGINE = MergeTree
ORDER BY (country_code, created_at, id)
SETTINGS fsync_after_insert = 1, fsync_part_directory = 1;

-- +goose Down
DROP TABLE IF EXISTS exercises;
