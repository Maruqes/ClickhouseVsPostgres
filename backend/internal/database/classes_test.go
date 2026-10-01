package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"example.com/clickhouse-vs-postgres/backend/internal/classes"
)

func reservationStore(t *testing.T) *store {
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
	name := fmt.Sprintf("reservation_contract_%d", time.Now().UnixNano())
	if err := parent.exec(context.Background(), "CREATE DATABASE "+name); err != nil {
		parent.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := parent.exec(context.Background(), "DROP DATABASE "+name); err != nil {
			t.Error(err)
		}
		parent.Close()
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
	if err := s.Migrate(context.Background(), 31); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestReservationContract(t *testing.T) {
	s := reservationStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if !s.dialect.numbered {
		var async, resultCache, conditionCache bool
		err := s.db.QueryRowContext(ctx, "SELECT getSetting('async_insert'), getSetting('use_query_cache'), getSetting('use_query_condition_cache')").Scan(&async, &resultCache, &conditionCache)
		if err != nil || async || resultCache || conditionCache {
			t.Fatal("synchronous uncached settings", async, resultCache, conditionCache, err)
		}
	}
	template, err := s.DemoClass(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if template.Capacity != 20 || template.Name != "Spinning" || len(template.Reservations) != 19 {
		t.Fatalf("bad template: %+v", template)
	}
	for _, row := range template.Reservations {
		if row.ClassID != classes.TemplateID || row.AttemptID >= 0 || !row.CreatedAt.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
			t.Fatalf("bad initial reservation: %+v", row)
		}
	}
	var reports [2]classes.Report
	var errs [2]error
	var wg sync.WaitGroup
	for i := range reports {
		wg.Go(func() { reports[i], errs[i] = classes.Run(ctx, s) })
	}
	wg.Wait()
	if reports[0].FixtureID == reports[1].FixtureID {
		t.Fatal("concurrent runs share fixture")
	}
	for i, report := range reports {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if len(report.Attempts) != 20 || report.InitialBookings != 19 || report.Summary.Accepted+report.Summary.Rejected+report.Summary.Errors != 20 || report.Summary.InfrastructureErrors != 0 {
			t.Fatalf("bad accounting: %+v", report)
		}
		if s.dialect.numbered && (report.Summary.Accepted != 1 || report.Summary.Rejected != 19 || report.FinalBookings != 20 || report.Summary.Overbooked != 0) {
			t.Fatalf("protected operation overbooked: %+v", report)
		}
		stored, err := s.ReadClass(ctx, report.FixtureID)
		if err != nil || len(stored.Reservations) != report.FinalBookings || report.FinalBookings != 19+report.Summary.Accepted {
			t.Fatalf("stored totals differ: %+v %v", report, err)
		}
		for index, attempt := range report.Attempts {
			if attempt.Number != index+1 || attempt.DurationMS < 0 || (attempt.Outcome == "rejected" && attempt.Reason != "class_full") {
				t.Fatalf("bad attempt: %+v", attempt)
			}
		}
		for index := 1; index < len(stored.Reservations); index++ {
			if stored.Reservations[index].AttemptID <= stored.Reservations[index-1].AttemptID {
				t.Fatal("ordering or uniqueness failure")
			}
		}
	}
	repeat, err := classes.Run(ctx, s)
	if err != nil || repeat.InitialBookings != 19 || repeat.FixtureID == reports[0].FixtureID {
		t.Fatalf("repeat not isolated: %+v %v", repeat, err)
	}
	if err := s.Migrate(ctx, 31); err != nil {
		t.Fatal(err)
	}
	template, err = s.DemoClass(ctx)
	if err != nil || len(template.Reservations) != 19 {
		t.Fatal("rerun changed template", err)
	}
	for _, report := range reports {
		stored, err := s.ReadClass(ctx, report.FixtureID)
		if err != nil || len(stored.Reservations) != report.FinalBookings {
			t.Fatal("migration changed race data", err)
		}
	}
	var exercises int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM exercises").Scan(&exercises); err != nil || exercises != 31 {
		t.Fatal("analytics modified", exercises, err)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if err := s.Reserve(canceled, classes.Reservation{ID: "canceled", ClassID: repeat.FixtureID, AttemptID: 50, CreatedAt: time.Now()}); err == nil {
		t.Fatal("canceled reservation succeeded")
	}
	if _, err := s.ReadClass(ctx, "missing"); err == nil {
		t.Fatal("missing class accepted")
	}
}

// The gate exists only in tests, at the insert boundary after both availability
// reads. This exercises the exact adapter SQL without manufacturing UI outcomes.
type gatedInsert struct {
	*sql.DB
	waiting *sync.WaitGroup
	release <-chan struct{}
}

func (q gatedInsert) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	q.waiting.Done()
	select {
	case <-q.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return q.DB.ExecContext(ctx, query, args...)
}
func TestClickHouseControlledInterleaving(t *testing.T) {
	if os.Getenv("CONTRACT_DRIVER") != "clickhouse" {
		t.Skip("unsafe ClickHouse strategy only")
	}
	s := reservationStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	id := fmt.Sprintf("controlled-%d", time.Now().UnixNano())
	if _, err := s.CreateFixture(ctx, id); err != nil {
		t.Fatal(err)
	}
	var waiting, workers sync.WaitGroup
	waiting.Add(2)
	gate := make(chan struct{})
	q := gatedInsert{s.db, &waiting, gate}
	errs := make([]error, 2)
	for i := range errs {
		workers.Go(func() {
			errs[i] = s.reserve(ctx, q, classes.Reservation{ID: fmt.Sprintf("%s-%d", id, i), ClassID: id, AttemptID: int64(i + 1), CreatedAt: time.Now().UTC()}, false)
		})
	}
	reached := make(chan struct{})
	go func() { waiting.Wait(); close(reached) }()
	select {
	case <-reached:
	case <-ctx.Done():
		close(gate)
		workers.Wait()
		t.Fatal("contenders did not both pass availability")
	}
	close(gate)
	workers.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	stored, err := s.ReadClass(ctx, id)
	if err != nil || len(stored.Reservations) != 21 {
		t.Fatalf("unsafe race not demonstrated: %d %v", len(stored.Reservations), err)
	}
}
