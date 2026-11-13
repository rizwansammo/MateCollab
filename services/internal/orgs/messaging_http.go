package orgs

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
)

type messagingJSON struct {
	Messaging
	Version   int32  `json:"version"`
	UpdatedAt string `json:"updated_at"`
}

func (h *Handler) getMessaging(w http.ResponseWriter, r *http.Request) {
	data, err := h.svc.GetMessaging(r.Context(), actor(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.Header().Set("ETag", etag(data.Version))
	httpx.WriteJSON(w, http.StatusOK, messagingJSON{Messaging: data.Value, Version: data.Version, UpdatedAt: data.UpdatedAt.UTC().Format("2006-01-02T15:04:05.999999Z07:00")})
}
func (h *Handler) putMessaging(w http.ResponseWriter, r *http.Request) {
	raw := strings.Trim(strings.TrimPrefix(r.Header.Get("If-Match"), "W/"), `"`)
	v, err := strconv.ParseInt(raw, 10, 32)
	if raw == "" || err != nil {
		httpx.WriteProblem(w, r, &apperr.Error{Kind: apperr.KindPreconditionRequired, Code: "precondition_required", Detail: "Send If-Match with the settings version."})
		return
	}
	var request Messaging
	if err := httpx.DecodeJSON(w, r, &request, 8<<10); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	data, err := h.svc.UpdateMessaging(r.Context(), actor(r), request, int32(v), httpx.ClientMetaFrom(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.Header().Set("ETag", etag(data.Version))
	httpx.WriteJSON(w, http.StatusOK, messagingJSON{Messaging: data.Value, Version: data.Version, UpdatedAt: data.UpdatedAt.UTC().Format("2006-01-02T15:04:05.999999Z07:00")})
}
