package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"slices"

	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
)

// CSRFToken is HMAC(csrf subkey, session ID): stable for the session, useless without it.
func (s *Service) CSRFToken(p Principal) string {
	mac := hmac.New(sha256.New, s.CSRFKey)
	mac.Write(p.SessionID[:])
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

var errCSRF = apperr.Forbidden("csrf_failed", "The request could not be verified. Reload the page and try again.")

// CSRF protects unsafe methods (SECURITY-BASELINE.md §5): the request must come from an allowed
// origin and carry either the session's X-CSRF-Token (signed in) or the custom X-MC-Client
// header (pre-session endpoints such as login), which cross-site pages cannot send without a
// CORS preflight that the API never grants. It runs after the session middleware.
func (s *Service) CSRF(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" {
				if !slices.Contains(allowedOrigins, origin) {
					httpx.WriteProblem(w, r, errCSRF)
					return
				}
			} else if r.Header.Get("Sec-Fetch-Site") != "same-origin" {
				httpx.WriteProblem(w, r, errCSRF)
				return
			}
			if p, ok := PrincipalFrom(r.Context()); ok {
				got := []byte(r.Header.Get("X-CSRF-Token"))
				if !hmac.Equal(got, []byte(s.CSRFToken(p))) {
					httpx.WriteProblem(w, r, errCSRF)
					return
				}
			} else if r.Header.Get("X-MC-Client") != "web" {
				httpx.WriteProblem(w, r, errCSRF)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
