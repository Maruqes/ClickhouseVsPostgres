// Package classes owns the shared reservation experiment and its HTTP contract.
package classes

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"
)

const Attempts = 20
const TemplateID = "spinning-template-v1"

var ErrFull = errors.New("class_full")
var ErrUncertain = errors.New("write_uncertain")

type Reservation struct {
	ID        string    `json:"id"`
	ClassID   string    `json:"class_id"`
	AttemptID int64     `json:"attempt_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Class struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Capacity     int64         `json:"capacity"`
	Reservations []Reservation `json:"reservations"`
}
type Storage interface {
	DemoClass(context.Context) (Class, error)
	CreateFixture(context.Context, string) (Class, error)
	Reserve(context.Context, Reservation) error
	ReadClass(context.Context, string) (Class, error)
}
type Attempt struct {
	Number              int     `json:"number"`
	ReservationID       string  `json:"reservation_id"`
	Outcome             string  `json:"outcome"`
	DurationMS          float64 `json:"duration_ms"`
	Reason              string  `json:"reason"`
	InfrastructureError bool    `json:"infrastructure_error"`
}
type Summary struct {
	Accepted             int   `json:"accepted"`
	Rejected             int   `json:"rejected"`
	Errors               int   `json:"errors"`
	InfrastructureErrors int   `json:"infrastructure_errors"`
	Overbooked           int64 `json:"overbooked"`
}
type Report struct {
	FixtureID       string    `json:"fixture_id"`
	Name            string    `json:"name"`
	Capacity        int64     `json:"capacity"`
	InitialBookings int       `json:"initial_bookings"`
	FinalBookings   int       `json:"final_bookings"`
	ElapsedMS       float64   `json:"elapsed_ms"`
	Attempts        []Attempt `json:"attempts"`
	Summary         Summary   `json:"summary"`
}

func Run(ctx context.Context, storage Storage) (Report, error) {
	started := time.Now()
	id := "race-" + rand.Text()
	// The fixture and contenders share a 25-second work deadline, with the
	// remaining HTTP budget reserved for reconciliation.
	workCtx, cancel := context.WithDeadline(ctx, started.Add(25*time.Second))
	defer cancel()
	fixture, err := storage.CreateFixture(workCtx, id)
	if err != nil {
		return Report{}, err
	}
	if fixture.Capacity != 20 || len(fixture.Reservations) != 19 {
		return Report{}, errors.New("invalid fixture")
	}
	report := Report{FixtureID: id, Name: fixture.Name, Capacity: fixture.Capacity, InitialBookings: len(fixture.Reservations), Attempts: make([]Attempt, Attempts)}
	gate := make(chan struct{})
	var workers, waiting sync.WaitGroup
	workers.Add(Attempts)
	waiting.Add(Attempts)
	for index := range report.Attempts {
		go func() {
			defer workers.Done()
			waiting.Done()
			<-gate
			start := time.Now()
			reservation := Reservation{ID: fmt.Sprintf("%s-%02d", id, index+1), ClassID: id, AttemptID: int64(index + 1), CreatedAt: time.Now().UTC().Truncate(time.Millisecond)}
			err := storage.Reserve(workCtx, reservation)
			attempt := Attempt{Number: index + 1, ReservationID: reservation.ID, Outcome: "accepted", DurationMS: float64(time.Since(start).Microseconds()) / 1000, Reason: ""}
			if errors.Is(err, ErrFull) {
				attempt.Outcome = "rejected"
				attempt.Reason = "class_full"
			} else if err != nil {
				attempt.Outcome = "error"
				attempt.InfrastructureError = true
				attempt.Reason = "reservation_unavailable"
				if errors.Is(err, ErrUncertain) {
					attempt.Reason = "write_uncertain"
				}
			}
			report.Attempts[index] = attempt
		}()
	}
	waiting.Wait()
	close(gate)
	workers.Wait()
	stored, err := storage.ReadClass(ctx, id)
	if err != nil {
		return Report{}, err
	}
	counts := make(map[string]int, len(stored.Reservations))
	for _, reservation := range stored.Reservations {
		counts[reservation.ID]++
	}
	for _, initial := range fixture.Reservations {
		if counts[initial.ID] != 1 {
			return Report{}, errors.New("fixture reconciliation failed")
		}
		delete(counts, initial.ID)
	}
	for index := range report.Attempts {
		attempt := &report.Attempts[index]
		if counts[attempt.ReservationID] > 1 {
			return Report{}, errors.New("duplicate reservation")
		}
		if counts[attempt.ReservationID] == 1 {
			if attempt.Outcome == "rejected" {
				return Report{}, errors.New("rejected reservation persisted")
			}
			attempt.Outcome = "accepted"
			if attempt.InfrastructureError {
				attempt.Reason = "confirmed_from_storage"
			}
		} else if attempt.Outcome == "accepted" {
			return Report{}, errors.New("confirmed reservation missing")
		}
		delete(counts, attempt.ReservationID)
		if attempt.InfrastructureError {
			report.Summary.InfrastructureErrors++
		}
		switch attempt.Outcome {
		case "accepted":
			report.Summary.Accepted++
		case "rejected":
			report.Summary.Rejected++
		case "error":
			report.Summary.Errors++
		}
	}
	if len(counts) != 0 {
		return Report{}, errors.New("unexpected reservations")
	}
	report.FinalBookings = len(stored.Reservations)
	report.Summary.Overbooked = max(int64(report.FinalBookings)-report.Capacity, 0)
	report.ElapsedMS = float64(time.Since(started).Microseconds()) / 1000
	return report, nil
}
