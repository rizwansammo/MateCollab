package orgs

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rizwansammo/MateCollab/services/internal/auth"
	"github.com/rizwansammo/MateCollab/services/internal/authz"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

// Handler serves organisation routes.
type Handler struct{ svc *Service }

// NewHandler returns the orgs HTTP handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// SessionRoutes registers routes that need a session but no organisation.
func (h *Handler) SessionRoutes(r chi.Router) {
	r.Post("/orgs", h.create)
}

// OrgRoutes registers tenant routes (mounted at /api/v1/orgs/{orgId}).
func (h *Handler) OrgRoutes(r chi.Router) {
	r.Get("/", h.get)
	r.Patch("/", h.update)
	r.Get("/roles", h.roles)
	r.Get("/settings/security", h.getSecurity)
	r.Get("/settings/messaging", h.getMessaging)
	r.Put("/settings/messaging", h.putMessaging)
	r.Put("/settings/security", h.putSecurity)
}

type organisationJSON struct {
	ID              uuid.UUID `json:"id"`
	Slug            string    `json:"slug"`
	Name            string    `json:"name"`
	Status          string    `json:"status"`
	DefaultTimezone string    `json:"default_timezone"`
	DefaultLocale   string    `json:"default_locale"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func toJSON(o Organisation) organisationJSON {
	return organisationJSON{ID: o.ID, Slug: o.Slug, Name: o.Name, Status: o.Status, DefaultTimezone: o.DefaultTimezone,
		DefaultLocale: o.DefaultLocale, CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt}
}

func actor(r *http.Request) tenant.Actor {
	a, _ := tenant.ActorFrom(r.Context())
	return a
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	p, _ := auth.PrincipalFrom(r.Context())
	o, err := h.svc.Create(r.Context(), p, req.Name, strings.ToLower(strings.TrimSpace(req.Slug)), httpx.ClientMetaFrom(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/orgs/"+o.ID.String())
	httpx.WriteJSON(w, http.StatusCreated, toJSON(o))
}

// profileETag identifies a version of the organisation profile (its last change, µs).
func profileETag(o Organisation) string {
	return `"` + strconv.FormatInt(o.UpdatedAt.UnixMicro(), 10) + `"`
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	o, err := h.svc.Get(r.Context(), actor(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.Header().Set("ETag", profileETag(o))
	httpx.WriteJSON(w, http.StatusOK, toJSON(o))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name            *string `json:"name"`
		DefaultTimezone *string `json:"default_timezone"`
		DefaultLocale   *string `json:"default_locale"`
	}
	// If-Match is optional here (older clients rename without it); when sent it must match.
	var ifMatch *time.Time
	if v := strings.Trim(strings.TrimPrefix(r.Header.Get("If-Match"), "W/"), `"`); v != "" {
		us, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			httpx.WriteProblem(w, r, errVersionMismatch)
			return
		}
		t := time.UnixMicro(us)
		ifMatch = &t
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	o, err := h.svc.Update(r.Context(), actor(r), ProfileUpdate{Name: req.Name, DefaultTimezone: req.DefaultTimezone, DefaultLocale: req.DefaultLocale}, ifMatch, httpx.ClientMetaFrom(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.Header().Set("ETag", profileETag(o))
	httpx.WriteJSON(w, http.StatusOK, toJSON(o))
}

// roles describes the fixed role model with this organisation's effective permissions, straight
// from the authz catalogue (the UI keeps no copy of the rules).
func (h *Handler) roles(w http.ResponseWriter, r *http.Request) {
	a := actor(r)
	if err := authz.Check(a, authz.SettingsFrom(r.Context()), authz.OrgView); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	settings := authz.SettingsFrom(r.Context())
	type permissionJSON struct {
		ID          authz.Permission `json:"id"`
		Description string           `json:"description"`
		DependsOn   string           `json:"depends_on,omitempty"`
	}
	type roleJSON struct {
		Role        tenant.Role        `json:"role"`
		Permissions []authz.Permission `json:"permissions"`
	}
	catalogue := authz.Catalogue()
	perms := make([]permissionJSON, len(catalogue))
	for i, p := range catalogue {
		perms[i] = permissionJSON{ID: p.ID, Description: p.Description, DependsOn: p.DependsOn}
	}
	var roles []roleJSON
	for _, role := range []tenant.Role{tenant.RoleOwner, tenant.RoleAdmin, tenant.RoleMember, tenant.RoleGuest} {
		granted := []authz.Permission{}
		for _, p := range catalogue {
			if authz.Allowed(role, settings, p.ID) {
				granted = append(granted, p.ID)
			}
		}
		roles = append(roles, roleJSON{Role: role, Permissions: granted})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"permissions": perms,
		"roles":       roles,
		"settings":    map[string]any{"invite_permission": settings.InvitePermission, "allow_guest_invites": settings.AllowGuestInvites},
	})
}

type securityJSON struct {
	Security
	Version   int32     `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

func etag(version int32) string { return `"` + strconv.Itoa(int(version)) + `"` }

func writeSecurity(w http.ResponseWriter, v Versioned[Security]) {
	w.Header().Set("ETag", etag(v.Version))
	httpx.WriteJSON(w, http.StatusOK, securityJSON{Security: v.Value, Version: v.Version, UpdatedAt: v.UpdatedAt})
}

func (h *Handler) getSecurity(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.GetSecurity(r.Context(), actor(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	writeSecurity(w, v)
}

var errIfMatchRequired = &apperr.Error{Kind: apperr.KindPreconditionRequired, Code: "precondition_required",
	Detail: "Send If-Match with the settings version you are changing."}

func (h *Handler) putSecurity(w http.ResponseWriter, r *http.Request) {
	match := strings.Trim(strings.TrimPrefix(r.Header.Get("If-Match"), "W/"), `"`)
	version, err := strconv.ParseInt(match, 10, 32)
	if match == "" || err != nil {
		httpx.WriteProblem(w, r, errIfMatchRequired)
		return
	}
	var req Security
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	p, _ := auth.PrincipalFrom(r.Context())
	v, err := h.svc.UpdateSecurity(r.Context(), actor(r), req, int32(version), p.TOTPEnabled, httpx.ClientMetaFrom(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	writeSecurity(w, v)
}
