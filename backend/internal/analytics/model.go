// Package analytics defines the shared country analytics contract.
package analytics

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const DateLayout = "2006-01-02"
const SeedVersion = "exercise-v2"
const DefaultRows int64 = 10_000_000
const BatchSize int64 = 250_000

var FirstDay = time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
var LastDay = time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
var Muscles = []string{"back", "shoulder", "legs", "chest", "tricep", "bicep", "forearm"}

// Metadata describes configured seed bounds without aggregating exercise rows.
type Metadata struct {
	Version string   `json:"version"`
	Rows    int64    `json:"rows"`
	From    string   `json:"from"`
	To      string   `json:"to"`
	Muscles []string `json:"muscles"`
}

func Dataset(rows int64) Metadata {
	return Metadata{SeedVersion, rows, FirstDay.Format(DateLayout), LastDay.Format(DateLayout), Muscles}
}

func ConfiguredRows() (int64, error) {
	value := os.Getenv("SEED_ROWS")
	if value == "" {
		return DefaultRows, nil
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n < 1 || n > 100_000_000 {
		return 0, fmt.Errorf("SEED_ROWS must be between 1 and 100000000")
	}
	return n, nil
}

type Filter struct {
	Country string
	From    time.Time
	To      time.Time // inclusive UTC day
}

type Totals struct {
	Volume   string `json:"volume_kg"` // exact decimal, never a binary float
	Athletes int64  `json:"athletes"`
	Sets     int64  `json:"sets"`
	Reps     int64  `json:"reps"`
}

type Volume struct {
	Key    string `json:"key"`
	Volume string `json:"volume_kg"`
}

type Result struct {
	Country string   `json:"country"`
	From    string   `json:"from"`
	To      string   `json:"to"`
	Totals  Totals   `json:"totals"`
	Muscles []Volume `json:"muscles,omitempty"`
	Days    []Volume `json:"days,omitempty"`
	QueryMS float64  `json:"query_ms"`
}

func Decimal(centikg int64) string {
	return fmt.Sprintf("%d.%02d", centikg/100, centikg%100)
}

// Complete supplies all seven categories and every UTC day, including zeros.
// It runs after the database timer stops; it performs no aggregation of raw sets.
func Complete(f Filter, totals Totals, muscles, days map[string]int64, detail bool, elapsed time.Duration) Result {
	result := Result{Country: f.Country, From: f.From.Format(DateLayout), To: f.To.Format(DateLayout), Totals: totals, QueryMS: float64(elapsed.Microseconds()) / 1000}
	if !detail {
		return result
	}
	for _, key := range Muscles {
		result.Muscles = append(result.Muscles, Volume{key, Decimal(muscles[key])})
	}
	for day := f.From; !day.After(f.To); day = day.AddDate(0, 0, 1) {
		key := day.Format(DateLayout)
		result.Days = append(result.Days, Volume{key, Decimal(days[key])})
	}
	return result
}
