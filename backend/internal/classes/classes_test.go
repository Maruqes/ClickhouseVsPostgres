package classes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type fakeStore struct {
	mu           sync.Mutex
	fixture      Class
	reserveError error
	createError  error
	readError    error
	persist      bool
	corrupt      bool
}

func (s *fakeStore) DemoClass(context.Context) (Class, error) { return Class{ID: TemplateID}, nil }
func (s *fakeStore) CreateFixture(_ context.Context, id string) (Class, error) {
	s.fixture = Class{ID: id, Name: "Spinning", Capacity: 20}
	for n := -19; n < 0; n++ {
		s.fixture.Reservations = append(s.fixture.Reservations, Reservation{ID: string(rune(100 - n)), ClassID: id, AttemptID: int64(n)})
	}
	return s.fixture, s.createError
}
func (s *fakeStore) Reserve(_ context.Context, r Reservation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.persist {
		s.fixture.Reservations = append(s.fixture.Reservations, r)
	}
	return s.reserveError
}
func (s *fakeStore) ReadClass(context.Context, string) (Class, error) {
	if s.corrupt {
		s.fixture.Reservations = nil
	}
	return s.fixture, s.readError
}
func TestReconciliation(t *testing.T) {
	cases := []struct {
		name                            string
		store                           *fakeStore
		accepted, rejected, errs, infra int
		fail                            bool
	}{
		{name: "full", store: &fakeStore{reserveError: ErrFull}, rejected: 20},
		{name: "read failure", store: &fakeStore{reserveError: errors.New("driver detail")}, errs: 20, infra: 20},
		{name: "uncertain absent", store: &fakeStore{reserveError: ErrUncertain}, errs: 20, infra: 20},
		{name: "acknowledgement lost but persisted", store: &fakeStore{reserveError: ErrUncertain, persist: true}, accepted: 20, infra: 20},
		{name: "missing accepted rows", store: &fakeStore{}, fail: true},
		{name: "reconciliation unavailable", store: &fakeStore{readError: errors.New("offline")}, fail: true},
		{name: "fixture unavailable", store: &fakeStore{createError: errors.New("offline")}, fail: true},
		{name: "fixture rows lost", store: &fakeStore{corrupt: true}, fail: true},
	}
	for i := range cases {
		tc := &cases[i]
		t.Run(tc.name, func(t *testing.T) {
			report, err := Run(context.Background(), tc.store)
			if (err != nil) != tc.fail {
				t.Fatalf("error: %v", err)
			}
			if tc.fail {
				return
			}
			if report.Summary.Accepted != tc.accepted || report.Summary.Rejected != tc.rejected || report.Summary.Errors != tc.errs || report.Summary.InfrastructureErrors != tc.infra {
				t.Fatalf("bad totals: %+v", report.Summary)
			}
			if report.Summary.Overbooked != max(int64(report.FinalBookings)-20, 0) {
				t.Fatal("overbooking not stored count")
			}
			for i, attempt := range report.Attempts {
				if attempt.Number != i+1 || attempt.DurationMS < 0 {
					t.Fatalf("bad attempt %+v", attempt)
				}
			}
		})
	}
}
func TestHTTPContract(t *testing.T) {
	var ready atomic.Bool
	s := &fakeStore{reserveError: ErrFull}
	mux := http.NewServeMux()
	Register(mux, s, &ready)
	for _, tc := range []struct {
		method, path, body string
		ready              bool
		status             int
	}{
		{"GET", "/api/classes/demo", "", false, 503},
		{"POST", "/api/classes/demo/race", "", false, 503},
		{"GET", "/api/classes/demo?x=1", "", true, 400},
		{"POST", "/api/classes/demo/race", "{}", true, 400},
		{"POST", "/api/classes/demo", "", true, 405},
		{"GET", "/api/classes/demo/race", "", true, 405},
		{"GET", "/api/classes/demo", "", true, 200},
		{"POST", "/api/classes/demo/race", "", true, 200},
	} {
		ready.Store(tc.ready)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if w.Code != tc.status {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body)
		}
		if tc.status != 405 && (w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Content-Type") != "application/json") {
			t.Fatal("JSON/cache contract")
		}
	}
	s.createError = errors.New("private driver error")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/classes/demo/race", nil))
	if w.Code != 503 || strings.Contains(w.Body.String(), "private") {
		t.Fatal("storage failure leaked", w.Body)
	}
}

func TestHTTPTimeout(t *testing.T) {
	var ready atomic.Bool
	ready.Store(true)
	mux := http.NewServeMux()
	Register(mux, &fakeStore{createError: context.DeadlineExceeded}, &ready)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/classes/demo/race", nil))
	if w.Code != 504 || !strings.Contains(w.Body.String(), "demo_timeout") {
		t.Fatal("timeout contract", w.Code, w.Body)
	}
}
