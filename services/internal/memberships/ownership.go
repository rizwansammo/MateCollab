package memberships

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rizwansammo/MateCollab/services/internal/audit"
	"github.com/rizwansammo/MateCollab/services/internal/auth"
	"github.com/rizwansammo/MateCollab/services/internal/authz"
	"github.com/rizwansammo/MateCollab/services/internal/memberships/store"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

var (
	errTransferToSelf = apperr.Conflict("transfer_to_self", "You already own this organisation.")
	errTransferTarget = apperr.Conflict("transfer_target_not_admin",
		"Ownership can only be transferred to an active Admin of this organisation.")
	errTransferInactive = apperr.Conflict("transfer_target_inactive",
		"This Admin's account is not active or is scheduled for deletion.")
)

// TransferOwnership makes an active Admin the Owner and the current Owner an Admin in one
// transaction. Only the Owner may do it, after a recent password check (SECURITY-BASELINE §5).
func (s *Service) TransferOwnership(ctx context.Context, a tenant.Actor, p auth.Principal, target uuid.UUID, meta httpx.ClientMeta) error {
	if err := authz.Check(a, authz.SettingsFrom(ctx), authz.OrgOwnershipTransfer); err != nil {
		return err
	}
	if !p.RecentlyAuthenticated(s.Now()) {
		return auth.ErrReauthRequired
	}
	if tenant.UserID(target) == a.UserID {
		return errTransferToSelf
	}
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		q := store.New(tx)
		rows, err := q.LockMembersForTransfer(ctx, store.LockMembersForTransferParams{
			OrgID: uuid.UUID(a.OrgID), UserIds: []uuid.UUID{uuid.UUID(a.UserID), target},
		})
		if err != nil {
			return err
		}
		var self, to *store.LockMembersForTransferRow
		for i := range rows {
			switch rows[i].UserID {
			case uuid.UUID(a.UserID):
				self = &rows[i]
			case target:
				to = &rows[i]
			}
		}
		// Re-checked under the lock: the cached role may predate a concurrent change.
		if self == nil || self.Role != string(tenant.RoleOwner) {
			return authz.ErrForbidden("forbidden")
		}
		if to == nil {
			return apperr.NotFound()
		}
		if to.Role != string(tenant.RoleAdmin) {
			return errTransferTarget
		}
		if to.Status != "active" || to.UserStatus != "active" || to.DeletionRequestedAt != nil {
			return errTransferInactive
		}
		if err := swapOwner(ctx, q, a.OrgID, a.UserID, tenant.UserID(target)); err != nil {
			return err
		}
		return audit.RecordActor(ctx, tx, a, meta, audit.Entry{
			Action: "org.ownership_transferred", TargetType: "member", TargetID: target.String(),
			Metadata: map[string]any{"from_user_id": a.UserID.String(), "to_user_id": target.String()},
		})
	})
	if err == nil {
		s.invalidate(ctx, a.OrgID, a.UserID)
		s.invalidate(ctx, a.OrgID, tenant.UserID(target))
	}
	return err
}

// swapOwner demotes from (if any) to Admin and makes to the Owner. The one-owner index is
// checked per statement, so the demotion must come first.
func swapOwner(ctx context.Context, q *store.Queries, org tenant.OrgID, from, to tenant.UserID) error {
	if !from.IsZero() {
		if err := q.UpdateMemberRole(ctx, store.UpdateMemberRoleParams{OrgID: uuid.UUID(org), UserID: uuid.UUID(from), Role: string(tenant.RoleAdmin)}); err != nil {
			return err
		}
	}
	if err := q.UpdateMemberRole(ctx, store.UpdateMemberRoleParams{OrgID: uuid.UUID(org), UserID: uuid.UUID(to), Role: string(tenant.RoleOwner)}); err != nil {
		return err
	}
	return q.UpdateMemberStatus(ctx, store.UpdateMemberStatusParams{OrgID: uuid.UUID(org), UserID: uuid.UUID(to), Status: "active"})
}

// AssignOwner makes user the Owner of org from operator tooling (mcctl user create-owner), in
// tx (which carries the organisation's tenant context). The previous Owner, if any, becomes an
// Admin; a user who is not yet a member joins as Owner. It returns the previous Owner.
func (s *Service) AssignOwner(ctx context.Context, tx *db.Tx, org tenant.OrgID, user tenant.UserID, contactEmail string) (tenant.UserID, error) {
	q := store.New(tx)
	var previous tenant.UserID
	owner, err := q.GetOwnerForUpdate(ctx, uuid.UUID(org))
	switch {
	case err == nil:
		previous = tenant.UserID(owner)
	case !errors.Is(err, pgx.ErrNoRows):
		return previous, err
	}
	if previous == user {
		return previous, nil
	}
	if _, err := q.GetMemberForUpdate(ctx, store.GetMemberForUpdateParams{OrgID: uuid.UUID(org), UserID: uuid.UUID(user)}); errors.Is(err, pgx.ErrNoRows) {
		if !previous.IsZero() {
			if err := q.UpdateMemberRole(ctx, store.UpdateMemberRoleParams{OrgID: uuid.UUID(org), UserID: uuid.UUID(previous), Role: string(tenant.RoleAdmin)}); err != nil {
				return previous, err
			}
		}
		if err := s.AddOwner(ctx, tx, org, user, contactEmail); err != nil {
			return previous, err
		}
	} else if err != nil {
		return previous, err
	} else if err := swapOwner(ctx, q, org, previous, user); err != nil {
		return previous, err
	}
	s.invalidate(ctx, org, user)
	if !previous.IsZero() {
		s.invalidate(ctx, org, previous)
	}
	return previous, nil
}
