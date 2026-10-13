package memberships

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/rizwansammo/MateCollab/services/internal/audit"
	"github.com/rizwansammo/MateCollab/services/internal/memberships/store"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
	"github.com/rizwansammo/MateCollab/services/internal/platform/textutil"
)

// Optional is a JSON Merge Patch field: Set when present, Value nil when sent as null.
type Optional struct {
	Set   bool
	Value *string
}

// OwnProfile is a member's change to their own membership in one organisation. Role, status
// and organisation are deliberately absent: they are managed only by administrators.
type OwnProfile struct {
	Title        Optional // null or "" clears it
	ContactEmail Optional // must be one of the member's verified addresses
}

// UpdateOwnMembership changes the caller's own title and contact email in this organisation.
func (s *Service) UpdateOwnMembership(ctx context.Context, a tenant.Actor, in OwnProfile, meta httpx.ClientMeta) (Member, error) {
	var fields []apperr.FieldError
	var title *string
	if in.Title.Set && in.Title.Value != nil {
		if clean := textutil.CleanName(*in.Title.Value); clean != "" {
			if textutil.Length(clean) > 100 {
				fields = append(fields, apperr.Field("title", "too_long", "Use 100 characters or fewer."))
			}
			title = &clean
		}
	}
	var contact string
	if in.ContactEmail.Set {
		if in.ContactEmail.Value == nil || strings.TrimSpace(*in.ContactEmail.Value) == "" {
			fields = append(fields, apperr.Field("contact_email", "required", "Choose one of your confirmed email addresses."))
		} else {
			contact = strings.TrimSpace(*in.ContactEmail.Value)
		}
	}
	if len(fields) > 0 {
		return Member{}, apperr.Validation(fields...)
	}
	var out Member
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		cur, err := s.getMember(ctx, tx, a.OrgID, uuid.UUID(a.UserID))
		if err != nil {
			return err
		}
		next := store.UpdateOwnMembershipProfileParams{OrgID: uuid.UUID(a.OrgID), UserID: uuid.UUID(a.UserID), Title: cur.Title, ContactEmail: cur.Email}
		var changed []string
		if in.Title.Set {
			next.Title = title
			changed = append(changed, "title")
		}
		if in.ContactEmail.Set {
			ok, err := s.Users.HasVerifiedEmail(ctx, tx, a.UserID, contact)
			if err != nil {
				return err
			}
			if !ok {
				return apperr.Validation(apperr.Field("contact_email", "not_verified", "Choose one of your confirmed email addresses."))
			}
			next.ContactEmail = &contact
			changed = append(changed, "contact_email")
		}
		if len(changed) > 0 {
			if err := store.New(tx).UpdateOwnMembershipProfile(ctx, next); err != nil {
				return err
			}
			if err := audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "member.profile_updated", TargetType: "member",
				TargetID: a.UserID.String(), Metadata: map[string]any{"fields": changed}}); err != nil {
				return err
			}
		}
		out, err = s.getMember(ctx, tx, a.OrgID, uuid.UUID(a.UserID))
		return err
	})
	return out, err
}
