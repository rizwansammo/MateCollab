package apiserver

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/rizwansammo/MateCollab/services/internal/auth"
	"github.com/rizwansammo/MateCollab/services/internal/emails"
	"github.com/rizwansammo/MateCollab/services/internal/memberships"
	"github.com/rizwansammo/MateCollab/services/internal/orgs"
	"github.com/rizwansammo/MateCollab/services/internal/platform/config"
	"github.com/rizwansammo/MateCollab/services/internal/platform/crypto"
	"github.com/rizwansammo/MateCollab/services/internal/platform/jobs"
	"github.com/rizwansammo/MateCollab/services/internal/platform/password"
	"github.com/rizwansammo/MateCollab/services/internal/platform/ratelimit"
	"github.com/rizwansammo/MateCollab/services/internal/users"
)

// Services are the feature modules of the API, wired together. mcctl builds the same graph for
// operator commands such as `org create`.
type Services struct {
	Auth        *auth.Service
	Users       *users.Service
	Memberships *memberships.Service
	Orgs        *orgs.Service
}

// Deps are the platform resources the modules are built from.
type Deps struct {
	Config *config.Config
	Pool   *pgxpool.Pool
	Redis  *redis.Client
	Jobs   jobs.Enqueuer
	Logger *slog.Logger
	Now    func() time.Time // injectable clock for tests
}

// NewServices builds the module graph. Configuration must already satisfy PartCrypto.
func NewServices(d Deps) (*Services, error) {
	if d.Now == nil {
		d.Now = time.Now
	}
	cfg := d.Config
	keyring, err := crypto.ParseKeyring(cfg.Crypto.Keyring, cfg.Crypto.ActiveKey)
	if err != nil {
		return nil, fmt.Errorf("keyring: %w", err)
	}
	subkeys, err := crypto.DeriveSubkeys(cfg.Crypto.HMACSecret)
	if err != nil {
		return nil, err
	}
	limiter := ratelimit.New(d.Redis)
	outbox := emails.NewOutbox(d.Jobs, keyring)
	// Argon2id at 64 MiB per hash: bound concurrent hashes so a login burst cannot exhaust memory.
	hasher := password.NewHasher(password.DefaultParams, max(2, runtime.GOMAXPROCS(0)))

	userSvc := users.New(d.Pool)
	authSvc := auth.New(auth.Deps{
		Pool: d.Pool, Redis: d.Redis, Hasher: hasher, Keyring: keyring, CSRFKey: subkeys.CSRF, Limiter: limiter,
		Outbox: outbox, Accounts: userSvc, PublicURL: cfg.PublicURL, Logger: d.Logger, Now: d.Now,
	})
	userSvc.SetVerifier(authSvc)
	memberSvc := memberships.New(memberships.Deps{
		Pool: d.Pool, Redis: d.Redis, Auth: authSvc, Users: userSvc, Outbox: outbox, Limiter: limiter,
		CursorKey: subkeys.Cursor, PublicURL: cfg.PublicURL, Logger: d.Logger, Now: d.Now,
	})
	orgSvc := orgs.New(orgs.Deps{
		Pool: d.Pool, Redis: d.Redis, Auth: authSvc, Users: userSvc, Memberships: memberSvc, Limiter: limiter,
		AllowSignup: cfg.Features.AllowOrgSignup, Logger: d.Logger, Now: d.Now,
	})
	return &Services{Auth: authSvc, Users: userSvc, Memberships: memberSvc, Orgs: orgSvc}, nil
}

// sessionJSON is the GET /auth/session document: everything the web client needs at start-up.
type sessionJSON struct {
	User          users.UserJSON                 `json:"user"`
	CSRFToken     string                         `json:"csrf_token"`
	Session       sessionInfoJSON                `json:"session"`
	MFA           mfaJSON                        `json:"mfa"`
	Organisations []memberships.OrganisationJSON `json:"organisations"`
}

type sessionInfoJSON struct {
	ID                uuid.UUID `json:"id"`
	AMR               []string  `json:"amr"`
	CreatedAt         time.Time `json:"created_at"`
	ExpiresAt         time.Time `json:"expires_at"`
	IdleExpiresAt     time.Time `json:"idle_expires_at"`
	ReauthenticatedAt time.Time `json:"reauthenticated_at"`
}

type mfaJSON struct {
	TOTPEnabled bool `json:"totp_enabled"`
}

// SessionView composes the session document across modules.
func (s *Services) SessionView(ctx context.Context, p auth.Principal) (any, error) {
	u, err := s.Users.Me(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	list, err := s.Memberships.MyOrganisations(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	return sessionJSON{
		User:      users.ToJSON(u),
		CSRFToken: s.Auth.CSRFToken(p),
		Session: sessionInfoJSON{ID: p.SessionID, AMR: p.AMR, CreatedAt: p.CreatedAt, ExpiresAt: p.ExpiresAt,
			IdleExpiresAt: p.IdleExpiresAt, ReauthenticatedAt: p.ReauthenticatedAt},
		MFA:           mfaJSON{TOTPEnabled: p.TOTPEnabled},
		Organisations: memberships.OrganisationsToJSON(list),
	}, nil
}
