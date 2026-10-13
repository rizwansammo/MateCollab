package auth

import (
	"context"
	"strings"
	"time"

	"github.com/rizwansammo/MateCollab/services/internal/auth/store"
	"github.com/rizwansammo/MateCollab/services/internal/emails"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

// DeletionGrace is how long a deletion request can be cancelled before the account is
// anonymised (the same window as organisation deletion).
const DeletionGrace = 30 * 24 * time.Hour

// Ownership reports the organisations a user owns (implemented by the memberships module).
type Ownership interface {
	OwnedOrganisations(ctx context.Context, user tenant.UserID) ([]string, error)
}

// SetOwnership wires the ownership check (memberships depends on auth, not the reverse).
func (s *Service) SetOwnership(o Ownership) { s.ownership = o }

var (
	errDeletionPending = apperr.Conflict("deletion_already_requested", "Your account is already scheduled for deletion.")
	errNoDeletion      = apperr.Conflict("no_deletion_requested", "Your account is not scheduled for deletion.")
)

// RequestDeletion schedules the caller's account for anonymisation after the grace period. It
// needs a recent password check, and an Owner must transfer ownership first so no
// organisation is left without one.
func (s *Service) RequestDeletion(ctx context.Context, p Principal, meta httpx.ClientMeta) (time.Time, error) {
	now := s.Now()
	if !p.RecentlyAuthenticated(now) {
		return time.Time{}, ErrReauthRequired
	}
	owned, err := s.ownership.OwnedOrganisations(ctx, p.UserID)
	if err != nil {
		return time.Time{}, err
	}
	if len(owned) > 0 {
		return time.Time{}, apperr.Conflict("ownership_transfer_required",
			"You own "+strings.Join(owned, ", ")+". Transfer ownership to an Admin before deleting your account.")
	}
	at := now.Add(DeletionGrace)
	err = db.InUserTx(ctx, s.Pool, p.UserID, func(tx *db.Tx) error {
		ok, err := s.Accounts.ScheduleDeletion(ctx, tx, p.UserID, now, at)
		if err != nil {
			return err
		}
		if !ok {
			return errDeletionPending
		}
		return s.notify(ctx, tx, p.UserID, emails.AccountDeletionRequested, map[string]string{"date": at.UTC().Format("2 January 2006")})
	})
	if err != nil {
		return time.Time{}, err
	}
	s.event(ctx, store.New(s.Pool), &p.UserID, "account.deletion_requested", "success", &p.SessionID, meta,
		map[string]any{"scheduled_at": at.UTC().Format(time.RFC3339)})
	return at, nil
}

// CancelDeletion withdraws a pending deletion request.
func (s *Service) CancelDeletion(ctx context.Context, p Principal, meta httpx.ClientMeta) error {
	err := db.InUserTx(ctx, s.Pool, p.UserID, func(tx *db.Tx) error {
		ok, err := s.Accounts.CancelDeletion(ctx, tx, p.UserID)
		if err != nil {
			return err
		}
		if !ok {
			return errNoDeletion
		}
		return s.notify(ctx, tx, p.UserID, emails.AccountDeletionCancelled, nil)
	})
	if err != nil {
		return err
	}
	s.event(ctx, store.New(s.Pool), &p.UserID, "account.deletion_cancelled", "success", &p.SessionID, meta, nil)
	return nil
}
