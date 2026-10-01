package database

import (
	"context"
	"time"

	"example.com/clickhouse-vs-postgres/backend/internal/analytics"
)

const where = " FROM exercises WHERE country_code = ? AND created_at >= ? AND created_at < ?"
const volumeSQL = "COALESCE(SUM(CAST(kg * reps * 100 AS BIGINT)), 0)"

func (s *store) Country(ctx context.Context, f analytics.Filter, detail bool) (analytics.Result, error) {
	args := []any{f.Country, f.From, f.To.AddDate(0, 0, 1)}
	start := time.Now()
	var centikg int64
	var totals analytics.Totals
	query := "SELECT " + volumeSQL + ", COUNT(DISTINCT athlete_id), COUNT(*), COALESCE(SUM(reps), 0)" + where + s.dialect.querySettings
	if err := s.db.QueryRowContext(ctx, s.dialect.bind(query), args...).Scan(&centikg, &totals.Athletes, &totals.Sets, &totals.Reps); err != nil {
		return analytics.Result{}, err
	}
	var muscles, days map[string]int64
	if detail {
		var err error
		muscles, err = s.group(ctx, "muscle_area", args)
		if err != nil {
			return analytics.Result{}, err
		}
		days, err = s.group(ctx, "CAST(created_at AS DATE)", args)
		if err != nil {
			return analytics.Result{}, err
		}
	}
	elapsed := time.Since(start)
	totals.Volume = analytics.Decimal(centikg)
	return analytics.Complete(f, totals, muscles, days, detail, elapsed), nil
}

func (s *store) group(ctx context.Context, field string, args []any) (map[string]int64, error) {
	query := "SELECT " + field + ", " + volumeSQL + where + " GROUP BY " + field + " ORDER BY " + field + s.dialect.querySettings
	rows, err := s.db.QueryContext(ctx, s.dialect.bind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]int64)
	for rows.Next() {
		var key any
		var volume int64
		if err := rows.Scan(&key, &volume); err != nil {
			return nil, err
		}
		switch v := key.(type) {
		case time.Time:
			result[v.UTC().Format(analytics.DateLayout)] = volume
		case string:
			result[v] = volume
		case []byte:
			result[string(v)] = volume
		}
	}
	return result, rows.Err()
}
