// Package orgs owns organisations: creation, the organisation profile and its settings sections
// (BACKEND.md §2). Membership and invitations live in internal/memberships; permission
// decisions go through internal/authz.
package orgs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/text/language"

	"github.com/rizwansammo/MateCollab/services/internal/audit"
	"github.com/rizwansammo/MateCollab/services/internal/auth"
	"github.com/rizwansammo/MateCollab/services/internal/authz"
	"github.com/rizwansammo/MateCollab/services/internal/memberships"
	"github.com/rizwansammo/MateCollab/services/internal/orgs/store"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/ratelimit"
	"github.com/rizwansammo/MateCollab/services/internal/platform/redisx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
	"github.com/rizwansammo/MateCollab/services/internal/platform/textutil"
	"github.com/rizwansammo/MateCollab/services/internal/users"
)

var limitCreatePerUser = ratelimit.PerDay(5)

// Deps are the collaborators of the orgs service.
type Deps struct {
	Pool        *pgxpool.Pool
	Redis       *redis.Client
	Auth        *auth.Service
	Users       *users.Service
	Memberships *memberships.Service
	Limiter     *ratelimit.Limiter
	AllowSignup bool // MC_ALLOW_ORG_SIGNUP: self-service organisation creation
	Logger      *slog.Logger
	Now         func() time.Time
}

// Service implements the organisation use cases.
type Service struct{ Deps }

// New returns an orgs service.
func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{Deps: d}
}

// Organisation is the organisation profile.
type Organisation struct {
	ID              uuid.UUID
	Slug            string
	Name            string
	Status          string
	DefaultTimezone string
	DefaultLocale   string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ---- validation -------------------------------------------------------------------------------

var slugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$`)

// Reserved slugs would collide with top-level web routes (/{org}/… shares the root with the
// public pages) or invite confusion.
var reservedSlugs = map[string]bool{
	"admin": true, "api": true, "app": true, "assets": true, "brand": true, "favicon": true, "forgot-password": true,
	"healthz": true, "help": true, "invite": true, "login": true, "logout": true, "matecollab": true, "metrics": true,
	"new": true, "readyz": true, "reset-password": true, "root": true, "security": true, "select-organisation": true,
	"settings": true, "signup": true, "static": true, "status": true, "support": true, "system": true, "verify-email": true,
	"www": true,
}

// ValidateName cleans and checks an organisation name.
func ValidateName(name string) (string, error) {
	clean := textutil.CleanName(name)
	if n := textutil.Length(clean); n < 1 || n > 100 {
		return "", apperr.Validation(apperr.Field("name", "invalid_length", "Use between 1 and 100 characters."))
	}
	return clean, nil
}

// ValidateSlug checks an organisation's URL name.
func ValidateSlug(slug string) error {
	switch {
	case !slugPattern.MatchString(slug) || strings.Contains(slug, "--"):
		return apperr.Validation(apperr.Field("slug", "invalid",
			"Use 3 to 40 lowercase letters, numbers and single hyphens, starting and ending with a letter or number."))
	case reservedSlugs[slug]:
		return apperr.Validation(apperr.Field("slug", "reserved", "This address is reserved. Choose another."))
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

var errSlugTaken = apperr.Conflict("slug_taken", "This organisation address is already in use.")

// ---- creation ---------------------------------------------------------------------------------

func (s *Service) insertOrganisation(ctx context.Context, tx *db.Tx, org tenant.OrgID, slug, name string, by *uuid.UUID) error {
	q := store.New(tx)
	if err := q.CreateOrganisation(ctx, store.CreateOrganisationParams{OrgID: uuid.UUID(org), Slug: slug, Name: name}); isUniqueViolation(err) {
		return errSlugTaken
	} else if err != nil {
		return err
	}
	raw, err := json.Marshal(DefaultSecurity())
	if err != nil {
		return err
	}
	return q.CreateSettingsSection(ctx, store.CreateSettingsSectionParams{OrgID: uuid.UUID(org), Section: "security", Settings: raw, UpdatedBy: by})
}

// Create is self-service organisation creation by a signed-in user, who becomes the Owner. It
// is disabled unless MC_ALLOW_ORG_SIGNUP is set.
func (s *Service) Create(ctx context.Context, p auth.Principal, name, slug string, meta httpx.ClientMeta) (Organisation, error) {
	if !s.AllowSignup {
		return Organisation{}, apperr.Forbidden("org_signup_disabled", "Creating organisations is not enabled on this server.")
	}
	clean, err := ValidateName(name)
	if err != nil {
		return Organisation{}, err
	}
	if err := ValidateSlug(slug); err != nil {
		return Organisation{}, err
	}
	if err := s.Limiter.Check(ctx, "orgs.create", p.UserID.String(), limitCreatePerUser); err != nil {
		return Organisation{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Organisation{}, err
	}
	org := tenant.OrgID(id)
	a := tenant.Actor{OrgID: org, UserID: p.UserID, Role: tenant.RoleOwner, SessionID: p.SessionID}
	var out Organisation
	err = db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		uid := uuid.UUID(p.UserID)
		if err := s.insertOrganisation(ctx, tx, org, slug, clean, &uid); err != nil {
			return err
		}
		_, email, err := s.Users.Profile(ctx, tx, p.UserID)
		if err != nil {
			return err
		}
		if err := s.Memberships.AddOwner(ctx, tx, org, p.UserID, email); err != nil {
			return err
		}
		if err := audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "org.created", TargetType: "organisation", TargetID: id.String()}); err != nil {
			return err
		}
		out, err = s.get(ctx, tx, org)
		return err
	})
	return out, err
}

// OwnerSpec identifies the first Owner of an operator-created organisation.
type OwnerSpec struct {
	Email       string
	DisplayName string // used only when the account is created
}

// Created reports the outcome of CreateWithOwner.
type Created struct {
	Organisation Organisation
	OwnerID      tenant.UserID
	NewAccount   bool
	SetupLink    string // set when a new account was created: the emailed set-your-password link
}

// CreateWithOwner creates an organisation from operator tooling (mcctl org create). An existing
// account becomes the Owner; otherwise an account is created and a set-your-password link is
// emailed (and returned so the operator can deliver it if email is not configured).
func (s *Service) CreateWithOwner(ctx context.Context, name, slug string, owner OwnerSpec) (Created, error) {
	clean, err := ValidateName(name)
	if err != nil {
		return Created{}, err
	}
	if err := ValidateSlug(slug); err != nil {
		return Created{}, err
	}
	email, existing, found, err := s.lookupOwner(ctx, owner)
	if err != nil {
		return Created{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Created{}, err
	}
	org := tenant.OrgID(id)
	res := Created{OwnerID: existing, NewAccount: !found}
	err = db.InTenantTx(ctx, s.Pool, tenant.Scope{OrgID: org, UserID: existing}, func(tx *db.Tx) error {
		if err := s.insertOrganisation(ctx, tx, org, slug, clean, nil); err != nil {
			return err
		}
		if !found {
			if res.OwnerID, res.SetupLink, err = s.createOwnerAccount(ctx, tx, org, email, owner.DisplayName, clean); err != nil {
				return err
			}
		}
		if err := s.Memberships.AddOwner(ctx, tx, org, res.OwnerID, email); err != nil {
			return err
		}
		if err := audit.Record(ctx, tx, org, tenant.UserID{}, uuid.Nil, httpx.ClientMeta{}, audit.Entry{
			Action: "org.created", TargetType: "organisation", TargetID: id.String(),
			Metadata: map[string]any{"via": "mcctl", "owner_user_id": res.OwnerID.String()},
		}); err != nil {
			return err
		}
		res.Organisation, err = s.get(ctx, tx, org)
		return err
	})
	if err != nil {
		return Created{}, err
	}
	return res, nil
}

// Assigned reports the outcome of AssignOwner.
type Assigned struct {
	Organisation  Organisation
	OwnerID       tenant.UserID
	PreviousOwner tenant.UserID // zero when the organisation had no Owner
	NewAccount    bool
	SetupLink     string
}

// AssignOwner makes an account the Owner of an existing organisation from operator tooling
// (mcctl user create-owner): bootstrap after an import, or recovery when the Owner has lost
// access. A new account gets a set-your-password link, never a password. The previous Owner
// stays as an Admin. Use the in-app ownership transfer whenever the Owner can sign in.
func (s *Service) AssignOwner(ctx context.Context, slug string, owner OwnerSpec) (Assigned, error) {
	email, existing, found, err := s.lookupOwner(ctx, owner)
	if err != nil {
		return Assigned{}, err
	}
	// A narrow definer lookup: operator tooling has no tenant context to read organisations.
	var id *uuid.UUID
	if err := s.Pool.QueryRow(ctx, "SELECT mc_org_id_by_slug($1)", strings.TrimSpace(slug)).Scan(&id); err != nil {
		return Assigned{}, err
	}
	if id == nil {
		return Assigned{}, apperr.Validation(apperr.Field("org", "not_found", "No organisation has this address."))
	}
	org := tenant.OrgID(*id)
	res := Assigned{OwnerID: existing, NewAccount: !found}
	err = db.InTenantTx(ctx, s.Pool, tenant.Scope{OrgID: org, UserID: existing}, func(tx *db.Tx) error {
		current, err := s.get(ctx, tx, org)
		if err != nil {
			return err
		}
		if !found {
			if res.OwnerID, res.SetupLink, err = s.createOwnerAccount(ctx, tx, org, email, owner.DisplayName, current.Name); err != nil {
				return err
			}
		}
		if res.PreviousOwner, err = s.Memberships.AssignOwner(ctx, tx, org, res.OwnerID, email); err != nil {
			return err
		}
		if res.PreviousOwner != res.OwnerID {
			if err := audit.Record(ctx, tx, org, tenant.UserID{}, uuid.Nil, httpx.ClientMeta{}, audit.Entry{
				Action: "org.ownership_transferred", TargetType: "member", TargetID: res.OwnerID.String(),
				Metadata: map[string]any{"via": "mcctl", "from_user_id": res.PreviousOwner.String(), "to_user_id": res.OwnerID.String()},
			}); err != nil {
				return err
			}
		}
		res.Organisation = current
		return nil
	})
	if err != nil {
		return Assigned{}, err
	}
	return res, nil
}

func (s *Service) lookupOwner(ctx context.Context, owner OwnerSpec) (string, tenant.UserID, bool, error) {
	email := strings.TrimSpace(owner.Email)
	if at := strings.LastIndex(email, "@"); len(email) < 3 || len(email) > 254 || at <= 0 || at == len(email)-1 || strings.ContainsAny(email, " \r\n\t<>") {
		return "", tenant.UserID{}, false, apperr.Validation(apperr.Field("owner_email", "invalid", "Enter a valid email address."))
	}
	existing, found, err := s.Auth.LookupAccount(ctx, email)
	return email, existing, found, err
}

// createOwnerAccount creates the account (tx switches to its user context) and issues the
// emailed set-your-password link, which also verifies the address.
func (s *Service) createOwnerAccount(ctx context.Context, tx *db.Tx, org tenant.OrgID, email, displayName, orgName string) (tenant.UserID, string, error) {
	user, emailID, err := s.Users.Create(ctx, tx, org, users.NewUser{DisplayName: displayName, Email: email})
	if err != nil {
		return user, "", err
	}
	name, _, err := s.Users.Profile(ctx, tx, user)
	if err != nil {
		return user, "", err
	}
	link, err := s.Auth.IssueAccountSetup(ctx, tx, user, emailID, email, name, orgName)
	return user, link, err
}

// ---- profile ----------------------------------------------------------------------------------

func (s *Service) get(ctx context.Context, tx *db.Tx, org tenant.OrgID) (Organisation, error) {
	r, err := store.New(tx).GetOrganisation(ctx, uuid.UUID(org))
	if errors.Is(err, pgx.ErrNoRows) {
		return Organisation{}, apperr.NotFound()
	}
	if err != nil {
		return Organisation{}, err
	}
	return Organisation{ID: r.ID, Slug: r.Slug, Name: r.Name, Status: r.Status, DefaultTimezone: r.DefaultTimezone,
		DefaultLocale: r.DefaultLocale, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}, nil
}

// Get returns the organisation profile to any member.
func (s *Service) Get(ctx context.Context, a tenant.Actor) (Organisation, error) {
	if err := authz.Check(a, authz.SettingsFrom(ctx), authz.OrgView); err != nil {
		return Organisation{}, err
	}
	var out Organisation
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		var err error
		out, err = s.get(ctx, tx, a.OrgID)
		return err
	})
	return out, err
}

// ProfileUpdate changes the organisation profile (Admin → General); nil fields are unchanged.
type ProfileUpdate struct {
	Name            *string
	DefaultTimezone *string
	DefaultLocale   *string
}

// Update changes the organisation profile. With ifMatch (the ETag the client saw) a concurrent
// change answers 412 instead of being overwritten.
func (s *Service) Update(ctx context.Context, a tenant.Actor, in ProfileUpdate, ifMatch *time.Time, meta httpx.ClientMeta) (Organisation, error) {
	if err := authz.Check(a, authz.SettingsFrom(ctx), authz.OrgSettingsManage); err != nil {
		return Organisation{}, err
	}
	var out Organisation
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		before, err := s.get(ctx, tx, a.OrgID)
		if err != nil {
			return err
		}
		next := store.UpdateOrganisationProfileParams{OrgID: uuid.UUID(a.OrgID), Name: before.Name,
			DefaultTimezone: before.DefaultTimezone, DefaultLocale: before.DefaultLocale, ExpectedUpdatedAt: ifMatch}
		var fields []apperr.FieldError
		if in.Name != nil {
			clean, err := ValidateName(*in.Name)
			if e, ok := apperr.As(err); ok {
				fields = append(fields, e.Fields...)
			}
			next.Name = clean
		}
		if in.DefaultTimezone != nil {
			if _, err := time.LoadLocation(*in.DefaultTimezone); err != nil || *in.DefaultTimezone == "" || *in.DefaultTimezone == "Local" || len(*in.DefaultTimezone) > 64 {
				fields = append(fields, apperr.Field("default_timezone", "invalid", "Choose a time zone from the list."))
			}
			next.DefaultTimezone = *in.DefaultTimezone
		}
		if in.DefaultLocale != nil {
			tag, err := language.Parse(*in.DefaultLocale)
			if err != nil || len(*in.DefaultLocale) > 35 {
				fields = append(fields, apperr.Field("default_locale", "invalid", "Choose a language from the list."))
			} else {
				next.DefaultLocale = tag.String()
			}
		}
		if len(fields) > 0 {
			return apperr.Validation(fields...)
		}
		r, err := store.New(tx).UpdateOrganisationProfile(ctx, next)
		if errors.Is(err, pgx.ErrNoRows) {
			return errVersionMismatch
		}
		if err != nil {
			return err
		}
		out = Organisation{ID: r.ID, Slug: r.Slug, Name: r.Name, Status: r.Status, DefaultTimezone: r.DefaultTimezone,
			DefaultLocale: r.DefaultLocale, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
		var changed []string
		if out.Name != before.Name {
			changed = append(changed, "name")
		}
		if out.DefaultTimezone != before.DefaultTimezone {
			changed = append(changed, "default_timezone")
		}
		if out.DefaultLocale != before.DefaultLocale {
			changed = append(changed, "default_locale")
		}
		if len(changed) == 0 {
			return nil
		}
		return audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "org.updated", TargetType: "organisation",
			TargetID: a.OrgID.String(), Metadata: map[string]any{"fields": changed}})
	})
	if err == nil {
		s.Memberships.InvalidateOrganisation(ctx, a.OrgID)
	}
	return out, err
}

// ---- security settings ------------------------------------------------------------------------

// Security is the organisation's `security` settings section (DATABASE.md §4.2).
type Security struct {
	Require2FA        bool   `json:"require_2fa"`
	SessionMaxAge     string `json:"session_max_age"`
	AllowGuestInvites bool   `json:"allow_guest_invites"`
	InvitePermission  string `json:"invite_permission"`
}

// DefaultSecurity is the section's initial value.
func DefaultSecurity() Security {
	return Security{SessionMaxAge: "30d", AllowGuestInvites: true, InvitePermission: "admins"}
}

var sessionMaxAges = map[string]time.Duration{"12h": 12 * time.Hour, "24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour}

// MaxAge is the absolute session age accepted for this organisation.
func (s Security) MaxAge() time.Duration {
	if d, ok := sessionMaxAges[s.SessionMaxAge]; ok {
		return d
	}
	return auth.SessionAbsolute
}

// Authz returns the settings that refine permissions.
func (s Security) Authz() authz.Settings {
	return authz.Settings{InvitePermission: s.InvitePermission, AllowGuestInvites: s.AllowGuestInvites}
}

func (s Security) validate() error {
	var fields []apperr.FieldError
	if _, ok := sessionMaxAges[s.SessionMaxAge]; !ok {
		fields = append(fields, apperr.Field("session_max_age", "invalid", "Choose 12h, 24h, 7d or 30d."))
	}
	if s.InvitePermission != "admins" && s.InvitePermission != "members" {
		fields = append(fields, apperr.Field("invite_permission", "invalid", "Choose admins or members."))
	}
	if len(fields) > 0 {
		return apperr.Validation(fields...)
	}
	return nil
}

// Versioned is a settings section with its optimistic-concurrency version.
type Versioned[T any] struct {
	Value     T
	Version   int32
	UpdatedAt time.Time
}

const securityCacheTTL = 60 * time.Second

func securityKey(org tenant.OrgID) string {
	return redisx.Key("org", org.String(), "settings", "security")
}

func (s *Service) loadSecurity(ctx context.Context, q *store.Queries, org tenant.OrgID) (Versioned[Security], error) {
	row, err := q.GetSettingsSection(ctx, store.GetSettingsSectionParams{OrgID: uuid.UUID(org), Section: "security"})
	if errors.Is(err, pgx.ErrNoRows) {
		return Versioned[Security]{Value: DefaultSecurity()}, nil
	}
	if err != nil {
		return Versioned[Security]{}, err
	}
	v := Versioned[Security]{Value: DefaultSecurity(), Version: row.Version, UpdatedAt: row.UpdatedAt}
	if err := json.Unmarshal(row.Settings, &v.Value); err != nil {
		return Versioned[Security]{}, fmt.Errorf("security settings for %s: %w", org, err)
	}
	return v, nil
}

// SecurityFor returns the organisation's security settings for request handling (cached
// briefly; invalidated on every change). The caller has already resolved membership.
func (s *Service) SecurityFor(ctx context.Context, org tenant.OrgID) (Security, error) {
	if raw, err := s.Redis.Get(ctx, securityKey(org)).Bytes(); err == nil {
		var v Security
		if json.Unmarshal(raw, &v) == nil {
			return v, nil
		}
	}
	var out Versioned[Security]
	err := db.InTenantTx(ctx, s.Pool, tenant.Scope{OrgID: org}, func(tx *db.Tx) error {
		var err error
		out, err = s.loadSecurity(ctx, store.New(tx), org)
		return err
	})
	if err != nil {
		return Security{}, err
	}
	if raw, err := json.Marshal(out.Value); err == nil {
		_ = s.Redis.Set(ctx, securityKey(org), raw, securityCacheTTL).Err()
	}
	return out.Value, nil
}

// GetSecurity returns the section for the admin settings page.
func (s *Service) GetSecurity(ctx context.Context, a tenant.Actor) (Versioned[Security], error) {
	if err := authz.Check(a, authz.SettingsFrom(ctx), authz.OrgSecurityManage); err != nil {
		return Versioned[Security]{}, err
	}
	var out Versioned[Security]
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		var err error
		out, err = s.loadSecurity(ctx, store.New(tx), a.OrgID)
		return err
	})
	return out, err
}

var errVersionMismatch = &apperr.Error{Kind: apperr.KindPreconditionFailed, Code: "version_mismatch",
	Detail: "These settings were changed by someone else. Reload and try again."}

var errOwnMFAFirst = apperr.Conflict("mfa_required_for_policy",
	"Turn on two-step verification for your own account before requiring it for everyone.")

// UpdateSecurity replaces the section if it is still at expectedVersion. actorHasTOTP guards
// against an administrator requiring two-step verification without having it, which would lock
// them out of the organisation they are configuring.
func (s *Service) UpdateSecurity(ctx context.Context, a tenant.Actor, in Security, expectedVersion int32, actorHasTOTP bool, meta httpx.ClientMeta) (Versioned[Security], error) {
	if err := authz.Check(a, authz.SettingsFrom(ctx), authz.OrgSecurityManage); err != nil {
		return Versioned[Security]{}, err
	}
	if err := in.validate(); err != nil {
		return Versioned[Security]{}, err
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return Versioned[Security]{}, err
	}
	var out Versioned[Security]
	err = db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		q := store.New(tx)
		before, err := s.loadSecurity(ctx, q, a.OrgID)
		if err != nil {
			return err
		}
		if in.Require2FA && !before.Value.Require2FA && !actorHasTOTP {
			return errOwnMFAFirst
		}
		uid := uuid.UUID(a.UserID)
		row, err := q.UpdateSettingsSection(ctx, store.UpdateSettingsSectionParams{
			OrgID: uuid.UUID(a.OrgID), Section: "security", Settings: raw, UpdatedBy: &uid, ExpectedVersion: expectedVersion,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return errVersionMismatch
		}
		if err != nil {
			return err
		}
		out = Versioned[Security]{Value: in, Version: row.Version, UpdatedAt: row.UpdatedAt}
		return audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "org.security_updated", TargetType: "organisation",
			TargetID: a.OrgID.String(), Metadata: map[string]any{"changed": securityDiff(before.Value, in)}})
	})
	if err == nil {
		_ = s.Redis.Del(context.WithoutCancel(ctx), securityKey(a.OrgID)).Err()
	}
	return out, err
}

func securityDiff(a, b Security) map[string]any {
	d := map[string]any{}
	if a.Require2FA != b.Require2FA {
		d["require_2fa"] = b.Require2FA
	}
	if a.SessionMaxAge != b.SessionMaxAge {
		d["session_max_age"] = b.SessionMaxAge
	}
	if a.AllowGuestInvites != b.AllowGuestInvites {
		d["allow_guest_invites"] = b.AllowGuestInvites
	}
	if a.InvitePermission != b.InvitePermission {
		d["invite_permission"] = b.InvitePermission
	}
	return d
}
