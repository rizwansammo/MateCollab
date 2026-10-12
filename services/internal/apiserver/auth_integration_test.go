//go:build integration

package apiserver

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rizwansammo/MateCollab/services/internal/auth"
	"github.com/rizwansammo/MateCollab/services/internal/emails"
)

func TestAuthentication(t *testing.T) {
	h := newHarness(t, nil)
	org, owner := h.seedOrg("Acme", "acme", "owner@acme.test")

	h.run("session document", func(t *testing.T) {
		doc := owner.b.refresh()
		user := doc["user"].(map[string]any)
		if user["primary_email"] != "owner@acme.test" || doc["csrf_token"] == "" {
			t.Fatalf("session: %v", doc)
		}
		orgList := doc["organisations"].([]any)
		if len(orgList) != 1 || orgList[0].(map[string]any)["id"] != org || orgList[0].(map[string]any)["role"] != "owner" {
			t.Fatalf("organisations: %v", orgList)
		}
	})

	h.run("wrong password and unknown account look the same", func(t *testing.T) {
		b := h.browser()
		wrong := b.do(http.MethodPost, "/auth/login", map[string]string{"email": owner.email, "password": "not-the-password"}).expect(http.StatusUnauthorized, "invalid_credentials")
		unknown := b.do(http.MethodPost, "/auth/login", map[string]string{"email": "nobody@acme.test", "password": "not-the-password"}).expect(http.StatusUnauthorized, "invalid_credentials")
		strip := func(r *response) string {
			m := r.m()
			delete(m, "request_id")
			return strings.Join([]string{m["code"].(string), m["detail"].(string), m["title"].(string)}, "|")
		}
		if strip(wrong) != strip(unknown) {
			t.Fatalf("responses differ:\n%s\n%s", wrong.body, unknown.body)
		}
		if b.sessionCookie() != "" {
			t.Fatal("failed login set a session cookie")
		}
	})

	h.run("cookie attributes", func(t *testing.T) {
		b := h.browser()
		res := b.do(http.MethodPost, "/auth/login", map[string]string{"email": owner.email, "password": owner.password}).expect(http.StatusOK)
		c := res.header.Get("Set-Cookie")
		for _, want := range []string{auth.SessionCookie + "=", "Path=/", "HttpOnly", "Secure", "SameSite=Lax"} {
			if !strings.Contains(c, want) {
				t.Fatalf("Set-Cookie %q lacks %q", c, want)
			}
		}
		if strings.Contains(strings.ToLower(c), "domain=") {
			t.Fatal("__Host- cookie must not set Domain")
		}
		if strings.Contains(string(res.body), b.sessionCookie()) {
			t.Fatal("session token leaked into the response body")
		}
		h.remember(b.sessionCookie())
	})

	h.run("CSRF", func(t *testing.T) {
		b := h.browser()
		b.do(http.MethodPost, "/auth/login", map[string]string{"email": owner.email, "password": owner.password}, withoutHeader("X-MC-Client")).
			expect(http.StatusForbidden, "csrf_failed")
		b.do(http.MethodPost, "/auth/login", map[string]string{"email": owner.email, "password": owner.password}, header("Origin", "https://evil.example")).
			expect(http.StatusForbidden, "csrf_failed")
		b.do(http.MethodPost, "/auth/login", map[string]string{"email": owner.email, "password": owner.password}, withoutHeader("Origin")).
			expect(http.StatusForbidden, "csrf_failed")
		b.login(owner.email, owner.password)
		// Signed in: the custom header alone is no longer enough; the session's token is required.
		b.do(http.MethodPost, "/auth/logout", nil, withoutHeader("X-CSRF-Token")).expect(http.StatusForbidden, "csrf_failed")
		b.do(http.MethodPost, "/auth/logout", nil, header("X-CSRF-Token", "forged")).expect(http.StatusForbidden, "csrf_failed")
		b.do(http.MethodPatch, "/me", map[string]string{"display_name": "Hijacked"}, header("X-CSRF-Token", owner.b.csrf)).
			expect(http.StatusForbidden, "csrf_failed") // another session's token
		b.do(http.MethodGet, "/auth/session", nil).expect(http.StatusOK) // safe methods need no token
		b.do(http.MethodPost, "/auth/logout", nil).expect(http.StatusNoContent)
	})

	h.run("session fixation: signing in replaces and revokes the previous session", func(t *testing.T) {
		b := h.browser()
		b.login(owner.email, owner.password)
		before := b.sessionCookie()
		b.login(owner.email, owner.password)
		after := b.sessionCookie()
		if before == after || after == "" {
			t.Fatal("sign-in did not rotate the session token")
		}
		replay := h.browser()
		replay.setSessionCookie(before)
		replay.do(http.MethodGet, "/auth/session", nil).expect(http.StatusUnauthorized, "unauthenticated")
		// A planted, never-issued token is not adopted either.
		planted := h.browser()
		planted.setSessionCookie("attacker-chosen-token-value-0000000000")
		planted.do(http.MethodGet, "/auth/session", nil).expect(http.StatusUnauthorized)
	})

	h.run("logout and revoked sessions", func(t *testing.T) {
		b := h.browser()
		b.login(owner.email, owner.password)
		token := b.sessionCookie()
		b.do(http.MethodPost, "/auth/logout", nil).expect(http.StatusNoContent)
		b.do(http.MethodGet, "/auth/session", nil).expect(http.StatusUnauthorized)
		replay := h.browser()
		replay.setSessionCookie(token)
		replay.do(http.MethodGet, "/auth/session", nil).expect(http.StatusUnauthorized)
	})

	h.run("device list, revoke one, sign out everywhere else", func(t *testing.T) {
		laptop, phone, tablet := h.browser(), h.browser(), h.browser()
		laptop.login(owner.email, owner.password)
		phone.login(owner.email, owner.password)
		tablet.login(owner.email, owner.password)
		var list struct {
			Data []struct {
				ID      string `json:"id"`
				Current bool   `json:"current"`
			} `json:"data"`
		}
		laptop.do(http.MethodGet, "/auth/sessions", nil).expect(http.StatusOK).json(&list)
		if len(list.Data) < 3 || !list.Data[0].Current {
			t.Fatalf("sessions: %+v", list.Data)
		}
		phoneSession := phone.refresh()["session"].(map[string]any)["id"].(string)
		laptop.do(http.MethodDelete, "/auth/sessions/"+phoneSession, nil).expect(http.StatusNoContent)
		phone.do(http.MethodGet, "/auth/session", nil).expect(http.StatusUnauthorized) // takes effect immediately (cache evicted)

		// Another user cannot revoke this user's sessions.
		_, other := h.seedOrg("Other", "other-co", "owner@other.test")
		tabletSession := tablet.refresh()["session"].(map[string]any)["id"].(string)
		other.b.do(http.MethodDelete, "/auth/sessions/"+tabletSession, nil).expect(http.StatusNotFound)
		tablet.do(http.MethodGet, "/auth/session", nil).expect(http.StatusOK)

		laptop.do(http.MethodDelete, "/auth/sessions", nil).expect(http.StatusOK)
		tablet.do(http.MethodGet, "/auth/session", nil).expect(http.StatusUnauthorized)
		laptop.do(http.MethodGet, "/auth/session", nil).expect(http.StatusOK)
		owner.b.login(owner.email, owner.password) // signed out above with the others
	})

	h.run("expired sessions", func(t *testing.T) {
		b := h.browser()
		b.login(owner.email, owner.password)
		h.clock.Advance(auth.SessionIdle + time.Minute)
		defer h.clock.Advance(-(auth.SessionIdle + time.Minute))
		b.do(http.MethodGet, "/auth/session", nil).expect(http.StatusUnauthorized)
	})

	h.run("forgot and reset password", func(t *testing.T) {
		anon := h.browser()
		anon.do(http.MethodPost, "/auth/password/forgot", map[string]string{"email": owner.email}).expect(http.StatusAccepted)
		anon.do(http.MethodPost, "/auth/password/forgot", map[string]string{"email": "nobody@acme.test"}).expect(http.StatusAccepted)
		if len(h.emails("nobody@acme.test")) != 0 {
			t.Fatal("reset email sent to an unknown address")
		}
		token := h.lastToken(owner.email, emails.PasswordReset)
		other := h.browser()
		other.login(owner.email, owner.password)

		anon = h.browser() // recovery requests share a per-address budget (5/hour)
		anon.do(http.MethodPost, "/auth/password/reset", map[string]string{"token": token, "new_password": "short"}).expect(http.StatusUnprocessableEntity, "validation_failed")
		newPW := "Reset-" + randomHex(t, 8)
		h.remember(newPW)
		anon.do(http.MethodPost, "/auth/password/reset", map[string]string{"token": token, "new_password": newPW}).expect(http.StatusNoContent)
		anon.do(http.MethodPost, "/auth/password/reset", map[string]string{"token": token, "new_password": "Another-" + randomHex(t, 8)}).
			expect(http.StatusUnprocessableEntity, "validation_failed") // single use
		anon.do(http.MethodPost, "/auth/password/reset", map[string]string{"token": token + "x", "new_password": newPW}).expect(http.StatusUnprocessableEntity)
		other.do(http.MethodGet, "/auth/session", nil).expect(http.StatusUnauthorized) // every session revoked
		h.browser().do(http.MethodPost, "/auth/login", map[string]string{"email": owner.email, "password": owner.password}).expect(http.StatusUnauthorized)
		owner.password = newPW
		owner.b.login(owner.email, owner.password)
	})

	h.run("expired reset token", func(t *testing.T) {
		anon := h.browser()
		anon.do(http.MethodPost, "/auth/password/forgot", map[string]string{"email": owner.email}).expect(http.StatusAccepted)
		token := h.lastToken(owner.email, emails.PasswordReset)
		h.clock.Advance(auth.ResetTTL + time.Minute)
		defer h.clock.Advance(-(auth.ResetTTL + time.Minute))
		anon.do(http.MethodPost, "/auth/password/reset", map[string]string{"token": token, "new_password": "Expired-" + randomHex(t, 8)}).expect(http.StatusUnprocessableEntity)
	})

	h.run("email verification", func(t *testing.T) {
		added := owner.b.do(http.MethodPost, "/me/emails", map[string]string{"email": "second@acme.test"}).expect(http.StatusCreated).m()
		token := h.lastToken("second@acme.test", emails.EmailVerification)
		anon := h.browser()
		anon.do(http.MethodPost, "/auth/email/verify", map[string]string{"token": token}).expect(http.StatusNoContent)
		anon.do(http.MethodPost, "/auth/email/verify", map[string]string{"token": token}).expect(http.StatusUnprocessableEntity) // single use
		var list struct {
			Data []struct {
				ID         string  `json:"id"`
				VerifiedAt *string `json:"verified_at"`
			} `json:"data"`
		}
		owner.b.do(http.MethodGet, "/me/emails", nil).expect(http.StatusOK).json(&list)
		for _, e := range list.Data {
			if e.ID == added["id"] && e.VerifiedAt == nil {
				t.Fatal("address not verified")
			}
		}
		// An address another account has proved cannot be claimed by verifying it again.
		_, rival := h.seedOrg("Rival", "rival", "owner@rival.test")
		rival.b.do(http.MethodPost, "/me/emails", map[string]string{"email": "second@acme.test"}).expect(http.StatusCreated)
		rivalToken := h.lastToken("second@acme.test", emails.EmailVerification)
		anon.do(http.MethodPost, "/auth/email/verify", map[string]string{"token": rivalToken}).expect(http.StatusConflict, "email_taken")
	})

	h.run("two-step verification and recovery codes", func(t *testing.T) {
		setup := owner.b.do(http.MethodPost, "/auth/mfa/totp/setup", nil).expect(http.StatusOK).m()
		secret := setup["secret"].(string)
		h.remember(secret)
		owner.b.do(http.MethodPost, "/auth/mfa/totp/enable", map[string]string{"code": "000000"}).expect(http.StatusUnprocessableEntity)
		var enabled struct {
			RecoveryCodes []string `json:"recovery_codes"`
		}
		owner.b.do(http.MethodPost, "/auth/mfa/totp/enable", map[string]string{"code": h.totpCode(secret)}).expect(http.StatusOK).json(&enabled)
		if len(enabled.RecoveryCodes) != 10 {
			t.Fatalf("recovery codes: %v", enabled.RecoveryCodes)
		}
		h.remember(enabled.RecoveryCodes...)

		// Password step now yields a challenge, not a session.
		b := h.browser()
		res := b.do(http.MethodPost, "/auth/login", map[string]string{"email": owner.email, "password": owner.password}).expect(http.StatusOK).m()
		if res["mfa_required"] != true || b.sessionCookie() != "" {
			t.Fatalf("expected an MFA challenge: %v", res)
		}
		b.do(http.MethodGet, "/auth/session", nil).expect(http.StatusUnauthorized)
		b.do(http.MethodPost, "/auth/login/mfa", map[string]string{"code": "123456"}).expect(http.StatusUnauthorized, "invalid_code")
		h.clock.Advance(31 * time.Second) // the enrolment code's time step cannot be replayed
		b.do(http.MethodPost, "/auth/login/mfa", map[string]string{"code": h.totpCode(secret)}).expect(http.StatusOK)
		if doc := b.refresh(); doc["mfa"].(map[string]any)["totp_enabled"] != true {
			t.Fatalf("session should report TOTP: %v", doc)
		}
		// The same code cannot be used twice.
		replay := h.browser()
		replay.do(http.MethodPost, "/auth/login", map[string]string{"email": owner.email, "password": owner.password}).expect(http.StatusOK)
		replay.do(http.MethodPost, "/auth/login/mfa", map[string]string{"code": h.totpCode(secret)}).expect(http.StatusUnauthorized, "invalid_code")

		// The challenge dies after five failures.
		locked := h.browser()
		locked.do(http.MethodPost, "/auth/login", map[string]string{"email": owner.email, "password": owner.password}).expect(http.StatusOK)
		for range 4 {
			locked.do(http.MethodPost, "/auth/login/mfa", map[string]string{"code": "999999"}).expect(http.StatusUnauthorized, "invalid_code")
		}
		locked.do(http.MethodPost, "/auth/login/mfa", map[string]string{"code": "999999"}).expect(http.StatusUnauthorized, "mfa_challenge_expired")
		h.clock.Advance(31 * time.Second)
		locked.do(http.MethodPost, "/auth/login/mfa", map[string]string{"code": h.totpCode(secret)}).expect(http.StatusUnauthorized, "mfa_challenge_expired")

		// Recovery codes work once.
		rc := h.browser()
		rc.do(http.MethodPost, "/auth/login", map[string]string{"email": owner.email, "password": owner.password}).expect(http.StatusOK)
		rc.do(http.MethodPost, "/auth/login/mfa", map[string]string{"recovery_code": enabled.RecoveryCodes[0]}).expect(http.StatusOK)
		again := h.browser()
		again.do(http.MethodPost, "/auth/login", map[string]string{"email": owner.email, "password": owner.password}).expect(http.StatusOK)
		again.do(http.MethodPost, "/auth/login/mfa", map[string]string{"recovery_code": enabled.RecoveryCodes[0]}).expect(http.StatusUnauthorized, "invalid_code")
		if st := rc.refresh(); st == nil {
			t.Fatal("no session")
		}
		status := rc.do(http.MethodGet, "/auth/mfa", nil).expect(http.StatusOK).m()
		if status["recovery_codes_remaining"].(float64) != 9 {
			t.Fatalf("remaining codes: %v", status)
		}
		h.remember(rc.sessionCookie(), b.sessionCookie())

		// Turning it off needs a recent password check.
		h.clock.Advance(auth.StepUpWindow + time.Minute)
		rc.do(http.MethodPost, "/auth/mfa/totp/disable", nil).expect(http.StatusForbidden, "reauthentication_required")
		rc.do(http.MethodPost, "/auth/reauthenticate", map[string]string{"password": "wrong-password"}).expect(http.StatusUnprocessableEntity)
		rc.do(http.MethodPost, "/auth/reauthenticate", map[string]string{"password": owner.password, "code": h.totpCode(secret)}).expect(http.StatusNoContent)
		rc.do(http.MethodPost, "/auth/mfa/totp/disable", nil).expect(http.StatusNoContent)
		h.resetLimits()                                // this subtest used up the account's sign-in budget
		h.browser().login(owner.email, owner.password) // single step again
	})

	h.run("login rate limiting", func(t *testing.T) {
		victim := "owner@other.test"
		b := h.browser()
		var limited *response
		for i := range 12 {
			res := b.do(http.MethodPost, "/auth/login", map[string]string{"email": victim, "password": "guess-" + randomHex(t, 4)})
			if res.status == http.StatusTooManyRequests {
				limited = res
				break
			}
			if res.status != http.StatusUnauthorized || i > 6 {
				t.Fatalf("attempt %d: %d %s", i, res.status, res.body)
			}
		}
		if limited == nil {
			t.Fatal("account was never rate limited")
		}
		if limited.header.Get("Retry-After") == "" {
			t.Fatal("429 without Retry-After")
		}
		// The limit follows the account, not the client address.
		h.browser().do(http.MethodPost, "/auth/login", map[string]string{"email": victim, "password": "another-guess"}).expect(http.StatusTooManyRequests, "rate_limited")
		// Other accounts are unaffected.
		h.browser().login(owner.email, owner.password)
	})

	h.run("recovery rate limiting", func(t *testing.T) {
		b := h.browser()
		var hit bool
		for range 8 {
			if b.do(http.MethodPost, "/auth/password/forgot", map[string]string{"email": "flood@acme.test"}).status == http.StatusTooManyRequests {
				hit = true
				break
			}
		}
		if !hit {
			t.Fatal("password reset requests were never rate limited")
		}
	})
}
