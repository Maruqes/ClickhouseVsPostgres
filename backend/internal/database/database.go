// Package database owns all database-specific connection and persistence code.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Store keeps database implementation details out of HTTP and business logic.
// Add shared domain operations here when application features are introduced.
type Store interface {
	Ping(context.Context) error
	Close() error
}

type store struct{ db *sql.DB }

func Open(driver, dsn string) (Store, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	var sqlDriver string
	switch driver {
	case "clickhouse":
		sqlDriver = "clickhouse"
	case "postgres":
		sqlDriver = "pgx"
	default:
		return nil, fmt.Errorf("unsupported DATABASE_DRIVER %q", driver)
	}
	db, err := sql.Open(sqlDriver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	return &store{db: db}, nil
}

func (s *store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *store) Close() error                   { return s.db.Close() }
