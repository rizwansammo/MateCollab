// Package redisx connects to the Redis-compatible server (Valkey in every environment, ADR-011).
// Only core commands are used so any Redis-protocol service remains compatible. Nothing stored
// here may be the only copy of durable data.
package redisx

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// New parses a redis:// or rediss:// URL and returns a client. It does not connect yet.
func New(url string) (*redis.Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, errors.New("redis: MC_REDIS_URL could not be parsed")
	}
	opt.DialTimeout = 5 * time.Second
	opt.ReadTimeout = 3 * time.Second
	opt.WriteTimeout = 3 * time.Second
	opt.ClientName = "matecollab"
	return redis.NewClient(opt), nil
}

// Ping checks connectivity and authentication.
func Ping(ctx context.Context, c *redis.Client) error {
	return c.Ping(ctx).Err()
}

// Key builds namespaced keys: Key("org", orgID, "presence") → "mc:org:<id>:presence".
func Key(parts ...string) string {
	n := 2
	for _, p := range parts {
		n += len(p) + 1
	}
	b := make([]byte, 0, n)
	b = append(b, "mc"...)
	for _, p := range parts {
		b = append(b, ':')
		b = append(b, p...)
	}
	return string(b)
}
