package auth

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rizwansammo/MateCollab/services/internal/auth/store"
	"github.com/rizwansammo/MateCollab/services/internal/emails"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

type verificationData struct {
	EmailID uuid.UUID `json:"email_id"`
}

// IssueEmailVerification emails a single-use link proving control of an address (called by the
// users module when an address is added). tx carries the user's context.
func (s *Service) IssueEmailVerification(ctx context.Context, tx *db.Tx, user tenant.UserID, emailID uuid.UUID, email, name string) error {
	token, err := s.issueToken(ctx, store.New(tx), user, "email_verification", verificationData{EmailID: emailID}, VerificationTTL)
	if err != nil {
		return err
	}
	return s.Outbox.Enqueue(ctx, tx, emails.Message{
		Template: emails.EmailVerification, To: email,
		Data: map[string]string{"name": name, "email": email, "link": s.link("/verify-email", token)},
	})
}

// ResendVerification sends a new link for one of the user's unverified addresses.
func (s *Service) ResendVerification(ctx context.Context, p Principal, emailID uuid.UUID, meta httpx.ClientMeta) error {
	if err := s.Limiter.Check(ctx, "auth.recovery.email", p.UserID.String(), limitRecoveryMail); err != nil {
		return err
	}
	return db.InUserTx(ctx, s.Pool, p.UserID, func(tx *db.Tx) error {
		address, verified, err := s.Accounts.Email(ctx, tx, p.UserID, emailID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if verified {
			return apperr.Conflict("email_already_verified", "This address is already verified.")
		}
		name, _, err := s.Accounts.Profile(ctx, tx, p.UserID)
		if err != nil {
			return err
		}
		return s.IssueEmailVerification(ctx, tx, p.UserID, emailID, address, name)
	})
}

// VerifyEmail redeems a verification link.
func (s *Service) VerifyEmail(ctx context.Context, token string, meta httpx.ClientMeta) error {
	if err := s.Limiter.Check(ctx, "auth.recovery.ip", meta.IP.String(), limitRecoveryIP); err != nil {
		return err
	}
	if token == "" || len(token) > 128 {
		return ErrInvalidToken
	}
	var user tenant.UserID
	err := db.InTx(ctx, s.Pool, func(tx *db.Tx) error {
		q := store.New(tx)
		tok, err := q.GetAuthTokenForUpdate(ctx, store.GetAuthTokenForUpdateParams{TokenHash: hashToken(token), Purpose: "email_verification"})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidToken
		}
		if err != nil {
			return err
		}
		if tok.UsedAt != nil || !s.Now().Before(tok.ExpiresAt) || tok.UserID == nil {
			return ErrInvalidToken
		}
		var data verificationData
		if err := json.Unmarshal(tok.Data, &data); err != nil || data.EmailID == uuid.Nil {
			return ErrInvalidToken
		}
		user = tenant.UserID(*tok.UserID)
		if err := db.SetScope(ctx, tx, tenant.Scope{UserID: user}); err != nil {
			return err
		}
		if err := s.Accounts.MarkEmailVerified(ctx, tx, user, data.EmailID); err != nil {
			return err
		}
		return q.UseAuthToken(ctx, tok.ID)
	})
	if err != nil {
		return err
	}
	s.event(ctx, store.New(s.Pool), &user, "email.verified", "success", nil, meta, nil)
	return nil
}
