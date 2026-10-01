package main

import (
	"context"
	"errors"
	"example.com/clickhouse-vs-postgres/backend/internal/analytics"
	"example.com/clickhouse-vs-postgres/backend/internal/classes"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"example.com/clickhouse-vs-postgres/backend/internal/database"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	store, err := database.Open(os.Getenv("DATABASE_DRIVER"), os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer store.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	rows, err := analytics.ConfiguredRows()
	if err != nil {
		return err
	}
	var ready atomic.Bool
	mux := http.NewServeMux()
	classes.Register(mux, store, &ready)
	mux.Handle("/", analytics.Handler(store, analytics.Dataset(rows), &ready))
	migrationErrors := make(chan error, 1)
	go func() {
		if err := store.Migrate(ctx, rows); err != nil {
			migrationErrors <- err
			return
		}
		ready.Store(true)
		slog.Info("dataset ready", "rows", rows)
	}()
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{
		Addr: addr, Handler: mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second, WriteTimeout: 35 * time.Second,
		IdleTimeout: 60 * time.Second,
	}
	errs := make(chan error, 1)
	go func() {
		slog.Info("server listening", "address", addr)
		errs <- server.ListenAndServe()
	}()
	select {
	case err := <-errs:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case err := <-migrationErrors:
		_ = server.Close()
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
	return nil
}
