// Package database owns SQL, migration execution, and driver connections.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"time"

	"example.com/clickhouse-vs-postgres/backend/internal/analytics"
	"example.com/clickhouse-vs-postgres/backend/internal/classes"
	"example.com/clickhouse-vs-postgres/backend/migrations"
	"github.com/pressly/goose/v3"
)

type Store interface {
	classes.Storage
	Ping(context.Context) error
	MigrateSchema(context.Context) error
	Migrate(context.Context, int64) error
	Country(context.Context, analytics.Filter, bool) (analytics.Result, error)
	Close() error
}

type store struct {
	db         *sql.DB
	dialect    dialect
	migrations *goose.Provider
}

func Open(driver, dsn string) (Store, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	d, err := selectDialect(driver)
	if err != nil {
		return nil, err
	}
	schema, err := fs.Sub(migrations.Files, driver)
	if err != nil {
		return nil, err
	}
	db, err := d.open(dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(40)
	db.SetMaxIdleConns(40)
	db.SetConnMaxLifetime(5 * time.Minute)
	provider, err := goose.NewProvider(goose.Dialect(driver), db, schema, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("configure schema migrations: %w", err)
	}
	return &store{db: db, dialect: d, migrations: provider}, nil
}

func (s *store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *store) Close() error                   { return s.db.Close() }

func (s *store) exec(ctx context.Context, query string, args ...any) error {
	_, err := s.db.ExecContext(ctx, s.dialect.bind(query), args...)
	return err
}
