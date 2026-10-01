package database

import (
	"context"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"

	"example.com/clickhouse-vs-postgres/backend/migrations"
	"github.com/pressly/goose/v3"
)

// Runs unchanged against both real databases, exclusively in a disposable database.
func TestGooseMigrationContract(t *testing.T) {
	s := analyticsStore(t)
	ctx := context.Background()
	assertVersion := func(want int64) {
		t.Helper()
		version, err := s.migrations.GetDBVersion(ctx)
		if err != nil || version != want {
			t.Fatalf("migration version: got %d, want %d: %v", version, want, err)
		}
	}
	assertVersion(4)
	if _, err := s.migrations.DownTo(ctx, 0); err != nil {
		t.Fatal("rollback including foreign-key dependency order:", err)
	}
	assertVersion(0)
	for _, table := range []string{"exercises", "seed_progress", "demo_classes", "demo_reservations"} {
		var count int64
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err == nil {
			t.Fatal("rollback retained table", table)
		}
	}

	// Create an unversioned schema, as on existing pre-Goose application volumes.
	engine := os.Getenv("CONTRACT_DRIVER")
	files, err := fs.Sub(migrations.Files, engine)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := goose.NewProvider(goose.Dialect(engine), s.db, files,
		goose.WithDisableGlobalRegistry(true), goose.WithDisableVersioning(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Up(ctx); err != nil {
		t.Fatal(err)
	}
	assertVersion(0)
	if err := s.exec(ctx, seedQuery(s.dialect.source(1, 3), s.dialect.timestamp)); err != nil {
		t.Fatal(err)
	}
	if err := s.exec(ctx, "INSERT INTO seed_progress VALUES (?, ?, ?)", "exercise-v2", int64(3), int64(3)); err != nil {
		t.Fatal(err)
	}
	if err := s.migrateClasses(ctx); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.MigrateSchema(ctx); err != nil {
			t.Fatal("adopt or rerun existing schema:", err)
		}
		assertVersion(4)
		for table, want := range map[string]int64{"exercises": 3, "seed_progress": 1, "demo_classes": 1, "demo_reservations": 19} {
			var count int64
			err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count)
			if err != nil || count != want {
				t.Fatalf("migration changed %s rows: got %d, want %d: %v", table, count, want, err)
			}
		}
	}

	// A failed pending migration must not be recorded as applied.
	bad := fstest.MapFS{"00005_invalid.sql": &fstest.MapFile{Data: []byte(
		"-- +goose NO TRANSACTION\n-- +goose Up\nCREATE TABLE;\n-- +goose Down\nSELECT 1;\n")}}
	failed, err := goose.NewProvider(goose.Dialect(engine), s.db, bad, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := failed.Up(ctx); err == nil {
		t.Fatal("invalid SQL migration was accepted")
	}
	assertVersion(4)
}
