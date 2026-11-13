package collaboration

import (
	"context"

	"github.com/google/uuid"
	"github.com/rizwansammo/MateCollab/services/internal/audit"
	"github.com/rizwansammo/MateCollab/services/internal/authz"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

// DeleteSpace permanently deletes a Space and all its Channels.
// Only organisation Owners and Admins may perform this destructive action.
func (s *Service) DeleteSpace(ctx context.Context, a tenant.Actor, id uuid.UUID, meta httpx.ClientMeta) error {
	if err := authz.Check(a, authz.DefaultSettings(), authz.SpacesManageAny); err != nil {
		return err
	}
	return db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		sp, err := s.space(ctx, tx, a, id)
		if err != nil {
			return err
		}
		if err = authz.CheckSpace(a, spaceFacts(sp), authz.SpaceDiscover); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM conversations WHERE org_id=$1 AND space_id=$2 AND kind='channel'`,
			uuid.UUID(a.OrgID), id); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `DELETE FROM spaces WHERE org_id=$1 AND id=$2`, uuid.UUID(a.OrgID), id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return authz.ErrForbidden("resource_unavailable")
		}
		return audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "space.deleted", TargetType: "space", TargetID: id.String()})
	})
}
