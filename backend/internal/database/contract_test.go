package database

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"example.com/clickhouse-vs-postgres/backend/internal/analytics"
)

// This same suite runs against both real engines using disposable test databases.
// It never writes the application's dataset or deletes persistent volumes.
func analyticsStore(t *testing.T) *store {
	t.Helper()
	engine, dsn := os.Getenv("CONTRACT_DRIVER"), os.Getenv("CONTRACT_URL")
	if dsn == "" {
		t.Skip("run scripts/test-databases.sh for real database coverage")
	}
	parentStore, err := Open(engine, dsn)
	if err != nil {
		t.Fatal(err)
	}
	parent := parentStore.(*store)
	t.Cleanup(func() { parent.Close() })
	ctx := context.Background()
	name := fmt.Sprintf("exercise_contract_%d", time.Now().UnixNano())
	if err := parent.exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		drop := "DROP DATABASE " + name
		if engine == "postgres" {
			drop += " WITH (FORCE)"
		}
		if err := parent.exec(ctx, drop); err != nil {
			t.Error(err)
		}
	})
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/" + name
	child, err := Open(engine, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	s := child.(*store)
	t.Cleanup(func() { s.Close() })
	for _, query := range s.dialect.schema {
		if err := s.exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestDatabaseContract(t *testing.T) {
	s := analyticsStore(t)
	ctx := context.Background()
	const target int64 = 10003
	// Simulate an interrupted first batch: checkpoint still zero, 113 rows exist.
	if err := s.exec(ctx, "INSERT INTO seed_progress VALUES (?, ?, ?)", analytics.SeedVersion, target, int64(0)); err != nil {
		t.Fatal(err)
	}
	if err := s.exec(ctx, seedQuery(s.dialect.source(1, 113), s.dialect.timestamp)); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx, target); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx, target); err != nil {
		t.Fatal(err)
	}
	var count, distinct int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*), COUNT(DISTINCT id) FROM exercises").Scan(&count, &distinct); err != nil {
		t.Fatal(err)
	}
	if count != target || distinct != target {
		t.Fatalf("restart duplicated/missed sets: %d, %d", count, distinct)
	}
	if err := s.Migrate(ctx, target+1); err == nil {
		t.Fatal("seed configuration change was accepted")
	}
	// Independent integer reference covers exact fractional volume, seven categories,
	// UTC boundaries, exact distinct athletes and empty days/countries.
	for _, filter := range []analytics.Filter{
		{Country: "US", From: analytics.FirstDay, To: analytics.LastDay},
		{Country: "PT", From: analytics.FirstDay, To: analytics.FirstDay},
		{Country: "DE", From: time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC), To: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)},
		{Country: "ZZ", From: analytics.LastDay, To: analytics.LastDay},
		{Country: "ES", From: analytics.FirstDay, To: analytics.LastDay},
		{Country: "CN", From: analytics.FirstDay, To: analytics.LastDay},
		{Country: "KE", From: analytics.FirstDay, To: analytics.LastDay},
		{Country: "MX", From: analytics.FirstDay, To: analytics.LastDay},
		{Country: "NZ", From: analytics.FirstDay, To: analytics.LastDay},
	} {
		got, err := s.Country(ctx, filter, true)
		if err != nil {
			t.Fatal(err)
		}
		expected := reference(target, filter)
		got.QueryMS = 0
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("contract mismatch for %s %s: got totals %+v, expected %+v", filter.Country, filter.From, got.Totals, expected.Totals)
		}
		summary, err := s.Country(ctx, filter, false)
		if err != nil || summary.Totals != expected.Totals || len(summary.Days) != 0 || len(summary.Muscles) != 0 {
			t.Fatal("summary/detail mismatch", err)
		}
	}
	// Insert known raw sets to make decimal, repeated-athlete and midnight cases
	// independent from the generator's distribution.
	for _, record := range []struct {
		id, athlete, reps int64
		kg                string
		date              time.Time
	}{
		{target + 1, 1, 8, "60.00", analytics.FirstDay}, {target + 2, 1, 3, "0.25", analytics.FirstDay.Add(23*time.Hour + 59*time.Minute + 59*time.Second)},
		{target + 3, 2, 1, "1.25", analytics.FirstDay.AddDate(0, 0, 1)},
	} {
		if err := s.exec(ctx, "INSERT INTO exercises VALUES (?, ?, ?, ?, ?, CAST(? AS DECIMAL(12,2)), ?, ?)", record.id, record.athlete, int64(1), "chest", record.reps, record.kg, "ZZ", record.date); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Country(ctx, analytics.Filter{Country: "ZZ", From: analytics.FirstDay, To: analytics.FirstDay}, true)
	if err != nil || got.Totals.Volume != "480.75" || got.Totals.Athletes != 1 || got.Totals.Sets != 2 || got.Totals.Reps != 11 {
		t.Fatalf("midnight/decimal contract: %+v %v", got, err)
	}
	// An actual running driver query must stop promptly when its context is canceled.
	canceled, cancel := context.WithCancel(ctx)
	timer := time.AfterFunc(50*time.Millisecond, cancel)
	start := time.Now()
	var total any
	err = s.db.QueryRowContext(canceled, "SELECT SUM(id) FROM ("+s.dialect.source(1, 1_000_000_000)+") AS cancellation_contract").Scan(&total)
	timer.Stop()
	cancel()
	if err == nil || time.Since(start) > 2*time.Second {
		t.Fatalf("driver cancellation: %v after %s", err, time.Since(start))
	}
	activeQuery := "SELECT COUNT(*) FROM system.processes WHERE query LIKE '%cancellation_contract%' AND query NOT LIKE '%system.processes%'"
	if s.dialect.numbered {
		activeQuery = "SELECT COUNT(*) FROM pg_stat_activity WHERE pid <> pg_backend_pid() AND state='active' AND query LIKE '%cancellation_contract%'"
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		var active int
		if err := s.db.QueryRowContext(ctx, activeQuery).Scan(&active); err != nil {
			t.Fatal(err)
		}
		if active == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("canceled work still running on database server")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := s.Ping(ctx); err != nil {
		t.Fatal("connection unusable after cancellation", err)
	}
}

func reference(target int64, filter analytics.Filter) analytics.Result {
	totals := analytics.Totals{}
	athletes := map[int64]bool{}
	muscles, days := map[string]int64{}, map[string]int64{}
	var volume int64
	countries := []struct {
		limit int64
		code  string
	}{{30, "US"}, {45, "BR"}, {57, "DE"}, {67, "IN"}, {75, "GB"}, {82, "FR"}, {88, "JP"}, {93, "AU"}, {97, "CA"}, {99, "PT"}, {100, "ZA"}}
	newCountries := strings.Fields("ES IT NL BE CH AT SE NO DK FI PL CZ RO GR TR UA RU CN KR ID TH VN MY PH PK BD SA AE IL EG MA NG KE MX AR CL CO PE NZ")
	for i, code := range newCountries {
		countries = append(countries, struct {
			limit int64
			code  string
		}{int64(101 + i), code})
	}
	for id := int64(1); id <= target; id++ {
		athlete := (id-1)%100000 + 1
		nationality := ((athlete - 1) * 37) % 139
		var country string
		for _, entry := range countries {
			if nationality < entry.limit {
				country = entry.code
				break
			}
		}
		date := analytics.FirstDay.Add(time.Duration((id*7919)%94694400) * time.Second)
		if country != filter.Country || date.Before(filter.From) || !date.Before(filter.To.AddDate(0, 0, 1)) {
			continue
		}
		reps := (id*17)%16 + 5
		centikg := ((id*29)%781 + 20) * 25 * reps
		totals.Sets++
		totals.Reps += reps
		volume += centikg
		athletes[athlete] = true
		muscles[analytics.Muscles[(id*13)%7]] += centikg
		days[date.Format(analytics.DateLayout)] += centikg
	}
	totals.Athletes = int64(len(athletes))
	totals.Volume = analytics.Decimal(volume)
	return analytics.Complete(filter, totals, muscles, days, true, 0)
}

func TestSeedUpgrade(t *testing.T) {
	for _, checkpoint := range []int64{-1, 0, 113} {
		t.Run(fmt.Sprintf("checkpoint=%d", checkpoint), func(t *testing.T) {
			s := analyticsStore(t)
			ctx := context.Background()
			const target int64 = 10003
			legacyCountry := `CASE
 WHEN MOD(MOD(id - 1, 100000) * 37, 100) < 30 THEN 'US'
 WHEN MOD(MOD(id - 1, 100000) * 37, 100) < 45 THEN 'BR'
 WHEN MOD(MOD(id - 1, 100000) * 37, 100) < 57 THEN 'DE'
 WHEN MOD(MOD(id - 1, 100000) * 37, 100) < 67 THEN 'IN'
 WHEN MOD(MOD(id - 1, 100000) * 37, 100) < 75 THEN 'GB'
 WHEN MOD(MOD(id - 1, 100000) * 37, 100) < 82 THEN 'FR'
 WHEN MOD(MOD(id - 1, 100000) * 37, 100) < 88 THEN 'JP'
 WHEN MOD(MOD(id - 1, 100000) * 37, 100) < 93 THEN 'AU'
 WHEN MOD(MOD(id - 1, 100000) * 37, 100) < 97 THEN 'CA'
 WHEN MOD(MOD(id - 1, 100000) * 37, 100) < 99 THEN 'PT' ELSE 'ZA' END`
			if err := s.exec(ctx, "INSERT INTO seed_progress VALUES (?, ?, ?)", "exercise-v1", target, target); err != nil {
				t.Fatal(err)
			}
			if err := s.exec(ctx, strings.Replace(seedQuery(s.dialect.source(1, target), s.dialect.timestamp), seedCountrySQL(), legacyCountry, 1)); err != nil {
				t.Fatal(err)
			}
			if err := s.migrateClasses(ctx); err != nil {
				t.Fatal(err)
			}
			before, err := s.DemoClass(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if checkpoint >= 0 {
				// v1's completed checkpoint is larger than v2's checkpoint. Uncertain
				// upgrade inserts exist, but only v2's checkpoint may control recovery.
				if err := s.exec(ctx, "INSERT INTO seed_progress VALUES (?, ?, ?)", analytics.SeedVersion, target, checkpoint); err != nil {
					t.Fatal(err)
				}
				if err := s.exec(ctx, s.dialect.deleteTail, int64(0)); err != nil {
					t.Fatal(err)
				}
				if err := s.exec(ctx, seedQuery(s.dialect.source(1, checkpoint+113), s.dialect.timestamp)); err != nil {
					t.Fatal(err)
				}
			}
			for range 2 {
				if err := s.Migrate(ctx, target); err != nil {
					t.Fatal(err)
				}
			}
			var count, distinct, countries int64
			if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*), COUNT(DISTINCT id), COUNT(DISTINCT country_code) FROM exercises").Scan(&count, &distinct, &countries); err != nil {
				t.Fatal(err)
			}
			if count != target || distinct != target || countries != 50 {
				t.Fatalf("upgrade count/distinct/countries: %d/%d/%d", count, distinct, countries)
			}
			for _, country := range seedCountries {
				filter := analytics.Filter{Country: country.code, From: analytics.FirstDay, To: analytics.LastDay}
				got, err := s.Country(ctx, filter, false)
				if err != nil || got.Totals != reference(target, filter).Totals {
					t.Fatalf("upgrade country %s: %+v %v", country.code, got, err)
				}
			}
			after, err := s.DemoClass(ctx)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("upgrade changed reservations", err)
			}
			if err := s.exec(ctx, "INSERT INTO seed_progress VALUES (?, ?, ?)", "exercise-future", target, target); err != nil {
				t.Fatal(err)
			}
			if err := s.Migrate(ctx, target); err == nil {
				t.Fatal("unknown seed version was accepted")
			}
		})
	}
}
