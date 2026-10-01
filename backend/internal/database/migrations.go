package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"example.com/clickhouse-vs-postgres/backend/internal/analytics"
)

// MigrateSchema applies only versioned Goose SQL, without seeding application data.
// There must be one migration writer per database; make migrate stops the servers.
func (s *store) MigrateSchema(ctx context.Context) error {
	if _, err := s.migrations.Up(ctx); err != nil {
		return fmt.Errorf("schema migration: %w", err)
	}
	return nil
}

// Migrate is the sole analytics seed writer. Each stack has one backend; readiness remains off
// until schema, deterministic seed and native indexes are complete.
// A durable checkpoint follows each synchronous batch insert. After an interrupted
// insert/checkpoint pair, remove only the uncheckpointed tail before replaying.
func (s *store) Migrate(ctx context.Context, target int64) error {
	if err := s.MigrateSchema(ctx); err != nil {
		return err
	}
	completed, err := s.seedCheckpoint(ctx, target)
	if err != nil {
		return err
	}
	if completed < target {
		for _, query := range s.dialect.prepare {
			if err := s.exec(ctx, query); err != nil {
				return fmt.Errorf("seed preparation: %w", err)
			}
		}
		// At checkpoint zero no exercise rows are durable seed progress. Truncate
		// that table rather than leaving v1's deleted rows and index pages behind.
		query := s.dialect.deleteTail
		args := []any{completed}
		if completed == 0 {
			query, args = "TRUNCATE TABLE exercises", nil
		}
		if err := s.exec(ctx, query, args...); err != nil {
			return fmt.Errorf("seed recovery: %w", err)
		}
	}
	for completed < target {
		size := min(analytics.BatchSize, target-completed)
		query := seedQuery(s.dialect.source(completed+1, size), s.dialect.timestamp)
		if err := s.exec(ctx, query); err != nil {
			return fmt.Errorf("seed batch after %d: %w", completed, err)
		}
		completed += size
		if err := s.exec(ctx, "INSERT INTO seed_progress (version, target, completed) VALUES (?, ?, ?)", analytics.SeedVersion, target, completed); err != nil {
			return err
		}
		slog.Info("seed migration", "completed", completed, "target", target)
	}
	for _, query := range s.dialect.finish {
		if err := s.exec(ctx, query); err != nil {
			return err
		}
	}
	return s.migrateClasses(ctx)
}

// seedCheckpoint permits the explicit v1 -> v2 nationality upgrade at the same
// row count. Checkpoints are version-scoped: an old completed seed must never
// hide an interrupted upgrade. The zero checkpoint is durable before replacement
// starts, so recovery regenerates only exercises and retains reservation fixtures.
func (s *store) seedCheckpoint(ctx context.Context, target int64) (int64, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT version, target FROM seed_progress")
	if err != nil {
		return 0, err
	}
	legacy := false
	for rows.Next() {
		var version string
		var previous int64
		if err := rows.Scan(&version, &previous); err != nil {
			rows.Close()
			return 0, err
		}
		if previous != target {
			rows.Close()
			return 0, fmt.Errorf("existing seed configuration differs: restore SEED_ROWS=%d; use separate volumes for a different dataset", previous)
		}
		if version != analytics.SeedVersion && version != "exercise-v1" {
			rows.Close()
			return 0, fmt.Errorf("unsupported existing seed version %q", version)
		}
		legacy = legacy || version == "exercise-v1"
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	var completed int64
	query := s.dialect.bind("SELECT completed FROM seed_progress WHERE version = ? ORDER BY completed DESC LIMIT 1")
	err = s.db.QueryRowContext(ctx, query, analytics.SeedVersion).Scan(&completed)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		if err := s.exec(ctx, "INSERT INTO seed_progress (version, target, completed) VALUES (?, ?, ?)", analytics.SeedVersion, target, int64(0)); err != nil {
			return 0, err
		}
		if legacy {
			slog.Info("upgrading exercise seed", "from", "exercise-v1", "to", analytics.SeedVersion, "target", target)
		}
	}
	if completed < 0 || completed > target {
		return 0, fmt.Errorf("invalid seed checkpoint %d for target %d", completed, target)
	}
	return completed, nil
}

// All arithmetic uses positive integers. Each version fixes the seed across
// engines, batches, restarts and row counts. Nationality is fixed per athlete;
// all non-nationality fields retain their v1 formulas.
func seedQuery(source, timestamp string) string {
	return fmt.Sprintf(`INSERT INTO exercises (id, athlete_id, exercise_id, muscle_area, reps, kg, country_code, created_at)
 SELECT id, MOD(id - 1, 100000) + 1, MOD(id * 13, 21) + 1,
 CASE MOD(id * 13, 7) WHEN 0 THEN 'back' WHEN 1 THEN 'shoulder' WHEN 2 THEN 'legs' WHEN 3 THEN 'chest' WHEN 4 THEN 'tricep' WHEN 5 THEN 'bicep' ELSE 'forearm' END,
 MOD(id * 17, 16) + 5, CAST((MOD(id * 29, 781) + 20) * 0.25 AS DECIMAL(12,2)),
 %s, %s FROM (%s) AS generated`, seedCountrySQL(), timestamp, source)
}
