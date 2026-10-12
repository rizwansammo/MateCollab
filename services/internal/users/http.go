package users

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rizwansammo/MateCollab/services/internal/auth"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
)

// Handler serves /api/v1/me (profile, emails, preferences). Routes require a session.
type Handler struct{ svc *Service }

// NewHandler returns the users HTTP handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// UserJSON is the API representation of the caller's profile.
type UserJSON struct {
	ID           uuid.UUID `json:"id"`
	DisplayName  string    `json:"display_name"`
	Timezone     string    `json:"timezone"`
	Locale       string    `json:"locale"`
	PrimaryEmail string    `json:"primary_email"`
}

// ToJSON converts a User.
func ToJSON(u User) UserJSON {
	return UserJSON{ID: u.ID, DisplayName: u.DisplayName, Timezone: u.Timezone, Locale: u.Locale, PrimaryEmail: u.PrimaryEmail}
}

type emailJSON struct {
	ID         uuid.UUID  `json:"id"`
	Email      string     `json:"email"`
	IsPrimary  bool       `json:"is_primary"`
	VerifiedAt *time.Time `json:"verified_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

func emailToJSON(e Email) emailJSON {
	return emailJSON{ID: e.ID, Email: e.Address, IsPrimary: e.IsPrimary, VerifiedAt: e.VerifiedAt, CreatedAt: e.CreatedAt}
}

// Routes registers the /me routes (mounted at /api/v1 behind RequireSession).
func (h *Handler) Routes(r chi.Router) {
	r.Get("/me", h.me)
	r.Patch("/me", h.updateMe)
	r.Get("/me/emails", h.listEmails)
	r.Post("/me/emails", h.addEmail)
	r.Delete("/me/emails/{emailId}", h.removeEmail)
	r.Put("/me/emails/{emailId}/primary", h.makePrimary)
	r.Get("/me/preferences", h.preferences)
	r.Patch("/me/preferences", h.updatePreferences)
}

func principal(r *http.Request) auth.Principal {
	p, _ := auth.PrincipalFrom(r.Context())
	return p
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	u, err := h.svc.Me(r.Context(), principal(r).UserID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ToJSON(u))
}

func (h *Handler) updateMe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DisplayName *string `json:"display_name"`
		Timezone    *string `json:"timezone"`
		Locale      *string `json:"locale"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	u, err := h.svc.UpdateMe(r.Context(), principal(r).UserID, ProfileUpdate{DisplayName: req.DisplayName, Timezone: req.Timezone, Locale: req.Locale})
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ToJSON(u))
}

func (h *Handler) listEmails(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListEmails(r.Context(), principal(r).UserID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	out := make([]emailJSON, len(list))
	for i, e := range list {
		out[i] = emailToJSON(e)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) addEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	email := strings.TrimSpace(req.Email)
	at := strings.LastIndex(email, "@")
	if len(email) < 3 || len(email) > 254 || at <= 0 || at == len(email)-1 || strings.ContainsAny(email, " \r\n\t<>") {
		httpx.WriteProblem(w, r, apperr.Validation(apperr.Field("email", "invalid", "Enter a valid email address.")))
		return
	}
	e, err := h.svc.AddEmail(r.Context(), principal(r).UserID, email)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, emailToJSON(e))
}

func emailID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "emailId"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.NotFound())
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) removeEmail(w http.ResponseWriter, r *http.Request) {
	id, ok := emailID(w, r)
	if !ok {
		return
	}
	if err := h.svc.RemoveEmail(r.Context(), principal(r).UserID, id); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) makePrimary(w http.ResponseWriter, r *http.Request) {
	id, ok := emailID(w, r)
	if !ok {
		return
	}
	if err := h.svc.MakePrimary(r.Context(), principal(r), id); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) preferences(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetPreferences(r.Context(), principal(r).UserID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

func (h *Handler) updatePreferences(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Theme *string `json:"theme"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	p, err := h.svc.UpdatePreferences(r.Context(), principal(r).UserID, req.Theme)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}
