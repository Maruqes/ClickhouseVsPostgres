package database

import (
	"database/sql"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgconn/ctxwatch"
	"github.com/jackc/pgx/v5/stdlib"
	"time"
)

func selectDialect(engine string) (dialect, error) {
	switch engine {
	case "clickhouse":
		return dialect{
			driver:        "clickhouse",
			series:        "SELECT number + %d AS id FROM numbers(%d)",
			timestamp:     "toDateTime(1672531200 + MOD(id * 7919, 94694400), 'UTC')",
			deleteTail:    "ALTER TABLE exercises DELETE WHERE id > ? SETTINGS mutations_sync=2",
			querySettings: " SETTINGS use_query_cache=0, use_query_condition_cache=0, count_distinct_implementation='uniqExact', max_threads=2",
		}, nil
	case "postgres":
		return dialect{
			driver: "pgx", numbered: true,
			series:     "SELECT generate_series(CAST(%d AS BIGINT), CAST(%d AS BIGINT)) AS id",
			timestamp:  "TIMESTAMP '2023-01-01' + MOD(id * 7919, 94694400) * INTERVAL '1 second'",
			deleteTail: "DELETE FROM exercises WHERE id > ?",
			prepare:    []string{"DROP INDEX IF EXISTS exercises_country_date"},
			finish:     []string{"CREATE INDEX IF NOT EXISTS exercises_country_date ON exercises (country_code, created_at)", "ANALYZE exercises"},
		}, nil
	default:
		return dialect{}, fmt.Errorf("unsupported DATABASE_DRIVER %q", engine)
	}
}

// PG cancellation must reach the server; simply closing the client socket
// can leave a long query running until it finishes. The deadline is a fallback.
func (d dialect) open(dsn string) (*sql.DB, error) {
	if !d.numbered {
		options, err := clickhouse.ParseDSN(dsn)
		if err != nil {
			return nil, err
		}
		if options.Settings == nil {
			options.Settings = clickhouse.Settings{}
		}
		// Pin write acknowledgement and uncached reads instead of inheriting
		// defaults that may change between ClickHouse versions.
		options.Settings["async_insert"] = 0
		options.Settings["use_query_cache"] = 0
		options.Settings["use_query_condition_cache"] = 0
		return clickhouse.OpenDB(options), nil
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	config.BuildContextWatcherHandler = func(conn *pgconn.PgConn) ctxwatch.Handler {
		return &pgconn.CancelRequestContextWatcherHandler{Conn: conn, CancelRequestDelay: 0, DeadlineDelay: time.Second}
	}
	return stdlib.OpenDB(*config), nil
}
