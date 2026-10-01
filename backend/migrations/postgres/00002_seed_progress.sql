-- +goose Up
CREATE TABLE IF NOT EXISTS seed_progress (
    version TEXT NOT NULL,
    target BIGINT NOT NULL,
    completed BIGINT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS seed_progress;
