// Package auth owns authentication: password sign-in with Argon2id, TOTP two-step
// verification and recovery codes, server-side sessions, CSRF tokens, step-up
// re-authentication, password reset, email verification and the account security history
// (BACKEND.md §2, SECURITY-BASELINE.md §3–5).
//
// Secrets are never stored recoverably: sessions, one-time tokens and invitation tokens are
// stored as SHA-256 hashes, recovery codes as keyring MACs, TOTP secrets encrypted with the
// keyring, and none of them are logged.
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/rizwansammo/MateCollab/services/internal/emails"
	"github.com/rizwansammo/MateCollab/services/internal/platform/crypto"
	"github.com/rizwansammo/MateCollab/services/internal/platform/password"
	"github.com/rizwansammo/MateCollab/services/internal/platform/ratelimit"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

// Lifetimes (SECURITY-BASELINE.md §3–4).
const (
	SessionIdle          = 7 * 24 * time.Hour  // sliding
	SessionAbsolute      = 30 * 24 * time.Hour // platform cap
	MFAChallengeTTL      = 5 * time.Minute
	MFAChallengeAttempts = 5
	ResetTTL             = 60 * time.Minute
	AccountSetupTTL      = 72 * time.Hour
	VerificationTTL      = 24 * time.Hour
	StepUpWindow         = 10 * time.Minute
	touchInterval        = 5 * time.Minute
	principalCacheTTL    = 60 * time.Second
)

// Accounts is what auth needs from the users module (which owns users and user_emails). The
// transaction already carries the user's RLS context.
type Accounts interface {
	Profile(ctx context.Context, tx pgx.Tx, user tenant.UserID) (displayName, primaryEmail string, err error)
	Email(ctx context.Context, tx pgx.Tx, user tenant.UserID, emailID uuid.UUID) (address string, verified bool, err error)
	MarkEmailVerified(ctx context.Context, tx pgx.Tx, user tenant.UserID, emailID uuid.UUID) error
	ScheduleDeletion(ctx context.Context, tx pgx.Tx, user tenant.UserID, requested, scheduled time.Time) (bool, error)
	CancelDeletion(ctx context.Context, tx pgx.Tx, user tenant.UserID) (bool, error)
}

// Outbox queues platform emails inside a transaction.
type Outbox interface {
	Enqueue(ctx context.Context, tx pgx.Tx, m emails.Message) error
}

// Deps are the collaborators of the auth service.
type Deps struct {
	Pool      *pgxpool.Pool
	Redis     *redis.Client
	Hasher    *password.Hasher
	Keyring   *crypto.Keyring
	CSRFKey   []byte
	Limiter   *ratelimit.Limiter
	Outbox    Outbox
	Accounts  Accounts
	PublicURL *url.URL
	Logger    *slog.Logger
	Now       func() time.Time // injectable clock for tests
}

// Service implements the authentication use cases.
type Service struct {
	Deps
	ownership Ownership
}

// New returns an auth service.
func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{Deps: d}
}

// Principal is the authenticated session attached to a request.
type Principal struct {
	SessionID         uuid.UUID     `json:"sid"`
	UserID            tenant.UserID `json:"uid"`
	AMR               []string      `json:"amr"`
	CreatedAt         time.Time     `json:"cat"`
	ReauthenticatedAt time.Time     `json:"rat"`
	IdleExpiresAt     time.Time     `json:"iex"`
	ExpiresAt         time.Time     `json:"exp"`
	TOTPEnabled       bool          `json:"totp"`
	// TokenHash identifies the cache entry to evict after changes to this session. It is the
	// SHA-256 of the cookie token, from which the token cannot be recovered.
	TokenHash []byte `json:"th"`
}

// RecentlyAuthenticated reports whether the session passed a password check within the
// step-up window.
func (p Principal) RecentlyAuthenticated(now time.Time) bool {
	return now.Sub(p.ReauthenticatedAt) <= StepUpWindow
}

type principalKey struct{}

// WithPrincipal stores p on ctx.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the request's principal.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// hashToken is how every bearer token (session, one-time, invitation) is stored.
func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// HashToken exposes the token hashing used for storage (invitation tokens use it too).
func HashToken(token string) []byte { return hashToken(token) }

// NewToken returns a 256-bit random URL-safe token.
func NewToken() (string, error) { return crypto.RandomToken(32) }

// NormalizeEmail trims and lower-cases an address for lookups and rate-limit keys.
func NormalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// accountKey is a stable, non-reversible rate-limit subject for an email address, so unknown
// and known addresses are throttled identically.
func accountKey(email string) string {
	sum := sha256.Sum256([]byte(NormalizeEmail(email)))
	return hex.EncodeToString(sum[:16])
}

func (s *Service) link(path, token string) string {
	u := *s.PublicURL
	u.Path = path
	u.RawQuery = ""
	u.Fragment = "token=" + token // fragments never reach servers, logs or Referer headers
	return u.String()
}
