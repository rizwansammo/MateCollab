package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rizwansammo/MateCollab/services/internal/auth/store"
	"github.com/rizwansammo/MateCollab/services/internal/emails"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/mfa"
	"github.com/rizwansammo/MateCollab/services/internal/platform/ratelimit"
	"github.com/rizwansammo/MateCollab/services/internal/platform/redisx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

// Rate-limit classes (API-DESIGN.md §8).
var (
	limitLoginIP      = ratelimit.PerMinute(10)
	limitLoginAccount = ratelimit.PerMinute(5)
	limitRecoveryIP   = ratelimit.PerHour(5)
	limitRecoveryMail = ratelimit.PerHour(5)
)

// errInvalidCredentials is identical for unknown email, wrong password and disabled accounts
// (status, body and — via the dummy hash — timing).
var errInvalidCredentials = apperr.Unauthenticated("invalid_credentials", "The email or password is incorrect.")

var errChallengeExpired = apperr.Unauthenticated("mfa_challenge_expired", "Your sign-in took too long. Start again.")

// LoginResult is either a session or a pending second-factor challenge.
type LoginResult struct {
	Session      *NewSession
	MFAChallenge string // token for the short-lived MFA cookie
}

// Login performs the password step.
func (s *Service) Login(ctx context.Context, email, pw string, meta httpx.ClientMeta) (LoginResult, error) {
	email = NormalizeEmail(email)
	acct := accountKey(email)
	if err := s.Limiter.Check(ctx, "auth.login.ip", meta.IP.String(), limitLoginIP); err != nil {
		return LoginResult{}, err
	}
	if err := s.Limiter.Check(ctx, "auth.login.account", acct, limitLoginAccount); err != nil {
		return LoginResult{}, err
	}
	if wait := s.loginDelay(ctx, acct); wait > 0 {
		return LoginResult{}, apperr.RateLimited(wait)
	}

	q := store.New(s.Pool)
	row, err := q.LoginLookup(ctx, email)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return LoginResult{}, err
	}
	if errors.Is(err, pgx.ErrNoRows) || row.PasswordHash == nil {
		s.Hasher.VerifyDummy(ctx, pw)
		s.loginFailed(ctx, q, acct, nil, meta)
		return LoginResult{}, errInvalidCredentials
	}
	user := tenant.UserID(row.UserID)
	ok, rehash, err := s.Hasher.Verify(ctx, pw, *row.PasswordHash)
	if err != nil {
		return LoginResult{}, err
	}
	if !ok || row.UserStatus != "active" {
		s.loginFailed(ctx, q, acct, &user, meta)
		return LoginResult{}, errInvalidCredentials
	}
	s.clearLoginFailures(ctx, acct)
	if rehash {
		if h, err := s.Hasher.Hash(ctx, pw); err == nil {
			_ = q.UpgradePasswordHash(ctx, store.UpgradePasswordHashParams{UserID: row.UserID, PasswordHash: h})
		}
	}

	if row.TOTPEnabled {
		token, err := NewToken()
		if err != nil {
			return LoginResult{}, err
		}
		if _, err := q.CreateAuthToken(ctx, store.CreateAuthTokenParams{
			UserID: &row.UserID, Purpose: "mfa_login", TokenHash: hashToken(token),
			Data: json.RawMessage(`{}`), ExpiresAt: s.Now().Add(MFAChallengeTTL),
		}); err != nil {
			return LoginResult{}, err
		}
		return LoginResult{MFAChallenge: token}, nil
	}
	ns, err := s.startSession(ctx, q, user, []string{"pwd"}, meta)
	if err != nil {
		return LoginResult{}, err
	}
	s.event(ctx, q, &user, "login.succeeded", "success", &ns.Principal.SessionID, meta, map[string]any{"amr": ns.Principal.AMR})
	return LoginResult{Session: &ns}, nil
}

// AccountExists reports whether email is a sign-in address of an existing account. Callers
// must only reveal the answer to someone who has proved access to that address (for example
// by holding an invitation link sent to it).
func (s *Service) AccountExists(ctx context.Context, email string) (bool, error) {
	_, ok, err := s.LookupAccount(ctx, email)
	return ok, err
}

// LookupAccount returns the account that signs in with email (operator tooling only).
func (s *Service) LookupAccount(ctx context.Context, email string) (tenant.UserID, bool, error) {
	row, err := store.New(s.Pool).LoginLookup(ctx, NormalizeEmail(email))
	if errors.Is(err, pgx.ErrNoRows) {
		return tenant.UserID{}, false, nil
	}
	if err != nil {
		return tenant.UserID{}, false, err
	}
	return tenant.UserID(row.UserID), true, nil
}

// LoginMFA completes sign-in with a TOTP code or a recovery code.
func (s *Service) LoginMFA(ctx context.Context, challenge, code, recoveryCode string, meta httpx.ClientMeta) (NewSession, error) {
	if challenge == "" {
		return NewSession{}, errChallengeExpired
	}
	var (
		ns      NewSession
		failed  bool
		burned  bool // the challenge used up its attempts: the user must start again
		user    tenant.UserID
		remains int64 = -1
	)
	err := db.InTx(ctx, s.Pool, func(tx *db.Tx) error {
		q := store.New(tx)
		tok, err := q.GetAuthTokenForUpdate(ctx, store.GetAuthTokenForUpdateParams{TokenHash: hashToken(challenge), Purpose: "mfa_login"})
		if errors.Is(err, pgx.ErrNoRows) {
			return errChallengeExpired
		}
		if err != nil {
			return err
		}
		if tok.UsedAt != nil || !s.Now().Before(tok.ExpiresAt) || tok.UserID == nil {
			return errChallengeExpired
		}
		user = tenant.UserID(*tok.UserID)
		attempts, err := q.IncrementAuthTokenAttempts(ctx, tok.ID)
		if err != nil {
			return err
		}
		if attempts > MFAChallengeAttempts {
			failed, burned = true, true // commit the burned challenge, then report it as expired
			return q.UseAuthToken(ctx, tok.ID)
		}
		amr, ok, err := s.checkSecondFactor(ctx, q, user, code, recoveryCode)
		if err != nil {
			return err
		}
		if !ok {
			failed = true // keep the attempt count
			if attempts == MFAChallengeAttempts {
				burned = true
				return q.UseAuthToken(ctx, tok.ID)
			}
			return nil
		}
		if err := q.UseAuthToken(ctx, tok.ID); err != nil {
			return err
		}
		if amr == "recovery" {
			n, err := q.CountUnusedRecoveryCodes(ctx, uuid.UUID(user))
			if err != nil {
				return err
			}
			remains = n
		}
		ns, err = s.startSession(ctx, q, user, []string{"pwd", amr}, meta)
		if err != nil {
			return err
		}
		if remains >= 0 {
			if err := s.notify(ctx, tx, user, emails.RecoveryCodeUsed, map[string]string{"remaining": strconv.FormatInt(remains, 10)}); err != nil {
				return err
			}
		}
		return nil
	})
	q := store.New(s.Pool)
	if err != nil {
		return NewSession{}, err
	}
	if failed {
		s.event(ctx, q, &user, "login.mfa_failed", "failure", nil, meta, nil)
		if burned {
			return NewSession{}, errChallengeExpired
		}
		return NewSession{}, apperr.Unauthenticated("invalid_code", "That code didn't work. Check it and try again.")
	}
	md := map[string]any{"amr": ns.Principal.AMR}
	if remains >= 0 {
		md["recovery_codes_remaining"] = remains
	}
	s.event(ctx, q, &user, "login.succeeded", "success", &ns.Principal.SessionID, meta, md)
	return ns, nil
}

// checkSecondFactor verifies a TOTP code (replay-protected) or consumes a recovery code.
func (s *Service) checkSecondFactor(ctx context.Context, q *store.Queries, user tenant.UserID, code, recoveryCode string) (string, bool, error) {
	uid := uuid.UUID(user)
	if recoveryCode != "" {
		normalized := mfa.NormalizeRecoveryCode(recoveryCode)
		if normalized == "" {
			return "", false, nil
		}
		codes, err := q.ListUnusedRecoveryCodesForUpdate(ctx, uid)
		if err != nil {
			return "", false, err
		}
		for _, c := range codes {
			if s.Keyring.VerifyMAC(c.KeyID, []byte(normalized), c.CodeHash) {
				return "recovery", true, q.UseRecoveryCode(ctx, c.ID)
			}
		}
		return "", false, nil
	}
	factor, err := q.GetTOTPFactorForUpdate(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if factor.ConfirmedAt == nil {
		return "", false, nil
	}
	secret, _, err := s.Keyring.Open(factor.SecretCiphertext, totpAAD(user))
	if err != nil {
		return "", false, err
	}
	step, ok := mfa.Verify(secret, code, s.Now(), factor.LastUsedStep)
	if !ok {
		return "", false, nil
	}
	n, err := q.AdvanceTOTPStep(ctx, store.AdvanceTOTPStepParams{Step: step, UserID: uid})
	if err != nil {
		return "", false, err
	}
	return "otp", n == 1, nil
}

func totpAAD(user tenant.UserID) []byte {
	id := uuid.UUID(user)
	return append(id[:], "totp"...)
}

// ---- progressive per-account delay (SECURITY-BASELINE.md §3) ------------------------------

const failureWindow = 15 * time.Minute

func (s *Service) loginDelay(ctx context.Context, acct string) time.Duration {
	ttl, err := s.Redis.PTTL(ctx, redisx.Key("auth", "wait", acct)).Result()
	if err != nil || ttl <= 0 {
		return 0
	}
	return ttl
}

// loginFailed counts failures per account key; from the fifth, each further failure imposes a
// doubling wait (1 s, 2 s, 4 s … capped at 30 s). There is no permanent lockout.
func (s *Service) loginFailed(ctx context.Context, q *store.Queries, acct string, user *tenant.UserID, meta httpx.ClientMeta) {
	failKey := redisx.Key("auth", "fail", acct)
	n, err := s.Redis.Incr(ctx, failKey).Result()
	if err == nil {
		_ = s.Redis.Expire(ctx, failKey, failureWindow).Err()
		if n >= 5 {
			delay := time.Second << min(n-5, 5)
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
			_ = s.Redis.Set(ctx, redisx.Key("auth", "wait", acct), "1", delay).Err()
		}
	}
	s.event(ctx, q, user, "login.failed", "failure", nil, meta, nil)
}

func (s *Service) clearLoginFailures(ctx context.Context, acct string) {
	_ = s.Redis.Del(ctx, redisx.Key("auth", "fail", acct), redisx.Key("auth", "wait", acct)).Err()
}

// ---- account security history ----------------------------------------------------------

func (s *Service) event(ctx context.Context, q *store.Queries, user *tenant.UserID, action, outcome string,
	session *uuid.UUID, meta httpx.ClientMeta, md map[string]any) {
	raw := json.RawMessage(`{}`)
	if md != nil {
		if b, err := json.Marshal(md); err == nil {
			raw = b
		}
	}
	var uid *uuid.UUID
	if user != nil {
		id := uuid.UUID(*user)
		uid = &id
	}
	if err := q.InsertAccountEvent(context.WithoutCancel(ctx), store.InsertAccountEventParams{
		UserID: uid, Action: action, Outcome: outcome, Ip: meta.IPPtr(), UserAgent: meta.UserAgentPtr(),
		SessionID: session, RequestID: meta.RequestIDPtr(), Metadata: raw,
	}); err != nil {
		s.Logger.WarnContext(ctx, "account audit event not recorded", "action", action, "error", err)
	}
}

// notify queues a security email to the user's primary address inside tx.
func (s *Service) notify(ctx context.Context, tx *db.Tx, user tenant.UserID, t emails.Template, data map[string]string) error {
	if err := db.SetScope(ctx, tx, tenant.Scope{UserID: user}); err != nil {
		return err
	}
	name, email, err := s.Accounts.Profile(ctx, tx, user)
	if err != nil {
		return err
	}
	if data == nil {
		data = map[string]string{}
	}
	data["name"] = name
	return s.Outbox.Enqueue(ctx, tx, emails.Message{Template: t, To: email, Data: data})
}
