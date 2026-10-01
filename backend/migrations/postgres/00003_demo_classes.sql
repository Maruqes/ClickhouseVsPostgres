-- +goose Up
CREATE TABLE IF NOT EXISTS demo_classes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    capacity BIGINT NOT NULL CHECK (capacity > 0)
);

-- +goose Down
DROP TABLE IF EXISTS demo_classes;
