// Package tenant carries the explicit organisation context every tenant operation requires
// (MULTITENANCY.md §2). There is no implicit or global tenant.
package tenant

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// OrgID identifies an organisation. It is a distinct type so it cannot be confused with other
// UUIDs at compile time; construct it only from a resolved membership or a job payload.
type OrgID uuid.UUID

// UserID identifies a global user identity.
type UserID uuid.UUID

func (id OrgID) String() string  { return uuid.UUID(id).String() }
func (id UserID) String() string { return uuid.UUID(id).String() }

// IsZero reports whether the ID is unset.
func (id OrgID) IsZero() bool  { return uuid.UUID(id) == uuid.Nil }
func (id UserID) IsZero() bool { return uuid.UUID(id) == uuid.Nil }

// Scope is the tenant context applied to a database transaction (transaction-local settings
// mc.org_id and mc.user_id, read by row-level security policies).
type Scope struct {
	OrgID  OrgID
	UserID UserID // optional for system jobs
}

// ErrNoTenant is returned when tenant work is attempted without an organisation.
var ErrNoTenant = errors.New("tenant: organisation context is required")

// Validate ensures the scope names an organisation.
func (s Scope) Validate() error {
	if s.OrgID.IsZero() {
		return ErrNoTenant
	}
	return nil
}

type scopeKey struct{}

// WithScope stores the scope on a context.
func WithScope(ctx context.Context, s Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, s)
}

// ScopeFrom returns the scope stored on ctx.
func ScopeFrom(ctx context.Context) (Scope, bool) {
	s, ok := ctx.Value(scopeKey{}).(Scope)
	return s, ok
}

// Role is an organisation role (MULTITENANCY.md §3.1).
type Role string

// Organisation roles, highest first.
const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleGuest  Role = "guest"
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleMember, RoleGuest:
		return true
	}
	return false
}

// Actor is a user acting inside one organisation, resolved from an active membership by the
// organisation-context middleware (MULTITENANCY.md §2.1). Handlers never inspect it; services
// pass it to the authorization policy and open tenant transactions with its scope.
type Actor struct {
	UserID    UserID
	OrgID     OrgID
	Role      Role
	SessionID uuid.UUID
}

// Scope returns the database tenant scope for the actor.
func (a Actor) Scope() Scope { return Scope{OrgID: a.OrgID, UserID: a.UserID} }

type actorKey struct{}

// WithActor stores the actor (and its scope) on a context.
func WithActor(ctx context.Context, a Actor) context.Context {
	return WithScope(context.WithValue(ctx, actorKey{}, a), a.Scope())
}

// ActorFrom returns the actor stored on ctx.
func ActorFrom(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(actorKey{}).(Actor)
	return a, ok
}
