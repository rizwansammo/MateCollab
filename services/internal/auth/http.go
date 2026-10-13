package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
)

// Cookie names. The __Host- prefix requires Secure, Path=/ and no Domain, so the cookie cannot
// be set or overwritten by another subdomain (SECURITY-BASELINE.md §4).
const (
	SessionCookie = "__Host-mc_session"
	MFACookie     = "__Host-mc_mfa"
)

func setCookie(w http.ResponseWriter, name, value string, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/", Secure: true, HttpOnly: true,
		SameSite: http.SameSiteLaxMode, MaxAge: int(maxAge.Seconds()),
	})
}

func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

func (s *Service) setSessionCookie(w http.ResponseWriter, ns NewSession) {
	setCookie(w, SessionCookie, ns.Token, ns.Principal.ExpiresAt.Sub(s.Now()))
}

// SetSessionCookie lets other modules (invitation acceptance) sign a new account in.
func (s *Service) SetSessionCookie(w http.ResponseWriter, ns NewSession) { s.setSessionCookie(w, ns) }

// Middleware resolves the session cookie into a Principal when present and valid. Requests
// without a valid session continue anonymously; RequireSession guards protected routes.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(SessionCookie)
		if err != nil || c.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		p, err := s.Authenticate(r.Context(), c.Value)
		switch {
		case err == nil:
			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
		case errors.Is(err, ErrNoSession):
			clearCookie(w, SessionCookie) // expired or revoked: drop it
			next.ServeHTTP(w, r)
		default:
			httpx.WriteProblem(w, r, apperr.Unavailable(err))
		}
	})
}

// RequireSession rejects anonymous requests with 401.
func RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := PrincipalFrom(r.Context()); !ok {
			httpx.WriteProblem(w, r, ErrNoSession)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SessionViewFunc builds the GET /auth/session document (user, organisations, CSRF token). It
// is supplied by the composition root because it spans modules.
type SessionViewFunc func(ctx context.Context, p Principal) (any, error)

// Handler serves the /api/v1/auth routes.
type Handler struct {
	svc  *Service
	view SessionViewFunc
}

// NewHandler returns the auth HTTP handler.
func NewHandler(svc *Service, view SessionViewFunc) *Handler { return &Handler{svc: svc, view: view} }

// Routes registers the auth routes on r (mounted at /api/v1).
func (h *Handler) Routes(r chi.Router) {
	r.Post("/auth/login", h.login)
	r.Post("/auth/login/mfa", h.loginMFA)
	r.Post("/auth/logout", h.logout)
	r.Post("/auth/password/forgot", h.forgotPassword)
	r.Post("/auth/password/reset", h.resetPassword)
	r.Post("/auth/email/verify", h.verifyEmail)
	r.Group(func(r chi.Router) {
		r.Use(RequireSession)
		r.Get("/auth/session", h.session)
		r.Post("/auth/reauthenticate", h.reauthenticate)
		r.Get("/auth/sessions", h.listSessions)
		r.Delete("/auth/sessions", h.revokeOtherSessions)
		r.Delete("/auth/sessions/{sessionId}", h.revokeSession)
		r.Get("/auth/mfa", h.mfaStatus)
		r.Post("/auth/mfa/totp/setup", h.setupTOTP)
		r.Post("/auth/mfa/totp/enable", h.enableTOTP)
		r.Post("/auth/mfa/totp/disable", h.disableTOTP)
		r.Post("/auth/mfa/recovery-codes", h.regenerateRecoveryCodes)
		r.Put("/auth/password", h.changePassword)
		r.Post("/auth/email/resend", h.resendVerification)
		r.Post("/me/deletion", h.requestDeletion)
		r.Delete("/me/deletion", h.cancelDeletion)
	})
}

const smallBody = 8 << 10

func validEmail(s string) bool {
	s = strings.TrimSpace(s)
	at := strings.LastIndex(s, "@")
	return len(s) >= 3 && len(s) <= 254 && at > 0 && at < len(s)-1 && !strings.ContainsAny(s, " \r\n\t<>")
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	MFARequired bool `json:"mfa_required"`
	Session     any  `json:"session,omitempty"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(w, r, &req, smallBody); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	var fields []apperr.FieldError
	if !validEmail(req.Email) {
		fields = append(fields, apperr.Field("email", "invalid", "Enter a valid email address."))
	}
	if req.Password == "" || len(req.Password) > 1024 {
		fields = append(fields, apperr.Field("password", "required", "Enter your password."))
	}
	if len(fields) > 0 {
		httpx.WriteProblem(w, r, apperr.Validation(fields...))
		return
	}
	res, err := h.svc.Login(r.Context(), req.Email, req.Password, httpx.ClientMetaFrom(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	if res.MFAChallenge != "" {
		setCookie(w, MFACookie, res.MFAChallenge, MFAChallengeTTL)
		httpx.WriteJSON(w, http.StatusOK, loginResponse{MFARequired: true})
		return
	}
	h.signedIn(w, r, *res.Session)
}

func (h *Handler) signedIn(w http.ResponseWriter, r *http.Request, ns NewSession) {
	// Signing in always issues a new token; a session the browser already held is revoked so it
	// cannot linger (or be swapped back in) once the cookie is replaced.
	if prev, ok := PrincipalFrom(r.Context()); ok && prev.SessionID != ns.Principal.SessionID {
		_ = h.svc.Logout(r.Context(), prev, httpx.ClientMetaFrom(r))
	}
	h.svc.setSessionCookie(w, ns)
	view, err := h.view(r.Context(), ns.Principal)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, loginResponse{Session: view})
}

type mfaRequest struct {
	Code         string `json:"code"`
	RecoveryCode string `json:"recovery_code"`
}

func (h *Handler) loginMFA(w http.ResponseWriter, r *http.Request) {
	var req mfaRequest
	if err := httpx.DecodeJSON(w, r, &req, smallBody); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	if (req.Code == "") == (req.RecoveryCode == "") {
		httpx.WriteProblem(w, r, apperr.Validation(apperr.Field("code", "required", "Enter a code from your authenticator app or a recovery code.")))
		return
	}
	challenge := ""
	if c, err := r.Cookie(MFACookie); err == nil {
		challenge = c.Value
	}
	ns, err := h.svc.LoginMFA(r.Context(), challenge, req.Code, req.RecoveryCode, httpx.ClientMetaFrom(r))
	if err != nil {
		if e, ok := apperr.As(err); ok && e.Code == "mfa_challenge_expired" {
			clearCookie(w, MFACookie)
		}
		httpx.WriteProblem(w, r, err)
		return
	}
	clearCookie(w, MFACookie)
	h.signedIn(w, r, ns)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if p, ok := PrincipalFrom(r.Context()); ok {
		if err := h.svc.Logout(r.Context(), p, httpx.ClientMetaFrom(r)); err != nil {
			httpx.WriteProblem(w, r, err)
			return
		}
	}
	clearCookie(w, SessionCookie)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) session(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	view, err := h.view(r.Context(), p)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (h *Handler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := httpx.DecodeJSON(w, r, &req, smallBody); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	if !validEmail(req.Email) {
		httpx.WriteProblem(w, r, apperr.Validation(apperr.Field("email", "invalid", "Enter a valid email address.")))
		return
	}
	if err := h.svc.ForgotPassword(r.Context(), req.Email, httpx.ClientMetaFrom(r)); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := httpx.DecodeJSON(w, r, &req, smallBody); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	if err := h.svc.ResetPassword(r.Context(), req.Token, req.NewPassword, httpx.ClientMetaFrom(r)); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	clearCookie(w, SessionCookie)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := httpx.DecodeJSON(w, r, &req, smallBody); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	if err := h.svc.VerifyEmail(r.Context(), req.Token, httpx.ClientMetaFrom(r)); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) resendVerification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EmailID uuid.UUID `json:"email_id"`
	}
	if err := httpx.DecodeJSON(w, r, &req, smallBody); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	p, _ := PrincipalFrom(r.Context())
	if err := h.svc.ResendVerification(r.Context(), p, req.EmailID, httpx.ClientMetaFrom(r)); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) reauthenticate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := httpx.DecodeJSON(w, r, &req, smallBody); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	if req.Password == "" {
		httpx.WriteProblem(w, r, apperr.Validation(apperr.Field("password", "required", "Enter your password.")))
		return
	}
	p, _ := PrincipalFrom(r.Context())
	if err := h.svc.Reauthenticate(r.Context(), p, req.Password, req.Code, httpx.ClientMetaFrom(r)); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type sessionItem struct {
	ID         uuid.UUID `json:"id"`
	Current    bool      `json:"current"`
	AMR        []string  `json:"amr"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	IP         *string   `json:"ip"`
	UserAgent  *string   `json:"user_agent"`
}

func (h *Handler) listSessions(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	list, err := h.svc.ListSessions(r.Context(), p)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	out := make([]sessionItem, len(list))
	for i, s := range list {
		out[i] = sessionItem{ID: s.ID, Current: s.Current, AMR: s.AMR, CreatedAt: s.CreatedAt, LastSeenAt: s.LastSeenAt, ExpiresAt: s.ExpiresAt}
		if s.IP != "" {
			out[i].IP = &list[i].IP
		}
		if s.UserAgent != "" {
			out[i].UserAgent = &list[i].UserAgent
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) revokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	n, err := h.svc.RevokeOtherSessions(r.Context(), p, httpx.ClientMetaFrom(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]int{"revoked": n})
}

func (h *Handler) revokeSession(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "sessionId"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.NotFound())
		return
	}
	p, _ := PrincipalFrom(r.Context())
	if err := h.svc.RevokeSession(r.Context(), p, id, httpx.ClientMetaFrom(r)); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	if id == p.SessionID {
		clearCookie(w, SessionCookie)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) mfaStatus(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	st, err := h.svc.MFAStatusFor(r.Context(), p)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"totp_enabled": st.TOTPEnabled, "recovery_codes_remaining": st.RecoveryCodesRemaining})
}

func (h *Handler) setupTOTP(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	setup, err := h.svc.SetupTOTP(r.Context(), p)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"secret": setup.Secret, "otpauth_uri": setup.OTPAuthURI, "qr_code": setup.QRCode})
}

func (h *Handler) enableTOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := httpx.DecodeJSON(w, r, &req, smallBody); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	p, _ := PrincipalFrom(r.Context())
	codes, err := h.svc.EnableTOTP(r.Context(), p, req.Code, httpx.ClientMetaFrom(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string][]string{"recovery_codes": codes})
}

func (h *Handler) disableTOTP(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	if err := h.svc.DisableTOTP(r.Context(), p, httpx.ClientMetaFrom(r)); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) regenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	codes, err := h.svc.RegenerateRecoveryCodes(r.Context(), p, httpx.ClientMetaFrom(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string][]string{"recovery_codes": codes})
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := httpx.DecodeJSON(w, r, &req, smallBody); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	p, _ := PrincipalFrom(r.Context())
	ns, err := h.svc.ChangePassword(r.Context(), p, req.NewPassword, httpx.ClientMetaFrom(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	h.signedIn(w, r, ns)
}

func (h *Handler) requestDeletion(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	at, err := h.svc.RequestDeletion(r.Context(), p, httpx.ClientMetaFrom(r))
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]time.Time{"scheduled_at": at})
}

func (h *Handler) cancelDeletion(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	if err := h.svc.CancelDeletion(r.Context(), p, httpx.ClientMetaFrom(r)); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
