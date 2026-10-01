-- Schema v1: raw rows only; merges preserve every completed set.
CREATE TABLE IF NOT EXISTS exercises (
    id Int64, athlete_id Int64, exercise_id Int64,
    muscle_area LowCardinality(String), reps Int64, kg Decimal(12,2),
    country_code LowCardinality(String), created_at DateTime('UTC')
) ENGINE=MergeTree ORDER BY (country_code, created_at, id)
SETTINGS fsync_after_insert=1, fsync_part_directory=1;
CREATE TABLE IF NOT EXISTS seed_progress (
    version String, target Int64, completed Int64
) ENGINE=MergeTree ORDER BY completed
SETTINGS fsync_after_insert=1, fsync_part_directory=1;
-- Applies the durability settings to existing v1 tables as well.
ALTER TABLE exercises MODIFY SETTING fsync_after_insert=1, fsync_part_directory=1;
ALTER TABLE seed_progress MODIFY SETTING fsync_after_insert=1, fsync_part_directory=1;
-- Reservation demo v1: plain raw rows, with no deduplication or capacity rule.
CREATE TABLE IF NOT EXISTS demo_classes (
    id String, name String, capacity Int64
) ENGINE=MergeTree ORDER BY id
SETTINGS fsync_after_insert=1, fsync_part_directory=1;
CREATE TABLE IF NOT EXISTS demo_reservations (
    id String, class_id String, attempt_id Int64, created_at DateTime64(3, 'UTC')
) ENGINE=MergeTree ORDER BY (class_id, attempt_id, id)
SETTINGS fsync_after_insert=1, fsync_part_directory=1;
