// Package health serves liveness and readiness endpoints on the internal admin listener.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Check is one dependency probe. Critical checks make the process not ready when they fail.
type Check struct {
	Name     string
	Critical bool
	Probe    func(context.Context) error
}

// Result is the outcome of one check.
type Result struct {
	Name     string `json:"name"`
	Status   string `json:"status"` // "ok" or "fail"
	Critical bool   `json:"critical"`
	Error    string `json:"error,omitempty"`
	Millis   int64  `json:"duration_ms"`
}

// Report is the readiness response body.
type Report struct {
	Status string   `json:"status"` // "ready" or "not_ready"
	Checks []Result `json:"checks"`
}

// Run executes every check concurrently with a shared timeout.
func Run(ctx context.Context, checks []Check, timeout time.Duration) Report {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	results := make([]Result, len(checks))
	var wg sync.WaitGroup
	for i, c := range checks {
		wg.Go(func() {
			start := time.Now()
			err := c.Probe(ctx)
			r := Result{Name: c.Name, Status: "ok", Critical: c.Critical, Millis: time.Since(start).Milliseconds()}
			if err != nil {
				r.Status, r.Error = "fail", err.Error()
			}
			results[i] = r
		})
	}
	wg.Wait()
	rep := Report{Status: "ready", Checks: results}
	for _, r := range results {
		if r.Critical && r.Status != "ok" {
			rep.Status = "not_ready"
		}
	}
	return rep
}

// Liveness reports that the process is running; it never checks dependencies.
func Liveness() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(`{"status":"ok"}` + "\n"))
	})
}

// Readiness runs the checks and answers 200 when all critical checks pass, otherwise 503.
func Readiness(checks []Check) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rep := Run(r.Context(), checks, 3*time.Second)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if rep.Status != "ready" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(rep)
	})
}
