-- Schema v1: equivalent logical records with native country/date indexing.
CREATE TABLE IF NOT EXISTS exercises (
    id BIGINT PRIMARY KEY, athlete_id BIGINT NOT NULL, exercise_id BIGINT NOT NULL,
    muscle_area TEXT NOT NULL, reps BIGINT NOT NULL, kg DECIMAL(12,2) NOT NULL,
    country_code TEXT NOT NULL, created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL
);
CREATE TABLE IF NOT EXISTS seed_progress (
    version TEXT NOT NULL, target BIGINT NOT NULL, completed BIGINT NOT NULL
);
-- Reservation demo v1: primary keys plus per-class attempt uniqueness.
CREATE TABLE IF NOT EXISTS demo_classes (
    id TEXT PRIMARY KEY, name TEXT NOT NULL, capacity BIGINT NOT NULL CHECK (capacity > 0)
);
CREATE TABLE IF NOT EXISTS demo_reservations (
    id TEXT PRIMARY KEY, class_id TEXT NOT NULL REFERENCES demo_classes(id),
    attempt_id BIGINT NOT NULL, created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (class_id, attempt_id)
);
