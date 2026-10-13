// Package users owns global identity data: profile (name, time zone, locale), email addresses
// and personal preferences (BACKEND.md §2). Every query runs with the user's RLS context, so a
// user can only ever read and change their own addresses.
package users

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rizwansammo/MateCollab/services/internal/auth"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
	"github.com/rizwansammo/MateCollab/services/internal/platform/textutil"
	"github.com/rizwansammo/MateCollab/services/internal/users/store"
)

// MaxEmails bounds how many addresses one account may hold.
const MaxEmails = 5

// Verifier issues email verification links (implemented by the auth module).
type Verifier interface {
	IssueEmailVerification(ctx context.Context, tx *db.Tx, user tenant.UserID, emailID uuid.UUID, email, name string) error
}

// Service implements the users use cases.
type Service struct {
	pool     *pgxpool.Pool
	verifier Verifier
	now      func() time.Time
}

// New returns a users service. SetVerifier completes the wiring (auth and users depend on each
// other through narrow interfaces).
func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool, now: time.Now} }

// SetVerifier wires the email verification issuer.
func (s *Service) SetVerifier(v Verifier) { s.verifier = v }

// User is the caller's own profile.
type User struct {
	ID           uuid.UUID
	DisplayName  string
	Timezone     string
	Locale       string
	PrimaryEmail string
	CreatedAt    time.Time
	// DeletionScheduledAt is set while an account deletion request is pending.
	DeletionScheduledAt *time.Time
}

// Email is one of the caller's addresses.
type Email struct {
	ID         uuid.UUID
	Address    string
	IsPrimary  bool
	VerifiedAt *time.Time
	CreatedAt  time.Time
}

// ---- auth.Accounts --------------------------------------------------------------------------

// Profile returns the display name and primary email (tx carries the user's context).
func (s *Service) Profile(ctx context.Context, tx pgx.Tx, user tenant.UserID) (string, string, error) {
	q := store.New(tx)
	u, err := q.GetUser(ctx, uuid.UUID(user))
	if err != nil {
		return "", "", err
	}
	e, err := q.PrimaryEmail(ctx, uuid.UUID(user))
	if err != nil {
		return "", "", err
	}
	return u.DisplayName, e.Email, nil
}

// Email returns one of the user's addresses.
func (s *Service) Email(ctx context.Context, tx pgx.Tx, user tenant.UserID, emailID uuid.UUID) (string, bool, error) {
	e, err := store.New(tx).GetEmail(ctx, store.GetEmailParams{ID: emailID, UserID: uuid.UUID(user)})
	if err != nil {
		return "", false, err
	}
	return e.Email, e.VerifiedAt != nil, nil
}

// MarkEmailVerified records proof of control of an address (idempotent).
func (s *Service) MarkEmailVerified(ctx context.Context, tx pgx.Tx, user tenant.UserID, emailID uuid.UUID) error {
	q := store.New(tx)
	if _, err := q.MarkEmailVerified(ctx, store.MarkEmailVerifiedParams{ID: emailID, UserID: uuid.UUID(user)}); isUniqueViolation(err) {
		// Another account proved ownership of this address first.
		return apperr.Conflict("email_taken", "This email address is already in use by another account.")
	} else if err != nil {
		return err
	}
	if _, err := q.GetEmail(ctx, store.GetEmailParams{ID: emailID, UserID: uuid.UUID(user)}); errors.Is(err, pgx.ErrNoRows) {
		return auth.ErrInvalidToken
	} else if err != nil {
		return err
	}
	return nil
}

// ScheduleDeletion starts the deletion grace period; false when one is already pending.
func (s *Service) ScheduleDeletion(ctx context.Context, tx pgx.Tx, user tenant.UserID, requested, scheduled time.Time) (bool, error) {
	n, err := store.New(tx).ScheduleDeletion(ctx, store.ScheduleDeletionParams{ID: uuid.UUID(user), RequestedAt: &requested, ScheduledAt: &scheduled})
	return n == 1, err
}

// CancelDeletion ends a pending deletion request; false when none was pending.
func (s *Service) CancelDeletion(ctx context.Context, tx pgx.Tx, user tenant.UserID) (bool, error) {
	n, err := store.New(tx).CancelDeletion(ctx, uuid.UUID(user))
	return n == 1, err
}

// HasVerifiedEmail reports whether the user owns a verified (or primary) copy of address.
func (s *Service) HasVerifiedEmail(ctx context.Context, tx pgx.Tx, user tenant.UserID, address string) (bool, error) {
	list, err := store.New(tx).ListEmails(ctx, uuid.UUID(user))
	if err != nil {
		return false, err
	}
	for _, e := range list {
		if strings.EqualFold(e.Email, address) && (e.VerifiedAt != nil || e.IsPrimary) {
			return true, nil
		}
	}
	return false, nil
}

// ---- account creation ------------------------------------------------------------------------

// NewUser describes an account created by invitation acceptance or by an operator.
type NewUser struct {
	DisplayName   string
	Email         string
	EmailVerified bool
	Timezone      string
	Locale        string
}

// ValidateDisplayName cleans and checks a display name.
func ValidateDisplayName(field, name string) (string, error) {
	clean := textutil.CleanName(name)
	if n := textutil.Length(clean); n < 1 || n > 80 {
		return "", apperr.Validation(apperr.Field(field, "invalid_length", "Use between 1 and 80 characters."))
	}
	return clean, nil
}

// Create inserts a user and their primary email inside tx. The transaction's tenant context is
// switched to the new user (keeping org, if any) because the RLS insert policy requires it.
func (s *Service) Create(ctx context.Context, tx *db.Tx, org tenant.OrgID, in NewUser) (tenant.UserID, uuid.UUID, error) {
	name, err := ValidateDisplayName("display_name", in.DisplayName)
	if err != nil {
		return tenant.UserID{}, uuid.Nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return tenant.UserID{}, uuid.Nil, err
	}
	user := tenant.UserID(id)
	if err := db.SetScope(ctx, tx, tenant.Scope{OrgID: org, UserID: user}); err != nil {
		return user, uuid.Nil, err
	}
	tz, locale := in.Timezone, in.Locale
	if tz == "" {
		tz = "UTC"
	}
	if locale == "" {
		locale = "en-GB"
	}
	q := store.New(tx)
	if err := q.CreateUser(ctx, store.CreateUserParams{ID: id, DisplayName: name, Timezone: tz, Locale: locale}); err != nil {
		return user, uuid.Nil, err
	}
	var verified *time.Time
	if in.EmailVerified {
		now := s.now()
		verified = &now
	}
	e, err := q.CreateEmail(ctx, store.CreateEmailParams{UserID: id, Email: strings.TrimSpace(in.Email), IsPrimary: true, VerifiedAt: verified})
	if isUniqueViolation(err) {
		return user, uuid.Nil, apperr.Conflict("email_taken", "An account with this email address already exists.")
	}
	if err != nil {
		return user, uuid.Nil, err
	}
	return user, e.ID, nil
}

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

// ---- profile -----------------------------------------------------------------------------------

// Me returns the caller's profile.
func (s *Service) Me(ctx context.Context, user tenant.UserID) (User, error) {
	var out User
	err := db.InUserTx(ctx, s.pool, user, func(tx *db.Tx) error {
		var err error
		out, err = s.load(ctx, tx, user)
		return err
	})
	return out, err
}

func (s *Service) load(ctx context.Context, tx pgx.Tx, user tenant.UserID) (User, error) {
	q := store.New(tx)
	u, err := q.GetUser(ctx, uuid.UUID(user))
	if err != nil {
		return User{}, err
	}
	out := User{ID: u.ID, DisplayName: u.DisplayName, Timezone: u.Timezone, Locale: u.Locale, CreatedAt: u.CreatedAt,
		DeletionScheduledAt: u.DeletionScheduledAt}
	if e, err := q.PrimaryEmail(ctx, uuid.UUID(user)); err == nil {
		out.PrimaryEmail = e.Email
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return User{}, err
	}
	return out, nil
}

// ProfileUpdate changes the caller's profile; nil fields are unchanged.
type ProfileUpdate struct {
	DisplayName *string
	Timezone    *string
	Locale      *string
}

var localeRe = regexp.MustCompile(`^[a-z]{2,3}(-[A-Z]{2})?$`)

// UpdateMe validates and applies a profile update.
func (s *Service) UpdateMe(ctx context.Context, user tenant.UserID, in ProfileUpdate) (User, error) {
	var out User
	err := db.InUserTx(ctx, s.pool, user, func(tx *db.Tx) error {
		cur, err := s.load(ctx, tx, user)
		if err != nil {
			return err
		}
		var fields []apperr.FieldError
		name, tz, locale := cur.DisplayName, cur.Timezone, cur.Locale
		if in.DisplayName != nil {
			if clean, err := ValidateDisplayName("display_name", *in.DisplayName); err != nil {
				e, _ := apperr.As(err)
				fields = append(fields, e.Fields...)
			} else {
				name = clean
			}
		}
		if in.Timezone != nil {
			if _, err := time.LoadLocation(*in.Timezone); err != nil || *in.Timezone == "" || len(*in.Timezone) > 64 {
				fields = append(fields, apperr.Field("timezone", "invalid", "Choose a valid time zone."))
			} else {
				tz = *in.Timezone
			}
		}
		if in.Locale != nil {
			if !localeRe.MatchString(*in.Locale) {
				fields = append(fields, apperr.Field("locale", "invalid", "Choose a valid language."))
			} else {
				locale = *in.Locale
			}
		}
		if len(fields) > 0 {
			return apperr.Validation(fields...)
		}
		if _, err := store.New(tx).UpdateProfile(ctx, store.UpdateProfileParams{ID: uuid.UUID(user), DisplayName: name, Timezone: tz, Locale: locale}); err != nil {
			return err
		}
		out, err = s.load(ctx, tx, user)
		return err
	})
	return out, err
}

// ---- email addresses --------------------------------------------------------------------------

// ListEmails returns the caller's addresses, primary first.
func (s *Service) ListEmails(ctx context.Context, user tenant.UserID) ([]Email, error) {
	var out []Email
	err := db.InUserTx(ctx, s.pool, user, func(tx *db.Tx) error {
		rows, err := store.New(tx).ListEmails(ctx, uuid.UUID(user))
		for _, r := range rows {
			out = append(out, Email{ID: r.ID, Address: r.Email, IsPrimary: r.IsPrimary, VerifiedAt: r.VerifiedAt, CreatedAt: r.CreatedAt})
		}
		return err
	})
	return out, err
}

// AddEmail adds an unverified address and emails a verification link.
func (s *Service) AddEmail(ctx context.Context, user tenant.UserID, address string) (Email, error) {
	address = strings.TrimSpace(address)
	var out Email
	err := db.InUserTx(ctx, s.pool, user, func(tx *db.Tx) error {
		q := store.New(tx)
		existing, err := q.ListEmails(ctx, uuid.UUID(user))
		if err != nil {
			return err
		}
		if len(existing) >= MaxEmails {
			return apperr.Conflict("email_limit_reached", "You can add up to 5 email addresses.")
		}
		row, err := q.CreateEmail(ctx, store.CreateEmailParams{UserID: uuid.UUID(user), Email: address})
		if isUniqueViolation(err) {
			return apperr.Conflict("email_exists", "You've already added this email address.")
		}
		if err != nil {
			return err
		}
		out = Email{ID: row.ID, Address: row.Email, IsPrimary: row.IsPrimary, VerifiedAt: row.VerifiedAt, CreatedAt: row.CreatedAt}
		u, err := q.GetUser(ctx, uuid.UUID(user))
		if err != nil {
			return err
		}
		return s.verifier.IssueEmailVerification(ctx, tx, user, row.ID, row.Email, u.DisplayName)
	})
	return out, err
}

// RemoveEmail deletes a non-primary address.
func (s *Service) RemoveEmail(ctx context.Context, user tenant.UserID, id uuid.UUID) error {
	return db.InUserTx(ctx, s.pool, user, func(tx *db.Tx) error {
		q := store.New(tx)
		e, err := q.GetEmail(ctx, store.GetEmailParams{ID: id, UserID: uuid.UUID(user)})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if e.IsPrimary {
			return apperr.Conflict("primary_email", "Make another verified address primary before removing this one.")
		}
		_, err = q.DeleteEmail(ctx, store.DeleteEmailParams{ID: id, UserID: uuid.UUID(user)})
		return err
	})
}

// MakePrimary switches the primary address to a verified one (requires step-up).
func (s *Service) MakePrimary(ctx context.Context, p auth.Principal, id uuid.UUID) error {
	if !p.RecentlyAuthenticated(s.now()) {
		return auth.ErrReauthRequired
	}
	return db.InUserTx(ctx, s.pool, p.UserID, func(tx *db.Tx) error {
		q := store.New(tx)
		e, err := q.GetEmail(ctx, store.GetEmailParams{ID: id, UserID: uuid.UUID(p.UserID)})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if e.VerifiedAt == nil {
			return apperr.Conflict("email_not_verified", "Verify this address before making it primary.")
		}
		if err := q.ClearPrimaryEmail(ctx, uuid.UUID(p.UserID)); err != nil {
			return err
		}
		_, err = q.SetPrimaryEmail(ctx, store.SetPrimaryEmailParams{ID: id, UserID: uuid.UUID(p.UserID)})
		return err
	})
}

// ---- preferences ---------------------------------------------------------------------------------

// Preferences is the versioned personal settings document (DATABASE.md §4.1). Only appearance
// is used so far; other keys arrive with their features.
type Preferences struct {
	Theme string `json:"theme"` // light | dark | system
}

func defaultPreferences() Preferences { return Preferences{Theme: "system"} }

// GetPreferences returns the caller's preferences (defaults when never saved).
func (s *Service) GetPreferences(ctx context.Context, user tenant.UserID) (Preferences, error) {
	prefs := defaultPreferences()
	err := db.InUserTx(ctx, s.pool, user, func(tx *db.Tx) error {
		row, err := store.New(tx).GetPreferences(ctx, uuid.UUID(user))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		return json.Unmarshal(row.Prefs, &prefs)
	})
	return prefs, err
}

// UpdatePreferences validates and saves a change.
func (s *Service) UpdatePreferences(ctx context.Context, user tenant.UserID, theme *string) (Preferences, error) {
	prefs, err := s.GetPreferences(ctx, user)
	if err != nil {
		return prefs, err
	}
	if theme != nil {
		switch *theme {
		case "light", "dark", "system":
			prefs.Theme = *theme
		default:
			return prefs, apperr.Validation(apperr.Field("theme", "invalid", "Choose light, dark or system."))
		}
	}
	raw, err := json.Marshal(prefs)
	if err != nil {
		return prefs, err
	}
	err = db.InUserTx(ctx, s.pool, user, func(tx *db.Tx) error {
		return store.New(tx).SavePreferences(ctx, store.SavePreferencesParams{UserID: uuid.UUID(user), Prefs: raw, SchemaVersion: 1})
	})
	return prefs, err
}
