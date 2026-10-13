package audit

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

// SlugFunc names the organisation in download filenames.
type SlugFunc func(r *http.Request) string

// Handler serves the organisation audit log (mounted under /api/v1/orgs/{orgId}).
type Handler struct {
	svc  *Service
	slug SlugFunc
}

// NewHandler returns the audit HTTP handler.
func NewHandler(svc *Service, slug SlugFunc) *Handler { return &Handler{svc: svc, slug: slug} }

// OrgRoutes registers the audit routes.
func (h *Handler) OrgRoutes(r chi.Router) {
	r.Get("/audit-logs", h.list)
	r.Get("/audit-logs/exports", h.listExports)
	r.Post("/audit-logs/exports", h.createExport)
	r.Get("/audit-logs/exports/{exportId}", h.getExport)
	r.Get("/audit-logs/exports/{exportId}/download", h.download)
}

type actorJSON struct {
	Type        string     `json:"type"`
	UserID      *uuid.UUID `json:"user_id"`
	DisplayName *string    `json:"display_name"`
}

type entryJSON struct {
	ID         uuid.UUID       `json:"id"`
	Actor      actorJSON       `json:"actor"`
	Action     string          `json:"action"`
	TargetType *string         `json:"target_type"`
	TargetID   *string         `json:"target_id"`
	Outcome    string          `json:"outcome"`
	IP         *string         `json:"ip"`
	UserAgent  *string         `json:"user_agent"`
	RequestID  *string         `json:"request_id"`
	Metadata   json.RawMessage `json:"metadata"`
	CreatedAt  time.Time       `json:"created_at"`
}

type filterJSON struct {
	ActorUserID *uuid.UUID `json:"actor_user_id,omitempty"`
	Action      string     `json:"action,omitempty"`
	TargetType  string     `json:"target_type,omitempty"`
	TargetID    string     `json:"target_id,omitempty"`
	From        *time.Time `json:"from,omitempty"`
	To          *time.Time `json:"to,omitempty"`
}

type exportJSON struct {
	ID          uuid.UUID  `json:"id"`
	Status      string     `json:"status"`
	Filter      filterJSON `json:"filter"`
	RowCount    *int32     `json:"row_count"`
	Error       *string    `json:"error"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
}

func exportToJSON(e Export) exportJSON {
	return exportJSON{ID: e.ID, Status: e.Status, Filter: filterJSON(e.Filter), RowCount: e.RowCount, Error: e.Error,
		CreatedAt: e.CreatedAt, CompletedAt: e.CompletedAt, ExpiresAt: e.ExpiresAt}
}

func actor(r *http.Request) tenant.Actor {
	a, _ := tenant.ActorFrom(r.Context())
	return a
}

// parseFilter reads the documented filters; invalid values are reported per field.
func parseFilter(get func(string) string) (Filter, error) {
	var f Filter
	var fields []apperr.FieldError
	if v := get("actor"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			fields = append(fields, apperr.Field("actor", "invalid", "Use a member ID."))
		}
		f.ActorUserID = &id
	}
	f.Action, f.TargetType, f.TargetID = get("action"), get("target_type"), get("target_id")
	if len(f.Action) > 64 || len(f.TargetType) > 40 || len(f.TargetID) > 64 {
		fields = append(fields, apperr.Field("action", "too_long", "A filter value is too long."))
	}
	for _, p := range []struct {
		name string
		dst  **time.Time
	}{{"from", &f.From}, {"to", &f.To}} {
		if v := get(p.name); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				fields = append(fields, apperr.Field(p.name, "invalid", "Use an RFC 3339 time, e.g. 2026-10-01T00:00:00Z."))
			}
			*p.dst = &t
		}
	}
	if len(fields) > 0 {
		return f, apperr.Validation(fields...)
	}
	return f, nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f, err := parseFilter(q.Get)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	limit := 0
	if v := q.Get("limit"); v != "" {
		if limit, err = strconv.Atoi(v); err != nil || limit < 1 || limit > 200 {
			httpx.WriteProblem(w, r, apperr.Validation(apperr.Field("limit", "invalid", "Use a number between 1 and 200.")))
			return
		}
	}
	rows, next, err := h.svc.List(r.Context(), actor(r), f, q.Get("cursor"), limit)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	out := make([]entryJSON, len(rows))
	for i, e := range rows {
		var ip *string
		if e.Ip != nil {
			s := e.Ip.String()
			ip = &s
		}
		out[i] = entryJSON{ID: e.ID, Actor: actorJSON{Type: e.ActorType, UserID: e.ActorUserID, DisplayName: e.ActorName},
			Action: e.Action, TargetType: e.TargetType, TargetID: e.TargetID, Outcome: e.Outcome, IP: ip, UserAgent: e.UserAgent,
			RequestID: e.RequestID, Metadata: e.Metadata, CreatedAt: e.CreatedAt}
	}
	page := map[string]any{"next_cursor": nil, "has_more": false}
	if next != "" {
		page = map[string]any{"next_cursor": next, "has_more": true}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": out, "page": page})
}

func (h *Handler) listExports(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListExports(r.Context(), actor(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	out := make([]exportJSON, len(list))
	for i, e := range list {
		out[i] = exportToJSON(e)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) createExport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ActorUserID string `json:"actor_user_id"`
		Action      string `json:"action"`
		TargetType  string `json:"target_type"`
		TargetID    string `json:"target_id"`
		From        string `json:"from"`
		To          string `json:"to"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	f, err := parseFilter(func(k string) string {
		return map[string]string{"actor": req.ActorUserID, "action": req.Action, "target_type": req.TargetType,
			"target_id": req.TargetID, "from": req.From, "to": req.To}[k]
	})
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	a := actor(r)
	e, err := h.svc.RequestExport(r.Context(), a, f, httpx.ClientMetaFrom(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/orgs/"+a.OrgID.String()+"/audit-logs/exports/"+e.ID.String())
	httpx.WriteJSON(w, http.StatusAccepted, exportToJSON(e))
}

func exportID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "exportId"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.NotFound())
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) getExport(w http.ResponseWriter, r *http.Request) {
	id, ok := exportID(w, r)
	if !ok {
		return
	}
	e, err := h.svc.GetExport(r.Context(), actor(r), id)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, exportToJSON(e))
}

func (h *Handler) download(w http.ResponseWriter, r *http.Request) {
	id, ok := exportID(w, r)
	if !ok {
		return
	}
	url, err := h.svc.DownloadURL(r.Context(), actor(r), id, h.slug(r), httpx.ClientMetaFrom(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Location", url)
	w.WriteHeader(http.StatusFound) // no body: the presigned URL is only in Location
}
