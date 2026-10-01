// Package database owns SQL, migration execution, and driver connections.
package database

import (
	"context"
	"database/sql"
	"example.com/clickhouse-vs-postgres/backend/migrations"
	"fmt"
	"strings"
	"time"

	"example.com/clickhouse-vs-postgres/backend/internal/analytics"
	"example.com/clickhouse-vs-postgres/backend/internal/classes"
)

type Store interface {
	classes.Storage
	Ping(context.Context) error
	Migrate(context.Context, int64) error
	Country(context.Context, analytics.Filter, bool) (analytics.Result, error)
	Close() error
}

type store struct {
	db      *sql.DB
	dialect dialect
}

func Open(driver, dsn string) (Store, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	d, err := selectDialect(driver)
	if err != nil {
		return nil, err
	}
	schema, err := migrations.Files.ReadFile(driver + ".sql")
	if err != nil {
		return nil, err
	}
	var schemaSQL strings.Builder
	for _, line := range strings.Split(string(schema), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			schemaSQL.WriteString(line + "\n")
		}
	}
	for _, statement := range strings.Split(schemaSQL.String(), ";") {
		if strings.TrimSpace(statement) != "" {
			d.schema = append(d.schema, statement)
		}
	}
	db, err := d.open(dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(40)
	db.SetMaxIdleConns(40)
	db.SetConnMaxLifetime(5 * time.Minute)
	return &store{db: db, dialect: d}, nil
}

func (s *store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *store) Close() error                   { return s.db.Close() }

func (s *store) exec(ctx context.Context, query string, args ...any) error {
	_, err := s.db.ExecContext(ctx, s.dialect.bind(query), args...)
	return err
}
