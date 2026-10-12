package auth

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rizwansammo/MateCollab/services/internal/auth/store"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/redisx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

// ErrNoSession means the request carries no valid session (missing, expired or revoked).
var ErrNoSession = apperr.Unauthenticated("unauthenticated", "Sign in to continue.")

// NewSession is a freshly issued session. Token goes into the cookie and is never stored.
type NewSession struct {
	Token     string
	Principal Principal
}

func cacheKey(hash []byte) string { return redisx.Key("sess", hex.EncodeToString(hash)) }

// startSession issues a new session in q (a pool or a transaction).
func (s *Service) startSession(ctx context.Context, q *store.Queries, user tenant.UserID, amr []string, meta httpx.ClientMeta) (NewSession, error) {
	token, err := NewToken()
	if err != nil {
		return NewSession{}, err
	}
	now := s.Now()
	expires := now.Add(SessionAbsolute)
	row, err := q.CreateSession(ctx, store.CreateSessionParams{
		UserID:        uuid.UUID(user),
		TokenHash:     hashToken(token),
		Amr:           amr,
		IdleExpiresAt: now.Add(SessionIdle),
		ExpiresAt:     expires,
		Ip:            meta.IPPtr(),
		UserAgent:     meta.UserAgentPtr(),
	})
	if err != nil {
		return NewSession{}, err
	}
	return NewSession{Token: token, Principal: Principal{
		SessionID: row.ID, UserID: user, AMR: amr, CreatedAt: row.CreatedAt,
		ReauthenticatedAt: row.ReauthenticatedAt, IdleExpiresAt: now.Add(SessionIdle), ExpiresAt: expires,
		TokenHash: hashToken(token),
	}}, nil
}

// StartSession signs a user in (used after invitation acceptance creates an account).
func (s *Service) StartSession(ctx context.Context, user tenant.UserID, amr []string, meta httpx.ClientMeta) (NewSession, error) {
	ns, err := s.startSession(ctx, store.New(s.Pool), user, amr, meta)
	if err != nil {
		return NewSession{}, err
	}
	s.event(ctx, store.New(s.Pool), &user, "login.succeeded", "success", &ns.Principal.SessionID, meta, map[string]any{"amr": amr})
	return ns, nil
}

// Authenticate resolves a session cookie token to a principal, sliding the idle expiry.
func (s *Service) Authenticate(ctx context.Context, token string) (Principal, error) {
	if token == "" || len(token) > 128 {
		return Principal{}, ErrNoSession
	}
	hash := hashToken(token)
	now := s.Now()
	if raw, err := s.Redis.Get(ctx, cacheKey(hash)).Bytes(); err == nil {
		var p Principal
		if json.Unmarshal(raw, &p) == nil && now.Before(p.ExpiresAt) && now.Before(p.IdleExpiresAt) {
			return p, nil
		}
	}
	row, err := store.New(s.Pool).LookupSession(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrNoSession
	}
	if err != nil {
		return Principal{}, err
	}
	if row.RevokedAt != nil || row.UserStatus != "active" || !now.Before(row.ExpiresAt) || !now.Before(row.IdleExpiresAt) {
		return Principal{}, ErrNoSession
	}
	p := Principal{
		SessionID: row.SessionID, UserID: tenant.UserID(row.UserID), AMR: row.Amr, CreatedAt: row.CreatedAt,
		ReauthenticatedAt: row.ReauthenticatedAt, IdleExpiresAt: row.IdleExpiresAt, ExpiresAt: row.ExpiresAt,
		TOTPEnabled: row.TOTPEnabled, TokenHash: hash,
	}
	if now.Sub(row.LastSeenAt) > touchInterval {
		idle := now.Add(SessionIdle)
		if idle.After(row.ExpiresAt) {
			idle = row.ExpiresAt
		}
		if err := store.New(s.Pool).TouchSession(ctx, store.TouchSessionParams{ID: row.SessionID, IdleExpiresAt: idle}); err == nil {
			p.IdleExpiresAt = idle
		}
	}
	if raw, err := json.Marshal(p); err == nil {
		_ = s.Redis.Set(ctx, cacheKey(hash), raw, principalCacheTTL).Err()
	}
	return p, nil
}

// forget drops cached principals so revocations take effect immediately.
func (s *Service) forget(ctx context.Context, hashes ...[]byte) {
	if len(hashes) == 0 {
		return
	}
	keys := make([]string, len(hashes))
	for i, h := range hashes {
		keys[i] = cacheKey(h)
	}
	_ = s.Redis.Del(context.WithoutCancel(ctx), keys...).Err()
}

// Logout revokes the current session.
func (s *Service) Logout(ctx context.Context, p Principal, meta httpx.ClientMeta) error {
	q := store.New(s.Pool)
	hash, err := q.RevokeSession(ctx, store.RevokeSessionParams{ID: p.SessionID, UserID: uuid.UUID(p.UserID), RevokedReason: ptr("logout")})
	if errors.Is(err, pgx.ErrNoRows) {
		s.forget(ctx, p.TokenHash) // already revoked (e.g. by a password change)
		return nil
	}
	if err != nil {
		return err
	}
	s.forget(ctx, hash)
	s.event(ctx, q, &p.UserID, "logout", "success", &p.SessionID, meta, nil)
	return nil
}

// SessionInfo describes one of the user's active sessions.
type SessionInfo struct {
	ID         uuid.UUID
	Current    bool
	AMR        []string
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
	IP         string
	UserAgent  string
}

// ListSessions returns the user's active sessions, current first.
func (s *Service) ListSessions(ctx context.Context, p Principal) ([]SessionInfo, error) {
	rows, err := store.New(s.Pool).ListActiveSessions(ctx, uuid.UUID(p.UserID))
	if err != nil {
		return nil, err
	}
	out := make([]SessionInfo, 0, len(rows))
	for _, r := range rows {
		info := SessionInfo{ID: r.ID, Current: r.ID == p.SessionID, AMR: r.Amr, CreatedAt: r.CreatedAt, LastSeenAt: r.LastSeenAt, ExpiresAt: r.ExpiresAt}
		if r.Ip != nil {
			info.IP = r.Ip.String()
		}
		if r.UserAgent != nil {
			info.UserAgent = *r.UserAgent
		}
		if info.Current {
			out = append([]SessionInfo{info}, out...)
		} else {
			out = append(out, info)
		}
	}
	return out, nil
}

// RevokeSession signs out one of the user's sessions (404 for anyone else's).
func (s *Service) RevokeSession(ctx context.Context, p Principal, id uuid.UUID, meta httpx.ClientMeta) error {
	q := store.New(s.Pool)
	hash, err := q.RevokeSession(ctx, store.RevokeSessionParams{ID: id, UserID: uuid.UUID(p.UserID), RevokedReason: ptr("revoked_by_user")})
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound()
	}
	if err != nil {
		return err
	}
	s.forget(ctx, hash)
	s.event(ctx, q, &p.UserID, "session.revoked", "success", &p.SessionID, meta, map[string]any{"revoked_session_id": id.String()})
	return nil
}

// RevokeOtherSessions signs out every session except the current one.
func (s *Service) RevokeOtherSessions(ctx context.Context, p Principal, meta httpx.ClientMeta) (int, error) {
	q := store.New(s.Pool)
	hashes, err := q.RevokeOtherSessions(ctx, store.RevokeOtherSessionsParams{UserID: uuid.UUID(p.UserID), ID: p.SessionID, RevokedReason: ptr("signed_out_elsewhere")})
	if err != nil {
		return 0, err
	}
	s.forget(ctx, hashes...)
	s.event(ctx, q, &p.UserID, "session.revoked_others", "success", &p.SessionID, meta, map[string]any{"count": len(hashes)})
	return len(hashes), nil
}

func ptr[T any](v T) *T { return &v }
