// Package ratelimit implements the Redis-backed GCRA limiter (SECURITY-BASELINE.md §8,
// API-DESIGN.md §8). One script call per check; the server's clock is used so every API node
// shares the same view.
package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/redisx"
)

// Limit allows Burst requests at once and refills at Burst per Period (e.g. 10 per minute).
type Limit struct {
	Burst  int
	Period time.Duration
}

// PerMinute and PerHour are convenience constructors.
func PerMinute(n int) Limit { return Limit{Burst: n, Period: time.Minute} }
func PerHour(n int) Limit   { return Limit{Burst: n, Period: time.Hour} }
func PerDay(n int) Limit    { return Limit{Burst: n, Period: 24 * time.Hour} }

// Result of one check.
type Result struct {
	Allowed    bool
	RetryAfter time.Duration
	Remaining  int
}

// GCRA: each request advances the theoretical arrival time (TAT) by one emission interval;
// a request is refused while the TAT would exceed now + burst × interval.
var gcra = redis.NewScript(`
local t = redis.call('TIME')
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
local interval = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local tat = tonumber(redis.call('GET', KEYS[1]) or now)
if tat < now then tat = now end
local new_tat = tat + interval
local allow_at = new_tat - interval * burst
if now < allow_at then
  return {0, allow_at - now, 0}
end
redis.call('SET', KEYS[1], new_tat, 'PX', math.ceil(new_tat - now))
return {1, 0, math.floor((now - allow_at) / interval)}
`)

// Limiter checks limits stored in Valkey/Redis.
type Limiter struct {
	rdb *redis.Client
}

// New returns a limiter using rdb.
func New(rdb *redis.Client) *Limiter { return &Limiter{rdb: rdb} }

// Allow records one request for (class, subject) and reports whether it is within lim.
func (l *Limiter) Allow(ctx context.Context, class, subject string, lim Limit) (Result, error) {
	interval := lim.Period.Milliseconds() / int64(lim.Burst)
	if interval < 1 {
		interval = 1
	}
	vals, err := gcra.Run(ctx, l.rdb, []string{redisx.Key("rl", class, subject)}, interval, lim.Burst).Int64Slice()
	if err != nil {
		return Result{}, err
	}
	return Result{
		Allowed:    vals[0] == 1,
		RetryAfter: time.Duration(vals[1]) * time.Millisecond,
		Remaining:  int(vals[2]),
	}, nil
}

// Check is Allow returning a ready-to-send rate-limited error when the limit is exceeded.
// Limiter outages fail open (logged by the caller) so a cache problem never locks users out;
// the per-account delays in the auth module still apply.
func (l *Limiter) Check(ctx context.Context, class, subject string, lim Limit) error {
	res, err := l.Allow(ctx, class, subject, lim)
	if err != nil {
		return nil
	}
	if !res.Allowed {
		return apperr.RateLimited(res.RetryAfter)
	}
	return nil
}

// Reset clears a bucket (e.g. after a successful sign-in).
func (l *Limiter) Reset(ctx context.Context, class, subject string) {
	_ = l.rdb.Del(ctx, redisx.Key("rl", class, subject)).Err()
}
