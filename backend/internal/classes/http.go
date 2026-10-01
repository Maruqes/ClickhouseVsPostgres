package classes

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"example.com/clickhouse-vs-postgres/backend/internal/analytics"
)

func Register(mux *http.ServeMux, storage Storage, ready *atomic.Bool) {
	for _, route := range []string{"GET /api/classes/demo", "POST /api/classes/demo/race"} {
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			// This fixed experiment has no input parameters or request body.
			body, err := io.ReadAll(io.LimitReader(r.Body, 1))
			if err != nil || len(body) != 0 || r.URL.RawQuery != "" {
				analytics.JSON(w, 400, map[string]string{"error": "invalid_request", "message": "this experiment accepts no parameters or body"})
				return
			}
			if !ready.Load() {
				analytics.JSON(w, 503, map[string]string{"error": "demo_unavailable"})
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			defer cancel()
			var result any
			if r.Method == http.MethodGet {
				result, err = storage.DemoClass(ctx)
			} else {
				result, err = Run(ctx, storage)
			}
			if err != nil {
				slog.Warn("reservation demo failed", "error", err)
				if r.Context().Err() != nil {
					return
				}
				if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
					analytics.JSON(w, 504, map[string]string{"error": "demo_timeout"})
				} else {
					analytics.JSON(w, 503, map[string]string{"error": "demo_unavailable"})
				}
				return
			}
			analytics.JSON(w, 200, result)
		})
	}
}
