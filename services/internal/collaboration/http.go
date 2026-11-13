package collaboration

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes registers all Phase 3 operations below the resolved /orgs/{orgId} route.
func (h *Handler) Routes(r chi.Router) {
	r.Route("/spaces", func(r chi.Router) {
		r.Get("/", h.listSpaces)
		r.Post("/", h.createSpace)
		r.Route("/{spaceId}", func(r chi.Router) {
			r.Get("/", h.getSpace)
			r.Put("/", h.updateSpace)
			r.Post("/archive", h.archiveSpace)
			r.Delete("/", h.deleteSpace)
			r.Post("/join", h.joinSpace)
			r.Post("/leave", h.leaveSpace)
			r.Get("/members", h.listSpaceMembers)
			r.Put("/members/{userId}", h.setSpaceMember)
			r.Delete("/members/{userId}", h.removeSpaceMember)
			r.Route("/channels", func(r chi.Router) {
				r.Get("/", h.listChannels)
				r.Post("/", h.createChannel)
				r.Route("/{channelId}", func(r chi.Router) {
					r.Get("/", h.getChannel)
					r.Put("/", h.updateChannel)
					r.Post("/archive", h.archiveChannel)
					r.Get("/members", h.listChannelMembers)
					r.Post("/members", h.addChannelMember)
					r.Delete("/members/{userId}", h.removeChannelMember)
				})
			})
		})
	})
}

func actor(r *http.Request) tenant.Actor { a, _ := tenant.ActorFrom(r.Context()); return a }

func pathID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.NotFound())
		return uuid.Nil, false
	}
	return id, true
}
func ids(w http.ResponseWriter, r *http.Request, params ...string) ([]uuid.UUID, bool) {
	out := make([]uuid.UUID, 0, len(params))
	for _, p := range params {
		id, ok := pathID(w, r, p)
		if !ok {
			return nil, false
		}
		out = append(out, id)
	}
	return out, true
}
func reply(w http.ResponseWriter, r *http.Request, value any, err error, status int) {
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	httpx.WriteJSON(w, status, value)
}
func versionETag(t time.Time) string { return `"` + strconv.FormatInt(t.UnixMicro(), 10) + `"` }
func parseMatch(r *http.Request) (time.Time, error) {
	raw := strings.Trim(strings.TrimPrefix(r.Header.Get("If-Match"), "W/"), `"`)
	if raw == "" {
		return time.Time{}, &apperr.Error{Kind: apperr.KindPreconditionRequired, Code: "precondition_required", Detail: "Send If-Match with the resource version."}
	}
	micros, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, &apperr.Error{Kind: apperr.KindPreconditionFailed, Code: "version_mismatch", Detail: "Reload this resource and try again."}
	}
	return time.UnixMicro(micros), nil
}
func decode(w http.ResponseWriter, r *http.Request, request any) bool {
	if err := httpx.DecodeJSON(w, r, request, 12<<10); err != nil {
		httpx.WriteProblem(w, r, err)
		return false
	}
	return true
}

func (h *Handler) listSpaces(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListSpaces(r.Context(), actor(r))
	reply(w, r, map[string]any{"data": out}, err, http.StatusOK)
}
func (h *Handler) createSpace(w http.ResponseWriter, r *http.Request) {
	var in SpaceInput
	if !decode(w, r, &in) {
		return
	}
	out, err := h.svc.CreateSpace(r.Context(), actor(r), in, httpx.ClientMetaFrom(r))
	if err == nil {
		w.Header().Set("Location", "/api/v1/orgs/"+actor(r).OrgID.String()+"/spaces/"+out.ID.String())
		w.Header().Set("ETag", versionETag(out.UpdatedAt))
	}
	reply(w, r, out, err, http.StatusCreated)
}
func (h *Handler) getSpace(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "spaceId")
	if !ok {
		return
	}
	out, err := h.svc.Space(r.Context(), actor(r), id)
	if err == nil {
		w.Header().Set("ETag", versionETag(out.UpdatedAt))
	}
	reply(w, r, out, err, http.StatusOK)
}
func (h *Handler) updateSpace(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "spaceId")
	if !ok {
		return
	}
	expected, err := parseMatch(r)
	if err != nil {
		reply(w, r, nil, err, 0)
		return
	}
	var in SpaceInput
	if !decode(w, r, &in) {
		return
	}
	out, err := h.svc.UpdateSpace(r.Context(), actor(r), id, in, expected, httpx.ClientMetaFrom(r))
	if err == nil {
		w.Header().Set("ETag", versionETag(out.UpdatedAt))
	}
	reply(w, r, out, err, http.StatusOK)
}
func (h *Handler) archiveSpace(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "spaceId")
	if !ok {
		return
	}
	reply(w, r, nil, h.svc.ArchiveSpace(r.Context(), actor(r), id, httpx.ClientMetaFrom(r)), http.StatusNoContent)
}
func (h *Handler) deleteSpace(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "spaceId")
	if !ok {
		return
	}
	reply(w, r, nil, h.svc.DeleteSpace(r.Context(), actor(r), id, httpx.ClientMetaFrom(r)), http.StatusNoContent)
}
func (h *Handler) joinSpace(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "spaceId")
	if !ok {
		return
	}
	reply(w, r, nil, h.svc.JoinSpace(r.Context(), actor(r), id, httpx.ClientMetaFrom(r)), http.StatusNoContent)
}
func (h *Handler) leaveSpace(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "spaceId")
	if !ok {
		return
	}
	reply(w, r, nil, h.svc.LeaveSpace(r.Context(), actor(r), id, httpx.ClientMetaFrom(r)), http.StatusNoContent)
}
func (h *Handler) listSpaceMembers(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "spaceId")
	if !ok {
		return
	}
	out, err := h.svc.ListSpaceMembers(r.Context(), actor(r), id)
	reply(w, r, map[string]any{"data": out}, err, http.StatusOK)
}
func (h *Handler) setSpaceMember(w http.ResponseWriter, r *http.Request) {
	id, ok := ids(w, r, "spaceId", "userId")
	if !ok {
		return
	}
	var in struct {
		Role string `json:"role"`
	}
	if !decode(w, r, &in) {
		return
	}
	reply(w, r, nil, h.svc.SetSpaceMember(r.Context(), actor(r), id[0], id[1], in.Role, httpx.ClientMetaFrom(r)), http.StatusNoContent)
}
func (h *Handler) removeSpaceMember(w http.ResponseWriter, r *http.Request) {
	id, ok := ids(w, r, "spaceId", "userId")
	if !ok {
		return
	}
	reply(w, r, nil, h.svc.RemoveSpaceMember(r.Context(), actor(r), id[0], id[1], httpx.ClientMetaFrom(r)), http.StatusNoContent)
}
func (h *Handler) listChannels(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "spaceId")
	if !ok {
		return
	}
	out, err := h.svc.ListChannels(r.Context(), actor(r), id)
	reply(w, r, map[string]any{"data": out}, err, http.StatusOK)
}
func (h *Handler) createChannel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "spaceId")
	if !ok {
		return
	}
	var in ChannelInput
	if !decode(w, r, &in) {
		return
	}
	out, err := h.svc.CreateChannel(r.Context(), actor(r), id, in, httpx.ClientMetaFrom(r))
	if err == nil {
		w.Header().Set("Location", "/api/v1/orgs/"+actor(r).OrgID.String()+"/spaces/"+id.String()+"/channels/"+out.ID.String())
		w.Header().Set("ETag", versionETag(out.UpdatedAt))
	}
	reply(w, r, out, err, http.StatusCreated)
}
func (h *Handler) getChannel(w http.ResponseWriter, r *http.Request) {
	id, ok := ids(w, r, "spaceId", "channelId")
	if !ok {
		return
	}
	out, err := h.svc.Channel(r.Context(), actor(r), id[0], id[1])
	if err == nil {
		w.Header().Set("ETag", versionETag(out.UpdatedAt))
	}
	reply(w, r, out, err, http.StatusOK)
}
func (h *Handler) updateChannel(w http.ResponseWriter, r *http.Request) {
	id, ok := ids(w, r, "spaceId", "channelId")
	if !ok {
		return
	}
	expected, err := parseMatch(r)
	if err != nil {
		reply(w, r, nil, err, 0)
		return
	}
	var in ChannelInput
	if !decode(w, r, &in) {
		return
	}
	out, err := h.svc.UpdateChannel(r.Context(), actor(r), id[0], id[1], in, expected, httpx.ClientMetaFrom(r))
	if err == nil {
		w.Header().Set("ETag", versionETag(out.UpdatedAt))
	}
	reply(w, r, out, err, http.StatusOK)
}
func (h *Handler) archiveChannel(w http.ResponseWriter, r *http.Request) {
	id, ok := ids(w, r, "spaceId", "channelId")
	if !ok {
		return
	}
	reply(w, r, nil, h.svc.ArchiveChannel(r.Context(), actor(r), id[0], id[1], httpx.ClientMetaFrom(r)), http.StatusNoContent)
}
func (h *Handler) listChannelMembers(w http.ResponseWriter, r *http.Request) {
	id, ok := ids(w, r, "spaceId", "channelId")
	if !ok {
		return
	}
	out, err := h.svc.ListChannelMembers(r.Context(), actor(r), id[0], id[1])
	reply(w, r, map[string]any{"data": out}, err, http.StatusOK)
}
func (h *Handler) addChannelMember(w http.ResponseWriter, r *http.Request) {
	id, ok := ids(w, r, "spaceId", "channelId")
	if !ok {
		return
	}
	var in struct {
		UserID uuid.UUID `json:"user_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.Validation(apperr.Field("user_id", "invalid", "Choose a person.")))
		return
	}
	reply(w, r, nil, h.svc.AddChannelMember(r.Context(), actor(r), id[0], id[1], in.UserID, httpx.ClientMetaFrom(r)), http.StatusNoContent)
}
func (h *Handler) removeChannelMember(w http.ResponseWriter, r *http.Request) {
	id, ok := ids(w, r, "spaceId", "channelId", "userId")
	if !ok {
		return
	}
	reply(w, r, nil, h.svc.RemoveChannelMember(r.Context(), actor(r), id[0], id[1], id[2], httpx.ClientMetaFrom(r)), http.StatusNoContent)
}
