package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReadinessFailsOnlyOnCriticalChecks(t *testing.T) {
	ok := func(context.Context) error { return nil }
	bad := func(context.Context) error { return errors.New("down") }

	rep := Run(context.Background(), []Check{{Name: "db", Critical: true, Probe: ok}, {Name: "optional", Probe: bad}}, time.Second)
	if rep.Status != "ready" {
		t.Fatalf("non-critical failure must not affect readiness: %+v", rep)
	}

	w := httptest.NewRecorder()
	Readiness([]Check{{Name: "db", Critical: true, Probe: bad}}).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), `"not_ready"`) {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
}

func TestChecksRespectTimeout(t *testing.T) {
	slow := func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
	start := time.Now()
	rep := Run(context.Background(), []Check{{Name: "slow", Critical: true, Probe: slow}}, 50*time.Millisecond)
	if time.Since(start) > time.Second || rep.Status != "not_ready" {
		t.Fatalf("timeout not applied: %+v", rep)
	}
}
