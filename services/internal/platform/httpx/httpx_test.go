package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
)

func TestProblemMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{apperr.NotFound(), 404, "not_found"},
		{apperr.Validation(apperr.FieldError{Field: "name", Code: "too_long", Message: "x"}), 422, "validation_failed"},
		{apperr.Forbidden("edit_window_expired", "Too late."), 403, "edit_window_expired"},
		{apperr.Conflict("name_taken", "Taken."), 409, "name_taken"},
		{errors.New("database exploded at 10.0.0.5"), 500, "internal_error"},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/things", nil)
		w := httptest.NewRecorder()
		WriteProblem(w, r, c.err)
		if w.Code != c.status {
			t.Errorf("%v: status %d, want %d", c.err, w.Code, c.status)
		}
		if ct := w.Header().Get("Content-Type"); ct != ProblemContentType {
			t.Errorf("content type %q", ct)
		}
		var p Problem
		if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		if p.Code != c.code || p.Type != "urn:matecollab:problem:"+c.code || p.Status != c.status || p.Instance != "/api/v1/things" {
			t.Errorf("unexpected problem %+v", p)
		}
		if strings.Contains(w.Body.String(), "10.0.0.5") {
			t.Error("internal error details leaked to the client")
		}
	}
}

func TestRequestIDAcceptsValidAndReplacesInvalid(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = RequestIDFrom(r.Context()) }))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-ID", "0123456789abcdef0123456789abcdef")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if seen != "0123456789abcdef0123456789abcdef" || w.Header().Get("X-Request-ID") != seen {
		t.Fatalf("valid ID not kept: %q", seen)
	}

	r = httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-ID", "bad id\nwith newline")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if seen == "" || strings.ContainsAny(seen, " \n") {
		t.Fatalf("invalid ID should be replaced, got %q", seen)
	}
}

func TestRealIPOnlyTrustsConfiguredProxies(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	var got netip.Addr
	h := RealIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = ClientIPFrom(r.Context()) }))

	// Request via a trusted proxy: the right-most untrusted hop is the client.
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.1.2.3:5000"
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 198.51.100.7, 10.9.9.9")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if got.String() != "198.51.100.7" {
		t.Fatalf("got %s", got)
	}

	// Direct request from an untrusted peer: spoofed header is ignored.
	r = httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "198.51.100.50:5000"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if got.String() != "198.51.100.50" {
		t.Fatalf("spoofed header trusted: %s", got)
	}
}

func TestDecodeJSONIsStrict(t *testing.T) {
	type in struct {
		Name string `json:"name"`
	}
	decode := func(body string, limit int64) error {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		var v in
		return DecodeJSON(httptest.NewRecorder(), r, &v, limit)
	}
	if err := decode(`{"name":"ok"}`, 0); err != nil {
		t.Fatalf("valid body: %v", err)
	}
	for body, wantCode := range map[string]string{
		`{"name":"ok","extra":1}`: "malformed_json",
		`{"name":"ok"}{}`:         "malformed_json",
		`{"name":1}`:              "validation_failed",
		`not json`:                "malformed_json",
	} {
		e, ok := apperr.As(decode(body, 0))
		if !ok || e.Code != wantCode {
			t.Errorf("%s: got %v, want %s", body, e, wantCode)
		}
	}
	e, ok := apperr.As(decode(`{"name":"`+strings.Repeat("x", 100)+`"}`, 32))
	if !ok || e.Code != "payload_too_large" {
		t.Errorf("oversized body: got %v", e)
	}
}

func TestBodyLimitRejectsDeclaredOversize(t *testing.T) {
	h := BodyLimit(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			WriteProblem(w, r, err)
		}
	}))
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 50)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d", w.Code)
	}
}

func TestRecoverReturnsProblem(t *testing.T) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	h := Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Code != 500 || !strings.Contains(w.Body.String(), "internal_error") || strings.Contains(w.Body.String(), "boom") {
		t.Fatalf("unexpected response %d %s", w.Code, w.Body.String())
	}
}
