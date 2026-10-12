//go:build integration

package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/ratelimit"
	"github.com/rizwansammo/MateCollab/services/internal/platform/redisx"
	"github.com/rizwansammo/MateCollab/services/internal/testkit"
)

func TestGCRAAllowsBurstThenRefuses(t *testing.T) {
	url := testkit.Require(t, testkit.EnvRedisURL)
	rdb, err := redisx.New(url)
	if err != nil {
		t.Fatal(err)
	}
	defer rdb.Close()
	l := ratelimit.New(rdb)
	ctx := context.Background()
	subject := uuid.NewString()
	lim := ratelimit.Limit{Burst: 5, Period: time.Minute}

	for i := 0; i < 5; i++ {
		res, err := l.Allow(ctx, "test", subject, lim)
		if err != nil || !res.Allowed {
			t.Fatalf("request %d refused: %+v %v", i+1, res, err)
		}
	}
	res, err := l.Allow(ctx, "test", subject, lim)
	if err != nil || res.Allowed {
		t.Fatalf("sixth request allowed: %+v %v", res, err)
	}
	// One emission interval (12 s) at most until the next slot.
	if res.RetryAfter <= 0 || res.RetryAfter > 12*time.Second {
		t.Fatalf("retry after %v", res.RetryAfter)
	}
	// Check returns the typed error with Retry-After.
	e, ok := apperr.As(l.Check(ctx, "test", subject, lim))
	if !ok || e.Code != "rate_limited" || e.RetryAfter <= 0 {
		t.Fatalf("Check = %+v", e)
	}
	// Other subjects are independent; Reset clears the bucket.
	if res, _ := l.Allow(ctx, "test", uuid.NewString(), lim); !res.Allowed {
		t.Fatal("independent subject refused")
	}
	l.Reset(ctx, "test", subject)
	if res, _ := l.Allow(ctx, "test", subject, lim); !res.Allowed {
		t.Fatal("reset did not clear the bucket")
	}
}

func TestGCRARefillsOverTime(t *testing.T) {
	url := testkit.Require(t, testkit.EnvRedisURL)
	rdb, _ := redisx.New(url)
	defer rdb.Close()
	l := ratelimit.New(rdb)
	ctx := context.Background()
	subject := uuid.NewString()
	lim := ratelimit.Limit{Burst: 2, Period: 400 * time.Millisecond} // one slot per 200 ms
	for i := 0; i < 2; i++ {
		_, _ = l.Allow(ctx, "refill", subject, lim)
	}
	if res, _ := l.Allow(ctx, "refill", subject, lim); res.Allowed {
		t.Fatal("expected refusal at capacity")
	}
	time.Sleep(250 * time.Millisecond)
	if res, _ := l.Allow(ctx, "refill", subject, lim); !res.Allowed {
		t.Fatal("slot did not refill")
	}
}
