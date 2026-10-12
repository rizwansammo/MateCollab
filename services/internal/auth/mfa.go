package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rizwansammo/MateCollab/services/internal/auth/store"
	"github.com/rizwansammo/MateCollab/services/internal/emails"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/mfa"
)

// MFAStatus summarises the user's second factor.
type MFAStatus struct {
	TOTPEnabled            bool
	RecoveryCodesRemaining int64
}

// MFAStatusFor returns the current second-factor state.
func (s *Service) MFAStatusFor(ctx context.Context, p Principal) (MFAStatus, error) {
	q := store.New(s.Pool)
	factor, err := q.GetTOTPFactor(ctx, uuid.UUID(p.UserID))
	if errors.Is(err, pgx.ErrNoRows) {
		return MFAStatus{}, nil
	}
	if err != nil {
		return MFAStatus{}, err
	}
	if factor.ConfirmedAt == nil {
		return MFAStatus{}, nil
	}
	n, err := q.CountUnusedRecoveryCodes(ctx, uuid.UUID(p.UserID))
	return MFAStatus{TOTPEnabled: true, RecoveryCodesRemaining: n}, err
}

// TOTPSetup is shown once while enrolling.
type TOTPSetup struct {
	Secret     string // base32, for manual entry
	OTPAuthURI string
	QRCode     string // PNG data URL
}

// SetupTOTP starts (or restarts) enrolment with a fresh secret. It is not active until a valid
// code confirms it.
func (s *Service) SetupTOTP(ctx context.Context, p Principal) (TOTPSetup, error) {
	q := store.New(s.Pool)
	if factor, err := q.GetTOTPFactor(ctx, uuid.UUID(p.UserID)); err == nil && factor.ConfirmedAt != nil {
		return TOTPSetup{}, apperr.Conflict("mfa_already_enabled", "Two-step verification is already on.")
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return TOTPSetup{}, err
	}
	secret, err := mfa.NewSecret()
	if err != nil {
		return TOTPSetup{}, err
	}
	sealed, err := s.Keyring.Seal(secret, totpAAD(p.UserID))
	if err != nil {
		return TOTPSetup{}, err
	}
	if err := q.SavePendingTOTP(ctx, store.SavePendingTOTPParams{UserID: uuid.UUID(p.UserID), SecretCiphertext: sealed, KeyID: s.Keyring.ActiveKeyID()}); err != nil {
		return TOTPSetup{}, err
	}
	var email string
	if err := db.InUserTx(ctx, s.Pool, p.UserID, func(tx *db.Tx) error {
		_, e, err := s.Accounts.Profile(ctx, tx, p.UserID)
		email = e
		return err
	}); err != nil {
		return TOTPSetup{}, err
	}
	uri := mfa.URI("MateCollab", email, secret)
	qr, err := mfa.QRDataURL(uri)
	if err != nil {
		return TOTPSetup{}, err
	}
	return TOTPSetup{Secret: mfa.EncodeSecret(secret), OTPAuthURI: uri, QRCode: qr}, nil
}

var errInvalidCode = apperr.Validation(apperr.Field("code", "invalid_code", "That code didn't work. Check the time on your device and try again."))

// EnableTOTP confirms enrolment with a valid code and returns the first set of recovery codes
// (shown once).
func (s *Service) EnableTOTP(ctx context.Context, p Principal, code string, meta httpx.ClientMeta) ([]string, error) {
	var codes []string
	err := db.InUserTx(ctx, s.Pool, p.UserID, func(tx *db.Tx) error {
		q := store.New(tx)
		factor, err := q.GetTOTPFactorForUpdate(ctx, uuid.UUID(p.UserID))
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.Conflict("mfa_setup_required", "Start two-step verification setup first.")
		}
		if err != nil {
			return err
		}
		if factor.ConfirmedAt != nil {
			return apperr.Conflict("mfa_already_enabled", "Two-step verification is already on.")
		}
		secret, _, err := s.Keyring.Open(factor.SecretCiphertext, totpAAD(p.UserID))
		if err != nil {
			return err
		}
		step, ok := mfa.Verify(secret, code, s.Now(), nil)
		if !ok {
			return errInvalidCode
		}
		if err := q.ConfirmTOTP(ctx, store.ConfirmTOTPParams{UserID: uuid.UUID(p.UserID), LastUsedStep: &step}); err != nil {
			return err
		}
		if codes, err = s.replaceRecoveryCodes(ctx, q, p); err != nil {
			return err
		}
		return s.notify(ctx, tx, p.UserID, emails.MFAEnabled, nil)
	})
	if err != nil {
		return nil, err
	}
	s.forget(ctx, p.TokenHash) // the session now reports TOTP as enabled
	s.event(ctx, store.New(s.Pool), &p.UserID, "mfa.enabled", "success", &p.SessionID, meta, nil)
	return codes, nil
}

// DisableTOTP turns two-step verification off (requires step-up).
func (s *Service) DisableTOTP(ctx context.Context, p Principal, meta httpx.ClientMeta) error {
	if !p.RecentlyAuthenticated(s.Now()) {
		return ErrReauthRequired
	}
	err := db.InUserTx(ctx, s.Pool, p.UserID, func(tx *db.Tx) error {
		q := store.New(tx)
		if err := q.DeleteTOTP(ctx, uuid.UUID(p.UserID)); err != nil {
			return err
		}
		if err := q.DeleteRecoveryCodes(ctx, uuid.UUID(p.UserID)); err != nil {
			return err
		}
		return s.notify(ctx, tx, p.UserID, emails.MFADisabled, nil)
	})
	if err != nil {
		return err
	}
	s.forget(ctx, p.TokenHash)
	s.event(ctx, store.New(s.Pool), &p.UserID, "mfa.disabled", "success", &p.SessionID, meta, nil)
	return nil
}

// RegenerateRecoveryCodes replaces all recovery codes (requires step-up and an active factor).
func (s *Service) RegenerateRecoveryCodes(ctx context.Context, p Principal, meta httpx.ClientMeta) ([]string, error) {
	if !p.RecentlyAuthenticated(s.Now()) {
		return nil, ErrReauthRequired
	}
	var codes []string
	err := db.InUserTx(ctx, s.Pool, p.UserID, func(tx *db.Tx) error {
		q := store.New(tx)
		factor, err := q.GetTOTPFactor(ctx, uuid.UUID(p.UserID))
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && factor.ConfirmedAt == nil) {
			return apperr.Conflict("mfa_not_enabled", "Turn on two-step verification first.")
		}
		if err != nil {
			return err
		}
		if codes, err = s.replaceRecoveryCodes(ctx, q, p); err != nil {
			return err
		}
		return s.notify(ctx, tx, p.UserID, emails.RecoveryCodesRegenerated, nil)
	})
	if err != nil {
		return nil, err
	}
	s.event(ctx, store.New(s.Pool), &p.UserID, "mfa.recovery_codes_regenerated", "success", &p.SessionID, meta, nil)
	return codes, nil
}

func (s *Service) replaceRecoveryCodes(ctx context.Context, q *store.Queries, p Principal) ([]string, error) {
	if err := q.DeleteRecoveryCodes(ctx, uuid.UUID(p.UserID)); err != nil {
		return nil, err
	}
	codes, err := mfa.NewRecoveryCodes()
	if err != nil {
		return nil, err
	}
	for _, c := range codes {
		sum, kid, err := s.Keyring.MAC([]byte(mfa.NormalizeRecoveryCode(c)))
		if err != nil {
			return nil, err
		}
		if err := q.InsertRecoveryCode(ctx, store.InsertRecoveryCodeParams{UserID: uuid.UUID(p.UserID), CodeHash: sum, KeyID: kid}); err != nil {
			return nil, err
		}
	}
	return codes, nil
}

// Reauthenticate confirms the password (and a TOTP code when enabled) for step-up.
func (s *Service) Reauthenticate(ctx context.Context, p Principal, pw, code string, meta httpx.ClientMeta) error {
	if err := s.Limiter.Check(ctx, "auth.login.account", p.UserID.String(), limitLoginAccount); err != nil {
		return err
	}
	q := store.New(s.Pool)
	hash, err := q.GetPasswordHash(ctx, uuid.UUID(p.UserID))
	if err != nil {
		return err
	}
	ok, _, err := s.Hasher.Verify(ctx, pw, hash)
	if err != nil {
		return err
	}
	if !ok {
		s.event(ctx, q, &p.UserID, "reauthentication.failed", "failure", &p.SessionID, meta, nil)
		return apperr.Validation(apperr.Field("password", "incorrect", "That password is incorrect."))
	}
	if p.TOTPEnabled {
		if code == "" {
			return apperr.Validation(apperr.Field("code", "required", "Enter the code from your authenticator app."))
		}
		var ok bool
		err := db.InTx(ctx, s.Pool, func(tx *db.Tx) error {
			_, valid, err := s.checkSecondFactor(ctx, store.New(tx), p.UserID, code, "")
			ok = valid
			return err
		})
		if err != nil {
			return err
		}
		if !ok {
			return errInvalidCode
		}
	}
	if err := q.MarkReauthenticated(ctx, store.MarkReauthenticatedParams{ID: p.SessionID, ReauthenticatedAt: s.Now()}); err != nil {
		return err
	}
	s.forget(ctx, p.TokenHash)
	s.event(ctx, q, &p.UserID, "reauthentication.succeeded", "success", &p.SessionID, meta, nil)
	return nil
}
