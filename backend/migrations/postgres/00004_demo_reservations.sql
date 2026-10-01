-- +goose Up
CREATE TABLE IF NOT EXISTS demo_reservations (
    id TEXT PRIMARY KEY,
    class_id TEXT NOT NULL REFERENCES demo_classes(id),
    attempt_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,

    UNIQUE (class_id, attempt_id)
);

-- +goose Down
DROP TABLE IF EXISTS demo_reservations;
