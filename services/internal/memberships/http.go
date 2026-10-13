package memberships

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rizwansammo/MateCollab/services/internal/auth"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

// Handler serves membership, directory and invitation routes.
type Handler struct{ svc *Service }

// NewHandler returns the memberships HTTP handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// PublicRoutes registers the invitee routes (mounted at /api/v1; a session is optional).
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Post("/invitations/preview", h.preview)
	r.Post("/invitations/accept", h.accept)
}

// SessionRoutes registers routes that need a session but no organisation.
func (h *Handler) SessionRoutes(r chi.Router) {
	r.Get("/me/organisations", h.myOrganisations)
}

// OrgRoutes registers tenant routes (mounted at /api/v1/orgs/{orgId} behind the organisation
// context middleware, which supplies the Actor).
func (h *Handler) OrgRoutes(r chi.Router) {
	r.Get("/members", h.listMembers)
	r.Get("/members/{userId}", h.getMember)
	r.Patch("/members/{userId}", h.updateMember)
	r.Get("/invitations", h.listInvitations)
	r.Post("/invitations", h.createInvitation)
	r.Delete("/invitations/{invitationId}", h.revokeInvitation)
	r.Post("/invitations/{invitationId}/resend", h.resendInvitation)
	r.Post("/ownership-transfer", h.transferOwnership)
}

// ---- JSON ---------------------------------------------------------------------------------------

// OrganisationJSON is an entry of GET /me/organisations (also embedded in the session document).
type OrganisationJSON struct {
	ID       uuid.UUID   `json:"id"`
	Slug     string      `json:"slug"`
	Name     string      `json:"name"`
	Status   string      `json:"status"`
	Role     tenant.Role `json:"role"`
	JoinedAt time.Time   `json:"joined_at"`
}

// OrganisationsToJSON converts the switcher list.
func OrganisationsToJSON(list []MyOrganisation) []OrganisationJSON {
	out := make([]OrganisationJSON, len(list))
	for i, o := range list {
		out[i] = OrganisationJSON{ID: o.ID, Slug: o.Slug, Name: o.Name, Status: o.Status, Role: o.Role, JoinedAt: o.JoinedAt}
	}
	return out
}

type memberJSON struct {
	UserID          uuid.UUID   `json:"user_id"`
	DisplayName     string      `json:"display_name"`
	Role            tenant.Role `json:"role"`
	Status          string      `json:"status"`
	Title           *string     `json:"title"`
	Email           *string     `json:"email"`
	JoinedAt        time.Time   `json:"joined_at"`
	StatusChangedAt *time.Time  `json:"status_changed_at"`
}

func memberToJSON(m Member) memberJSON {
	return memberJSON{UserID: m.UserID, DisplayName: m.DisplayName, Role: m.Role, Status: m.Status, Title: m.Title,
		Email: m.Email, JoinedAt: m.JoinedAt, StatusChangedAt: m.StatusChangedAt}
}

type inviterJSON struct {
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
}

type invitationJSON struct {
	ID         uuid.UUID   `json:"id"`
	Email      string      `json:"email"`
	Role       tenant.Role `json:"role"`
	Status     string      `json:"status"`
	InvitedBy  inviterJSON `json:"invited_by"`
	ExpiresAt  time.Time   `json:"expires_at"`
	LastSentAt time.Time   `json:"last_sent_at"`
	SendCount  int32       `json:"send_count"`
	CreatedAt  time.Time   `json:"created_at"`
	AcceptedAt *time.Time  `json:"accepted_at"`
}

func invitationToJSON(i Invitation) invitationJSON {
	return invitationJSON{ID: i.ID, Email: i.Email, Role: i.Role, Status: i.Status,
		InvitedBy: inviterJSON{UserID: i.InvitedBy, DisplayName: i.InviterName}, ExpiresAt: i.ExpiresAt,
		LastSentAt: i.LastSentAt, SendCount: i.SendCount, CreatedAt: i.CreatedAt, AcceptedAt: i.AcceptedAt}
}

type pageJSON struct {
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

func page(next string) pageJSON {
	if next == "" {
		return pageJSON{}
	}
	return pageJSON{NextCursor: &next, HasMore: true}
}

// ---- helpers ----------------------------------------------------------------------------------

func actor(r *http.Request) tenant.Actor {
	a, _ := tenant.ActorFrom(r.Context())
	return a
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.NotFound())
		return uuid.Nil, false
	}
	return id, true
}

func limitParam(q url.Values) (int, error) {
	raw := q.Get("limit")
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 200 {
		return 0, apperr.Validation(apperr.Field("limit", "invalid", "Use a number between 1 and 200."))
	}
	return n, nil
}

// ---- organisations ----------------------------------------------------------------------------

func (h *Handler) myOrganisations(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFrom(r.Context())
	list, err := h.svc.MyOrganisations(r.Context(), p.UserID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": OrganisationsToJSON(list)})
}

// ---- members ----------------------------------------------------------------------------------

func (h *Handler) listMembers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, err := limitParam(q)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	f := MemberFilter{Role: q.Get("role"), Status: q.Get("status"), Query: q.Get("q"), Cursor: q.Get("cursor"), Limit: limit}
	var fields []apperr.FieldError
	if f.Role != "" && !tenant.Role(f.Role).Valid() {
		fields = append(fields, apperr.Field("role", "invalid", "Choose owner, admin, member or guest."))
	}
	if f.Status != "" && f.Status != "active" && f.Status != "suspended" && f.Status != "deactivated" {
		fields = append(fields, apperr.Field("status", "invalid", "Choose active, suspended or deactivated."))
	}
	if len(f.Query) > 100 {
		fields = append(fields, apperr.Field("q", "too_long", "Use 100 characters or fewer."))
	}
	if len(fields) > 0 {
		httpx.WriteProblem(w, r, apperr.Validation(fields...))
		return
	}
	list, next, err := h.svc.ListMembers(r.Context(), actor(r), f)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	out := make([]memberJSON, len(list))
	for i, m := range list {
		out[i] = memberToJSON(m)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": out, "page": page(next)})
}

func (h *Handler) getMember(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "userId")
	if !ok {
		return
	}
	m, err := h.svc.GetMember(r.Context(), actor(r), id)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, memberToJSON(m))
}

func (h *Handler) updateMember(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "userId")
	if !ok {
		return
	}
	var req struct {
		Role   *string `json:"role"`
		Status *string `json:"status"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	m, err := h.svc.UpdateMember(r.Context(), actor(r), id, MemberUpdate{Role: req.Role, Status: req.Status}, httpx.ClientMetaFrom(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, memberToJSON(m))
}

// ---- invitations (admin side) -----------------------------------------------------------------

func (h *Handler) listInvitations(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, err := limitParam(q)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	status := q.Get("status")
	if status != "" && status != "pending" {
		httpx.WriteProblem(w, r, apperr.Validation(apperr.Field("status", "invalid", "Use status=pending or omit it.")))
		return
	}
	list, next, err := h.svc.ListInvitations(r.Context(), actor(r), status == "pending", q.Get("cursor"), limit)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	out := make([]invitationJSON, len(list))
	for i, inv := range list {
		out[i] = invitationToJSON(inv)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": out, "page": page(next)})
}

func (h *Handler) createInvitation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	a := actor(r)
	inv, err := h.svc.Invite(r.Context(), a, req.Email, req.Role, httpx.ClientMetaFrom(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/orgs/"+a.OrgID.String()+"/invitations/"+inv.ID.String())
	httpx.WriteJSON(w, http.StatusCreated, invitationToJSON(inv))
}

func (h *Handler) revokeInvitation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "invitationId")
	if !ok {
		return
	}
	if err := h.svc.Revoke(r.Context(), actor(r), id, httpx.ClientMetaFrom(r)); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) resendInvitation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "invitationId")
	if !ok {
		return
	}
	inv, err := h.svc.Resend(r.Context(), actor(r), id, httpx.ClientMetaFrom(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, invitationToJSON(inv))
}

// ---- invitations (invitee side) ---------------------------------------------------------------

type tokenRequest struct {
	Token string `json:"token"`
}

func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	var req tokenRequest
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	p, err := h.svc.PreviewInvitation(r.Context(), req.Token, httpx.ClientMetaFrom(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"organisation":   map[string]any{"id": p.OrgID, "slug": p.OrgSlug, "name": p.OrgName},
		"inviter_name":   p.InviterName,
		"email":          p.Email,
		"role":           p.Role,
		"status":         p.Status,
		"expires_at":     p.ExpiresAt,
		"account_exists": p.AccountExists,
	})
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	var principal *auth.Principal
	if p, ok := auth.PrincipalFrom(r.Context()); ok {
		principal = &p
	}
	res, err := h.svc.Accept(r.Context(), AcceptInput{Token: req.Token, DisplayName: req.DisplayName, Password: req.Password},
		principal, httpx.ClientMetaFrom(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	if res.Session != nil {
		h.svc.Auth.SetSessionCookie(w, *res.Session)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"organisation": map[string]any{"id": res.OrgID, "slug": res.OrgSlug, "name": res.OrgName},
		"signed_in":    res.Session != nil || principal != nil,
	})
}

// ---- ownership --------------------------------------------------------------------------------

func (h *Handler) transferOwnership(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID uuid.UUID `json:"user_id"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	if req.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.Validation(apperr.Field("user_id", "required", "Choose the Admin who will become the Owner.")))
		return
	}
	p, _ := auth.PrincipalFrom(r.Context())
	if err := h.svc.TransferOwnership(r.Context(), actor(r), p, req.UserID, httpx.ClientMetaFrom(r)); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
