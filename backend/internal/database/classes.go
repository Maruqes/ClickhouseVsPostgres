package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"example.com/clickhouse-vs-postgres/backend/internal/classes"
)

func (s *store) DemoClass(ctx context.Context) (classes.Class, error) {
	return s.ReadClass(ctx, classes.TemplateID)
}
func (s *store) ReadClass(ctx context.Context, id string) (classes.Class, error) {
	result := classes.Class{ID: id, Reservations: []classes.Reservation{}}
	if err := s.db.QueryRowContext(ctx, s.dialect.bind("SELECT name, capacity FROM demo_classes WHERE id = ?"), id).Scan(&result.Name, &result.Capacity); err != nil {
		return result, err
	}
	rows, err := s.db.QueryContext(ctx, s.dialect.bind("SELECT id, class_id, attempt_id, created_at FROM demo_reservations WHERE class_id = ? ORDER BY attempt_id, id"), id)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var reservation classes.Reservation
		if err := rows.Scan(&reservation.ID, &reservation.ClassID, &reservation.AttemptID, &reservation.CreatedAt); err != nil {
			return result, err
		}
		reservation.CreatedAt = reservation.CreatedAt.UTC()
		result.Reservations = append(result.Reservations, reservation)
	}
	return result, rows.Err()
}

func (s *store) migrateClasses(ctx context.Context) error {
	var count int
	if err := s.db.QueryRowContext(ctx, s.dialect.bind("SELECT COUNT(*) FROM demo_classes WHERE id = ?"), classes.TemplateID).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if err := s.exec(ctx, "INSERT INTO demo_classes (id, name, capacity) VALUES (?, ?, ?)", classes.TemplateID, "Spinning", int64(20)); err != nil {
			return err
		}
	}
	for attempt := int64(-19); attempt < 0; attempt++ {
		id := fmt.Sprintf("%s-%02d", classes.TemplateID, -attempt)
		if err := s.db.QueryRowContext(ctx, s.dialect.bind("SELECT COUNT(*) FROM demo_reservations WHERE id = ?"), id).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			if err := s.exec(ctx, "INSERT INTO demo_reservations (id, class_id, attempt_id, created_at) VALUES (?, ?, ?, ?)", id, classes.TemplateID, attempt, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *store) CreateFixture(ctx context.Context, id string) (classes.Class, error) {
	template, err := s.DemoClass(ctx)
	if err != nil {
		return classes.Class{}, err
	}
	if err := s.exec(ctx, "INSERT INTO demo_classes (id, name, capacity) VALUES (?, ?, ?)", id, template.Name, template.Capacity); err != nil {
		return classes.Class{}, err
	}
	values := make([]string, 0, len(template.Reservations))
	args := make([]any, 0, len(template.Reservations)*4)
	for _, initial := range template.Reservations {
		values = append(values, "(?, ?, ?, ?)")
		args = append(args, fmt.Sprintf("%s-initial-%02d", id, -initial.AttemptID), id, initial.AttemptID, initial.CreatedAt)
	}
	if err := s.exec(ctx, "INSERT INTO demo_reservations (id, class_id, attempt_id, created_at) VALUES "+strings.Join(values, ","), args...); err != nil {
		return classes.Class{}, err
	}
	return s.ReadClass(ctx, id)
}

// SQL executors stay private to persistence. A transaction and a connection run
// the same availability and insert statements; only the lock protocol differs.
type reservationQueries interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (s *store) Reserve(ctx context.Context, reservation classes.Reservation) error {
	if !s.dialect.numbered {
		return s.reserve(ctx, s.db, reservation, false)
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.reserve(ctx, tx, reservation, true); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: %v", classes.ErrUncertain, err)
	}
	return nil
}
func (s *store) reserve(ctx context.Context, q reservationQueries, reservation classes.Reservation, lock bool) error {
	var capacity, booked int64
	query := "SELECT capacity FROM demo_classes WHERE id = ?"
	if lock {
		query += " FOR UPDATE"
	}
	if err := q.QueryRowContext(ctx, s.dialect.bind(query), reservation.ClassID).Scan(&capacity); err != nil {
		return err
	}
	// Fresh READ COMMITTED statement after the PostgreSQL lock has been acquired.
	if err := q.QueryRowContext(ctx, s.dialect.bind("SELECT COUNT(*) FROM demo_reservations WHERE class_id = ?"), reservation.ClassID).Scan(&booked); err != nil {
		return err
	}
	if booked >= capacity {
		return classes.ErrFull
	}
	_, err := q.ExecContext(ctx, s.dialect.bind("INSERT INTO demo_reservations (id, class_id, attempt_id, created_at) VALUES (?, ?, ?, ?)"), reservation.ID, reservation.ClassID, reservation.AttemptID, reservation.CreatedAt)
	if err != nil {
		return errors.Join(classes.ErrUncertain, err)
	}
	return nil
}
