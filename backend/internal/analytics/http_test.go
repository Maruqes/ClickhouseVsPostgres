package analytics

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeReader struct {
	calls int
	err   error
}

func (f *fakeReader) Ping(context.Context) error { return f.err }
func (f *fakeReader) Country(ctx context.Context, filter Filter, detail bool) (Result, error) {
	f.calls++
	if f.err != nil {
		return Result{}, f.err
	}
	return Complete(filter, Totals{Volume: "0.00"}, nil, nil, detail, time.Millisecond), nil
}

func TestValidation(t *testing.T) {
	for _, query := range []string{
		"", "country=us&from=2023-01-01&to=2023-01-02", "country=US&from=2024-02-30&to=2024-03-01",
		"country=US&from=2024-02-01&to=2024-01-01", "country=US&from=2022-12-31&to=2023-01-01",
		"country=US&from=2025-12-31&to=2026-01-01", "country=US&country=GB&from=2023-01-01&to=2023-01-02",
		"country=US&from=2023-01-01&to=2023-01-02&extra=1",
	} {
		if _, err := ParseFilter(httptest.NewRequest("GET", "/?"+query, nil)); err == nil {
			t.Errorf("accepted invalid query %q", query)
		}
	}
}

func TestLazyReadinessAndErrors(t *testing.T) {
	reader := &fakeReader{}
	var ready atomic.Bool
	handler := Handler(reader, Dataset(100), &ready)
	run := func(path string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("missing no-store")
		}
		return response
	}
	if run("/livez").Code != 200 || run("/api/health").Code != 503 || run("/api/dataset").Code != 503 {
		t.Fatal("readiness before migration")
	}
	ready.Store(true)
	if run("/api/dataset").Code != 200 || reader.calls != 0 {
		t.Fatal("metadata performs analytics")
	}
	path := "/api/country/detail?country=NZ&from=2024-02-28&to=2024-03-01"
	response := run(path)
	if response.Code != 200 || !strings.Contains(response.Body.String(), "2024-02-29") {
		t.Fatal(response.Body.String())
	}
	reader.err = errors.New("secret driver connection details")
	response = run(path)
	if response.Code != 503 || strings.Contains(response.Body.String(), "secret") {
		t.Fatal("driver error leaked")
	}
	reader.err = context.DeadlineExceeded
	if run(path).Code != 504 {
		t.Fatal("deadline status")
	}
	if run("/api/country/summary?country=US").Code != 400 {
		t.Fatal("validation status")
	}
}

func TestExactFormattingAndEmptyDays(t *testing.T) {
	if Decimal(48025) != "480.25" || Decimal(1) != "0.01" || Decimal(0) != "0.00" {
		t.Fatal("decimal formatting")
	}
	filter := Filter{"PT", time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC), time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)}
	result := Complete(filter, Totals{Volume: "480.25"}, map[string]int64{"chest": 48025}, map[string]int64{"2024-02-29": 48025}, true, time.Millisecond)
	if len(result.Days) != 3 || result.Days[1].Key != "2024-02-29" || result.Days[0].Volume != "0.00" || result.Muscles[3].Volume != "480.25" {
		t.Fatal(result)
	}
}
