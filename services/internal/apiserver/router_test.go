package apiserver

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rizwansammo/MateCollab/services/internal/clientconfig"
	"github.com/rizwansammo/MateCollab/services/internal/platform/config"
	"github.com/rizwansammo/MateCollab/services/internal/platform/telemetry"
)

func testRouter(t *testing.T) http.Handler {
	t.Helper()
	cfg, err := config.Load(func(k string) string {
		return map[string]string{
			"MC_PUBLIC_URL":         "http://localhost:8080",
			"MC_LIVEKIT_PUBLIC_URL": "ws://localhost:7880",
			"MC_MAX_UPLOAD_MB":      "25",
		}[k]
	})
	if err != nil {
		t.Fatal(err)
	}
	return Router(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), telemetry.NewMetrics("test"))
}

func TestConfigEndpoint(t *testing.T) {
	w := httptest.NewRecorder()
	testRouter(t).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/config", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var rc clientconfig.RuntimeConfig
	if err := json.Unmarshal(w.Body.Bytes(), &rc); err != nil {
		t.Fatal(err)
	}
	if rc.APIVersion != "v1" || rc.Realtime.Protocol != "mc.v1" || rc.Media.LiveKitURL == nil || *rc.Media.LiveKitURL != "ws://localhost:7880" || rc.Uploads.MaxUploadBytes != 25<<20 {
		t.Fatalf("unexpected config %+v", rc)
	}
	for k, want := range map[string]string{
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
	} {
		if got := w.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("missing X-Request-ID")
	}
	if strings.Contains(strings.ToLower(w.Body.String()), "secret") || strings.Contains(w.Body.String(), "password") {
		t.Error("runtime config must not expose secrets")
	}
}

func TestUnknownRoutesAndMethodsReturnProblems(t *testing.T) {
	r := testRouter(t)
	for _, c := range []struct {
		method, path string
		status       int
		code         string
	}{
		{http.MethodGet, "/api/v1/nope", 404, "not_found"},
		{http.MethodPost, "/api/v1/config", 405, "method_not_allowed"},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(c.method, c.path, nil))
		if w.Code != c.status || !strings.Contains(w.Body.String(), `"code":"`+c.code+`"`) {
			t.Errorf("%s %s: %d %s", c.method, c.path, w.Code, w.Body.String())
		}
		if w.Header().Get("Content-Type") != "application/problem+json" {
			t.Errorf("%s %s: content type %q", c.method, c.path, w.Header().Get("Content-Type"))
		}
	}
}
