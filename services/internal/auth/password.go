package auth

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rizwansammo/MateCollab/services/internal/auth/store"
	"github.com/rizwansammo/MateCollab/services/internal/emails"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/password"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

// ErrInvalidToken is returned for unknown, used or expired one-time links.
var ErrInvalidToken = apperr.Validation(apperr.Field("token", "invalid", "This link is invalid or has expired."))

// ErrReauthRequired asks the client to confirm the password before a sensitive change.
var ErrReauthRequired = apperr.Forbidden("reauthentication_required", "Confirm your password to continue.")

// CheckPassword validates a new password against the policy for the given account email.
func CheckPassword(field, pw, email string) error {
	if code := password.Check(pw, email); code != "" {
		return apperr.Validation(apperr.Field(field, code, password.Message(code)))
	}
	return nil
}

type resetData struct {
	Setup   bool       `json:"setup,omitempty"`
	EmailID *uuid.UUID `json:"email_id,omitempty"`
}

// ForgotPassword sends a reset link if the address belongs to an active account. The response
// is identical either way, so it cannot be used to discover accounts.
func (s *Service) ForgotPassword(ctx context.Context, email string, meta httpx.ClientMeta) error {
	email = NormalizeEmail(email)
	if err := s.Limiter.Check(ctx, "auth.recovery.ip", meta.IP.String(), limitRecoveryIP); err != nil {
		return err
	}
	if err := s.Limiter.Check(ctx, "auth.recovery.email", accountKey(email), limitRecoveryMail); err != nil {
		return nil // silently drop: the response must not reveal per-address throttling
	}
	q := store.New(s.Pool)
	row, err := q.LoginLookup(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if row.UserStatus != "active" {
		return nil
	}
	user := tenant.UserID(row.UserID)
	return db.InTx(ctx, s.Pool, func(tx *db.Tx) error {
		q := store.New(tx)
		token, err := s.issueToken(ctx, q, user, "password_reset", resetData{}, ResetTTL)
		if err != nil {
			return err
		}
		s.event(ctx, q, &user, "password.reset_requested", "success", nil, meta, nil)
		return s.Outbox.Enqueue(ctx, tx, emails.Message{
			Template: emails.PasswordReset, To: email,
			Data: map[string]string{"name": row.DisplayName, "link": s.link("/reset-password", token)},
		})
	})
}

// IssueAccountSetup creates a set-your-password link for an account created by an operator
// (mcctl org create) and emails it. Redeeming it also verifies the email address.
func (s *Service) IssueAccountSetup(ctx context.Context, tx *db.Tx, user tenant.UserID, emailID uuid.UUID,
	email, name, orgName string) (string, error) {
	q := store.New(tx)
	token, err := s.issueToken(ctx, q, user, "password_reset", resetData{Setup: true, EmailID: &emailID}, AccountSetupTTL)
	if err != nil {
		return "", err
	}
	link := s.link("/reset-password", token)
	return link, s.Outbox.Enqueue(ctx, tx, emails.Message{
		Template: emails.AccountSetup, To: email,
		Data: map[string]string{"name": name, "org": orgName, "link": link},
	})
}

func (s *Service) issueToken(ctx context.Context, q *store.Queries, user tenant.UserID, purpose string, data any, ttl time.Duration) (string, error) {
	uid := uuid.UUID(user)
	if purpose == "password_reset" {
		// A new reset link supersedes any earlier one.
		if err := q.InvalidateUserTokens(ctx, store.InvalidateUserTokensParams{UserID: &uid, Purpose: purpose}); err != nil {
			return "", err
		}
	}
	token, err := NewToken()
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	if _, err := q.CreateAuthToken(ctx, store.CreateAuthTokenParams{
		UserID: &uid, Purpose: purpose, TokenHash: hashToken(token), Data: raw, ExpiresAt: s.Now().Add(ttl),
	}); err != nil {
		return "", err
	}
	return token, nil
}

// ResetPassword redeems a reset (or account setup) link: sets the password, revokes every
// session and, for setup links, marks the email address verified.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string, meta httpx.ClientMeta) error {
	if err := s.Limiter.Check(ctx, "auth.recovery.ip", meta.IP.String(), limitRecoveryIP); err != nil {
		return err
	}
	if token == "" || len(token) > 128 {
		return ErrInvalidToken
	}
	var revoked [][]byte
	var user tenant.UserID
	var setup bool
	err := db.InTx(ctx, s.Pool, func(tx *db.Tx) error {
		q := store.New(tx)
		tok, err := q.GetAuthTokenForUpdate(ctx, store.GetAuthTokenForUpdateParams{TokenHash: hashToken(token), Purpose: "password_reset"})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidToken
		}
		if err != nil {
			return err
		}
		if tok.UsedAt != nil || !s.Now().Before(tok.ExpiresAt) || tok.UserID == nil {
			return ErrInvalidToken
		}
		user = tenant.UserID(*tok.UserID)
		var data resetData
		_ = json.Unmarshal(tok.Data, &data)
		setup = data.Setup
		if err := db.SetScope(ctx, tx, tenant.Scope{UserID: user}); err != nil {
			return err
		}
		name, email, err := s.Accounts.Profile(ctx, tx, user)
		if err != nil {
			return err
		}
		if err := CheckPassword("new_password", newPassword, email); err != nil {
			return err
		}
		hash, err := s.Hasher.Hash(ctx, newPassword)
		if err != nil {
			return err
		}
		if err := q.SetPassword(ctx, store.SetPasswordParams{UserID: uuid.UUID(user), PasswordHash: hash}); err != nil {
			return err
		}
		if err := q.UseAuthToken(ctx, tok.ID); err != nil {
			return err
		}
		if data.EmailID != nil {
			if err := s.Accounts.MarkEmailVerified(ctx, tx, user, *data.EmailID); err != nil {
				return err
			}
		}
		if revoked, err = q.RevokeAllSessions(ctx, store.RevokeAllSessionsParams{UserID: uuid.UUID(user), RevokedReason: ptr("password_reset")}); err != nil {
			return err
		}
		if !setup {
			return s.Outbox.Enqueue(ctx, tx, emails.Message{Template: emails.PasswordChanged, To: email, Data: map[string]string{"name": name}})
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.forget(ctx, revoked...)
	action := "password.reset"
	if setup {
		action = "password.set"
	}
	s.event(ctx, store.New(s.Pool), &user, action, "success", nil, meta, nil)
	return nil
}

// ChangePassword sets a new password for a recently re-authenticated session, signs out every
// other session and rotates the current one (the returned session replaces the cookie).
func (s *Service) ChangePassword(ctx context.Context, p Principal, newPassword string, meta httpx.ClientMeta) (NewSession, error) {
	if !p.RecentlyAuthenticated(s.Now()) {
		return NewSession{}, ErrReauthRequired
	}
	var ns NewSession
	var revoked [][]byte
	err := db.InUserTx(ctx, s.Pool, p.UserID, func(tx *db.Tx) error {
		q := store.New(tx)
		name, email, err := s.Accounts.Profile(ctx, tx, p.UserID)
		if err != nil {
			return err
		}
		if err := CheckPassword("new_password", newPassword, email); err != nil {
			return err
		}
		hash, err := s.Hasher.Hash(ctx, newPassword)
		if err != nil {
			return err
		}
		if err := q.SetPassword(ctx, store.SetPasswordParams{UserID: uuid.UUID(p.UserID), PasswordHash: hash}); err != nil {
			return err
		}
		if revoked, err = q.RevokeAllSessions(ctx, store.RevokeAllSessionsParams{UserID: uuid.UUID(p.UserID), RevokedReason: ptr("password_changed")}); err != nil {
			return err
		}
		if ns, err = s.startSession(ctx, q, p.UserID, p.AMR, meta); err != nil {
			return err
		}
		return s.Outbox.Enqueue(ctx, tx, emails.Message{Template: emails.PasswordChanged, To: email, Data: map[string]string{"name": name}})
	})
	if err != nil {
		return NewSession{}, err
	}
	s.forget(ctx, revoked...)
	s.event(ctx, store.New(s.Pool), &p.UserID, "password.changed", "success", &ns.Principal.SessionID, meta, nil)
	return ns, nil
}

// SetInitialPassword stores the password for an account created through an invitation.
func (s *Service) SetInitialPassword(ctx context.Context, tx *db.Tx, user tenant.UserID, email, pw string) error {
	if err := CheckPassword("password", pw, email); err != nil {
		return err
	}
	hash, err := s.Hasher.Hash(ctx, pw)
	if err != nil {
		return err
	}
	return store.New(tx).SetPassword(ctx, store.SetPasswordParams{UserID: uuid.UUID(user), PasswordHash: hash})
}
