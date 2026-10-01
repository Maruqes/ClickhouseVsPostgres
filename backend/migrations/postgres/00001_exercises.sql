-- +goose Up
CREATE TABLE IF NOT EXISTS exercises (
    id BIGINT PRIMARY KEY,
    athlete_id BIGINT NOT NULL,
    exercise_id BIGINT NOT NULL,
    muscle_area TEXT NOT NULL,
    reps BIGINT NOT NULL,
    kg DECIMAL(12, 2) NOT NULL,
    country_code TEXT NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS exercises;
