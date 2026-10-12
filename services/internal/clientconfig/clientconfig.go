// Package clientconfig serves GET /api/v1/config: the runtime configuration the web app needs,
// so one web image serves every environment. Nothing secret is ever included.
package clientconfig

import (
	"net/http"

	"github.com/rizwansammo/MateCollab/services/internal/app"
	"github.com/rizwansammo/MateCollab/services/internal/platform/config"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
)

// RuntimeConfig matches the RuntimeConfig schema in packages/contracts.
type RuntimeConfig struct {
	Environment string   `json:"environment"`
	Version     string   `json:"version"`
	APIVersion  string   `json:"api_version"`
	PublicURL   string   `json:"public_url"`
	Realtime    Realtime `json:"realtime"`
	Media       Media    `json:"media"`
	Uploads     Uploads  `json:"uploads"`
	Features    Features `json:"features"`
}

// Realtime describes the WebSocket endpoint.
type Realtime struct {
	Path     string `json:"path"`
	Protocol string `json:"protocol"`
}

// Media describes the LiveKit signalling URL clients connect to; null when this deployment
// has no media server configured.
type Media struct {
	LiveKitURL *string `json:"livekit_url"`
}

// Uploads describes platform upload limits (organisations can only lower them).
type Uploads struct {
	MaxUploadBytes int64 `json:"max_upload_bytes"`
}

// Features lists deployment-level switches the UI needs to know about.
type Features struct {
	OrgSignup bool `json:"org_signup"`
}

// Build assembles the response from configuration.
func Build(cfg *config.Config) RuntimeConfig {
	public := ""
	if cfg.PublicURL != nil {
		public = cfg.PublicURL.String()
	}
	var livekit *string
	if cfg.LiveKit.PublicURL != "" {
		livekit = &cfg.LiveKit.PublicURL
	}
	return RuntimeConfig{
		Environment: string(cfg.Env),
		Version:     app.BuildVersion(),
		APIVersion:  "v1",
		PublicURL:   public,
		Realtime:    Realtime{Path: "/api/v1/realtime", Protocol: "mc.v1"},
		Media:       Media{LiveKitURL: livekit},
		Uploads:     Uploads{MaxUploadBytes: int64(cfg.Limits.MaxUploadMB) << 20},
		Features:    Features{OrgSignup: cfg.Features.AllowOrgSignup},
	}
}

// Handler serves the configuration. It is public and identical for every caller.
func Handler(cfg *config.Config) http.HandlerFunc {
	body := Build(cfg)
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		httpx.WriteJSON(w, http.StatusOK, body)
	}
}
