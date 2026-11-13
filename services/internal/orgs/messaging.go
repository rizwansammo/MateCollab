package orgs

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rizwansammo/MateCollab/services/internal/audit"
	"github.com/rizwansammo/MateCollab/services/internal/authz"
	"github.com/rizwansammo/MateCollab/services/internal/orgs/store"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

// Messaging is the Phase 3 part of the organisation's messaging policy.
// Editing and retention policy are introduced with message delivery in Phase 4.
type Messaging struct {
	PublicSpaceCreation  string `json:"public_space_creation"`
	PrivateSpaceCreation string `json:"private_space_creation"`
}

func DefaultMessaging() Messaging {
	return Messaging{PublicSpaceCreation: "all", PrivateSpaceCreation: "all"}
}

func uuidPtr(v uuid.UUID) *uuid.UUID { return &v }

func (m Messaging) validate() error {
	if m.PublicSpaceCreation != "all" && m.PublicSpaceCreation != "admins" {
		return apperr.Validation(apperr.Field("public_space_creation", "invalid", "Choose all or admins."))
	}
	if m.PrivateSpaceCreation != "all" && m.PrivateSpaceCreation != "admins" {
		return apperr.Validation(apperr.Field("private_space_creation", "invalid", "Choose all or admins."))
	}
	return nil
}

func (s *Service) loadMessaging(ctx context.Context, tx *db.Tx, org tenant.OrgID) (Versioned[Messaging], error) {
	row, err := store.New(tx).GetSettingsSection(ctx, store.GetSettingsSectionParams{OrgID: uuid.UUID(org), Section: "messaging"})
	if errors.Is(err, pgx.ErrNoRows) {
		return Versioned[Messaging]{Value: DefaultMessaging(), Version: 0}, nil
	}
	if err != nil {
		return Versioned[Messaging]{}, err
	}
	out := Versioned[Messaging]{Value: DefaultMessaging(), Version: row.Version, UpdatedAt: row.UpdatedAt}
	if err = json.Unmarshal(row.Settings, &out.Value); err != nil {
		return out, err
	}
	return out, nil
}

func (s *Service) GetMessaging(ctx context.Context, a tenant.Actor) (Versioned[Messaging], error) {
	if err := authz.Check(a, authz.SettingsFrom(ctx), authz.OrgSettingsManage); err != nil {
		return Versioned[Messaging]{}, err
	}
	var out Versioned[Messaging]
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		var err error
		out, err = s.loadMessaging(ctx, tx, a.OrgID)
		return err
	})
	return out, err
}

func (s *Service) UpdateMessaging(ctx context.Context, a tenant.Actor, in Messaging, expectedVersion int32, meta httpx.ClientMeta) (Versioned[Messaging], error) {
	if err := authz.Check(a, authz.SettingsFrom(ctx), authz.OrgSettingsManage); err != nil {
		return Versioned[Messaging]{}, err
	}
	if err := in.validate(); err != nil {
		return Versioned[Messaging]{}, err
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return Versioned[Messaging]{}, err
	}
	var out Versioned[Messaging]
	err = db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		previous, err := s.loadMessaging(ctx, tx, a.OrgID)
		if err != nil {
			return err
		}
		if previous.Version != expectedVersion {
			return &apperr.Error{Kind: apperr.KindPreconditionFailed, Code: "version_mismatch", Detail: "Messaging settings changed. Reload and try again."}
		}
		if previous.Version == 0 {
			_, err = tx.Exec(ctx, `INSERT INTO organisation_settings(org_id,section,settings,updated_by)
                VALUES($1,'messaging',$2,$3) ON CONFLICT DO NOTHING`, uuid.UUID(a.OrgID), raw, uuid.UUID(a.UserID))
			if err != nil {
				return err
			}
			// The missing-row case must be rejected if a concurrent writer created it.
			var count int
			err = tx.QueryRow(ctx, `SELECT version FROM organisation_settings WHERE org_id=$1 AND section='messaging'`, uuid.UUID(a.OrgID)).Scan(&count)
			if err != nil {
				return err
			}
			if count != 1 {
				return &apperr.Error{Kind: apperr.KindPreconditionFailed, Code: "version_mismatch", Detail: "Messaging settings changed."}
			}
			out = Versioned[Messaging]{Value: in, Version: 1, UpdatedAt: time.Now()}
		} else {
			row, err := store.New(tx).UpdateSettingsSection(ctx, store.UpdateSettingsSectionParams{
				OrgID: uuid.UUID(a.OrgID), Section: "messaging", Settings: raw, UpdatedBy: uuidPtr(uuid.UUID(a.UserID)), ExpectedVersion: expectedVersion,
			})
			if errors.Is(err, pgx.ErrNoRows) {
				return &apperr.Error{Kind: apperr.KindPreconditionFailed, Code: "version_mismatch", Detail: "Messaging settings changed."}
			}
			if err != nil {
				return err
			}
			out = Versioned[Messaging]{Value: in, Version: row.Version, UpdatedAt: row.UpdatedAt}
		}
		return audit.RecordActor(ctx, tx, a, meta, audit.Entry{
			Action: "org.messaging_updated", TargetType: "organisation", TargetID: a.OrgID.String(),
			Metadata: map[string]any{"public_space_creation": in.PublicSpaceCreation, "private_space_creation": in.PrivateSpaceCreation},
		})
	})
	return out, err
}
