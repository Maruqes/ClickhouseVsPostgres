package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
 "net/url"
	"regexp"
	"sync/atomic"
	"time"
)

// Reader is a domain-oriented persistence boundary, with no SQL or engine flags.
type Reader interface {
	Ping(context.Context) error
	Country(context.Context, Filter, bool) (Result, error)
}

var countryPattern = regexp.MustCompile(`^[A-Z]{2}$`)

func ParseFilter(r *http.Request) (Filter, error) {
	values, parseErr := url.ParseQuery(r.URL.RawQuery)
 if parseErr != nil { return Filter{}, errors.New("query parameters must be valid URL encoding") }
	for key, entries := range values {
		if (key != "country" && key != "from" && key != "to") || len(entries) != 1 {
			return Filter{}, errors.New("use country, from and to once each")
		}
	}
	country := values.Get("country")
	if !countryPattern.MatchString(country) {
		return Filter{}, errors.New("country must be a two-letter uppercase code")
	}
	from, e1 := time.Parse(DateLayout, values.Get("from"))
	to, e2 := time.Parse(DateLayout, values.Get("to"))
	if e1 != nil || e2 != nil || from.After(to) || from.Before(FirstDay) || to.After(LastDay) {
		return Filter{}, errors.New("choose an ordered UTC date range within the dataset")
	}
	return Filter{country, from, to}, nil
}

func Handler(store Reader, metadata Metadata, ready *atomic.Bool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) { JSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if !ready.Load() || store.Ping(ctx) != nil {
			JSON(w, 503, map[string]string{"status": "unavailable"})
			return
		}
		JSON(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/dataset", func(w http.ResponseWriter, r *http.Request) {
		if !ready.Load() {
			JSON(w, 503, map[string]string{"error": "dataset_unavailable"})
			return
		}
		JSON(w, 200, metadata)
	})
	for _, endpoint := range []struct {
		path   string
		detail bool
	}{{"/api/country/summary", false}, {"/api/country/detail", true}} {
		mux.HandleFunc("GET "+endpoint.path, func(w http.ResponseWriter, r *http.Request) {
			filter, err := ParseFilter(r)
			if err != nil {
				JSON(w, 400, map[string]string{"error": "invalid_request", "message": err.Error()})
				return
			}
			if !ready.Load() {
				JSON(w, 503, map[string]string{"error": "dataset_unavailable"})
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			defer cancel()
			result, err := store.Country(ctx, filter, endpoint.detail)
			if err != nil {
				// Clients never receive a driver error, table name or connection details.
				slog.Warn("country analytics failed", "error", err)
				if r.Context().Err() != nil {
					return
				}
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
					JSON(w, 504, map[string]string{"error": "query_timeout"})
					return
				}
				JSON(w, 503, map[string]string{"error": "query_unavailable"})
				return
			}
			JSON(w, 200, result)
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})
}

func JSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
