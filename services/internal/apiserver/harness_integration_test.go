//go:build integration

// HTTP-level integration tests: the real router, modules, PostgreSQL (with RLS and the
// production roles) and Valkey. Each "browser" has its own cookie jar and client address and
// sends the same headers the web client does, so CSRF, cookies and rate limits behave as in
// production.
package apiserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rizwansammo/MateCollab/services/internal/auth"
	"github.com/rizwansammo/MateCollab/services/internal/emails"
	"github.com/rizwansammo/MateCollab/services/internal/orgs"
	"github.com/rizwansammo/MateCollab/services/internal/platform/config"
	"github.com/rizwansammo/MateCollab/services/internal/platform/crypto"
	"github.com/rizwansammo/MateCollab/services/internal/platform/jobs"
	"github.com/rizwansammo/MateCollab/services/internal/platform/mfa"
	"github.com/rizwansammo/MateCollab/services/internal/platform/objectstore"
	"github.com/rizwansammo/MateCollab/services/internal/platform/redisx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/telemetry"
	"github.com/rizwansammo/MateCollab/services/internal/testkit"
)

const testOrigin = "http://localhost:8080"

// clock is the injectable time source; tests move it forward to expire sessions and codes.
type clock struct {
	mu     sync.Mutex
	offset time.Duration
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Now().Add(c.offset)
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	c.offset += d
	c.mu.Unlock()
}

// syncBuffer collects log output so tests can assert secrets never reach it.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

type harness struct {
	t       *testing.T
	db      *testkit.Database
	rdb     *redis.Client
	svc     *Services
	srv     *httptest.Server
	keyring *crypto.Keyring
	clock   *clock
	logs    *syncBuffer
	ipSeq   int
	secrets []string // values that must never appear in logs
	storage objectstore.Store
	queue   *jobs.Client
}

func newHarness(t *testing.T, env map[string]string) *harness {
	t.Helper()
	ctx := context.Background()
	database := testkit.NewDatabase(t)
	rdb, err := redisx.New(testkit.Require(t, testkit.EnvRedisURL))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.FlushDB(ctx).Err(); err != nil { // rate-limit and cache state from earlier runs
		t.Fatal(err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	vars := map[string]string{
		"MC_PUBLIC_URL":            testOrigin,
		"MC_TRUSTED_PROXIES":       "127.0.0.1/32,::1/128",
		"MC_ENCRYPTION_KEYRING":    "t1:" + key,
		"MC_ENCRYPTION_ACTIVE_KEY": "t1",
		"MC_HMAC_SECRET":           randomHex(t, 32),
	}
	for k, v := range env {
		vars[k] = v
	}
	cfg, err := config.Load(func(k string) string { return vars[k] })
	if err != nil {
		t.Fatal(err)
	}
	keyring, err := crypto.ParseKeyring(cfg.Crypto.Keyring, cfg.Crypto.ActiveKey)
	if err != nil {
		t.Fatal(err)
	}
	logs := &syncBuffer{}
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	queue, err := jobs.NewInsertOnlyClient(database.App, logger)
	if err != nil {
		t.Fatal(err)
	}
	clk := &clock{}
	// Object storage (audit exports) is optional: MinIO runs locally but has no CI image.
	var storage objectstore.Store
	if endpoint := os.Getenv(testkit.EnvS3Endpoint); endpoint != "" {
		s3, err := objectstore.NewS3(config.StorageConfig{
			Endpoint: endpoint, PublicEndpoint: endpoint, Region: "us-east-1", ForcePathStyle: true,
			Bucket: "mc-it-" + randomHex(t, 4), AccessKeyID: os.Getenv(testkit.EnvS3AccessKey), SecretAccessKey: os.Getenv(testkit.EnvS3SecretKey),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s3.Init(ctx, []string{testOrigin}); err != nil {
			t.Fatal(err)
		}
		storage = s3
	}
	svc, err := NewServices(Deps{Config: cfg, Pool: database.App, Redis: rdb, Jobs: queue, Storage: storage, Logger: logger, Now: clk.Now})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewTLSServer(Router(cfg, logger, telemetry.NewMetrics("test"), svc))
	t.Cleanup(srv.Close)
	h := &harness{t: t, db: database, rdb: rdb, svc: svc, srv: srv, keyring: keyring, clock: clk, logs: logs, storage: storage, queue: queue}
	t.Cleanup(h.assertNoSecretsLogged)
	return h
}

// run is t.Run for harness tests: assertions go to the subtest, and rate-limit state is reset so
// each subtest starts with fresh budgets (the rate-limit tests exhaust them deliberately).
func (h *harness) run(name string, fn func(t *testing.T)) {
	parent := h.t
	parent.Run(name, func(t *testing.T) {
		h.t = t
		defer func() { h.t = parent }()
		h.resetLimits()
		fn(t)
	})
}

func (h *harness) resetLimits() {
	ctx := context.Background()
	for _, pattern := range []string{"mc:rl:*", "mc:auth:fail:*", "mc:auth:wait:*"} {
		keys, err := h.rdb.Keys(ctx, pattern).Result()
		if err != nil {
			h.t.Fatal(err)
		}
		if len(keys) > 0 {
			if err := h.rdb.Del(ctx, keys...).Err(); err != nil {
				h.t.Fatal(err)
			}
		}
	}
}

func randomHex(t testing.TB, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// remember registers a secret (password, token, code) that must never be logged.
func (h *harness) remember(secrets ...string) {
	h.secrets = append(h.secrets, secrets...)
}

func (h *harness) assertNoSecretsLogged() {
	logs := h.logs.String()
	for _, s := range h.secrets {
		if s != "" && strings.Contains(logs, s) {
			h.t.Errorf("a secret value reached the logs: %.6s…", s)
		}
	}
}

// ---- browser -----------------------------------------------------------------------------------

type browser struct {
	h    *harness
	c    *http.Client
	ip   string
	csrf string
}

// browser returns a new client with its own cookies and address.
func (h *harness) browser() *browser {
	jar, err := cookiejar.New(nil)
	if err != nil {
		h.t.Fatal(err)
	}
	c := *h.srv.Client() // srv.Client returns a shared client; each browser needs its own jar
	c.Jar = jar
	h.ipSeq++
	return &browser{h: h, c: &c, ip: fmt.Sprintf("198.51.%d.%d", h.ipSeq/250, h.ipSeq%250+1)}
}

type response struct {
	t      *testing.T
	status int
	header http.Header
	body   []byte
}

type reqOpt func(*http.Request)

func header(k, v string) reqOpt { return func(r *http.Request) { r.Header.Set(k, v) } }
func withoutHeader(k string) reqOpt {
	return func(r *http.Request) { r.Header.Del(k) }
}

func (b *browser) do(method, path string, body any, opts ...reqOpt) *response {
	b.h.t.Helper()
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			b.h.t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, b.h.srv.URL+"/api/v1"+path, rd)
	if err != nil {
		b.h.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Origin", testOrigin)
	req.Header.Set("X-MC-Client", "web")
	req.Header.Set("X-Forwarded-For", b.ip)
	if b.csrf != "" {
		req.Header.Set("X-CSRF-Token", b.csrf)
	}
	for _, o := range opts {
		o(req)
	}
	res, err := b.c.Do(req)
	if err != nil {
		b.h.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		b.h.t.Fatal(err)
	}
	// Every response, success or problem, must match the OpenAPI document.
	loadContract(b.h.t).check(b.h.t, method, path, res.StatusCode, raw)
	return &response{t: b.h.t, status: res.StatusCode, header: res.Header, body: raw}
}

func (r *response) expect(status int, code ...string) *response {
	r.t.Helper()
	if r.status != status {
		r.t.Fatalf("status %d, want %d: %s", r.status, status, r.body)
	}
	if len(code) > 0 {
		var p struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(r.body, &p)
		if p.Code != code[0] {
			r.t.Fatalf("problem code %q, want %q: %s", p.Code, code[0], r.body)
		}
	}
	return r
}

func (r *response) json(v any) {
	r.t.Helper()
	if err := json.Unmarshal(r.body, v); err != nil {
		r.t.Fatalf("decode %s: %v", r.body, err)
	}
}

func (r *response) m() map[string]any {
	r.t.Helper()
	var v map[string]any
	r.json(&v)
	return v
}

// sessionCookie returns the browser's current session token (empty when signed out).
func (b *browser) sessionCookie() string {
	u, _ := url.Parse(b.h.srv.URL)
	for _, c := range b.c.Jar.Cookies(u) {
		if c.Name == auth.SessionCookie {
			return c.Value
		}
	}
	return ""
}

// setSessionCookie plants a cookie value (to replay old tokens).
func (b *browser) setSessionCookie(token string) {
	u, _ := url.Parse(b.h.srv.URL)
	b.c.Jar.SetCookies(u, []*http.Cookie{{Name: auth.SessionCookie, Value: token, Path: "/", Secure: true, HttpOnly: true}})
}

// refresh loads GET /auth/session and keeps the CSRF token.
func (b *browser) refresh() map[string]any {
	b.h.t.Helper()
	b.csrf = ""
	doc := b.do(http.MethodGet, "/auth/session", nil).expect(http.StatusOK).m()
	b.csrf = doc["csrf_token"].(string)
	return doc
}

func (b *browser) login(email, password string) map[string]any {
	b.h.t.Helper()
	res := b.do(http.MethodPost, "/auth/login", map[string]string{"email": email, "password": password}).expect(http.StatusOK).m()
	if res["mfa_required"] == true {
		b.h.t.Fatalf("unexpected MFA challenge for %s", email)
	}
	return b.refresh()
}

// ---- email capture -----------------------------------------------------------------------------

type sentEmail struct {
	Template emails.Template
	To       string
	Data     map[string]string
}

// emails returns the queued email jobs for to (all recipients when empty), oldest first.
func (h *harness) emails(to string) []sentEmail {
	h.t.Helper()
	rows, err := h.db.Owner.Query(context.Background(), `SELECT args FROM river_job WHERE kind = 'email.send' ORDER BY id`)
	if err != nil {
		h.t.Fatal(err)
	}
	defer rows.Close()
	var out []sentEmail
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			h.t.Fatal(err)
		}
		var a emails.Args
		if err := json.Unmarshal(raw, &a); err != nil {
			h.t.Fatal(err)
		}
		if to != "" && !strings.EqualFold(a.To, to) {
			continue
		}
		plain, _, err := h.keyring.Open(a.Sealed, []byte("email:"+string(a.Template)+":"+strings.ToLower(a.To)))
		if err != nil {
			h.t.Fatalf("open email payload: %v", err)
		}
		e := sentEmail{Template: a.Template, To: a.To}
		if err := json.Unmarshal(plain, &e.Data); err != nil {
			h.t.Fatal(err)
		}
		out = append(out, e)
	}
	return out
}

// lastToken returns the token from the newest email of template t sent to to.
func (h *harness) lastToken(to string, t emails.Template) string {
	h.t.Helper()
	list := h.emails(to)
	for i := len(list) - 1; i >= 0; i-- {
		if list[i].Template == t {
			link, err := url.Parse(list[i].Data["link"])
			if err != nil {
				h.t.Fatal(err)
			}
			if link.RawQuery != "" || !strings.HasPrefix(link.Fragment, "token=") {
				h.t.Fatalf("token must travel in the URL fragment only: %s", link.Redacted())
			}
			token := strings.TrimPrefix(link.Fragment, "token=")
			h.remember(token)
			return token
		}
	}
	h.t.Fatalf("no %s email for %s", t, to)
	return ""
}

// ---- fixtures ----------------------------------------------------------------------------------

// account is a signed-in user.
type account struct {
	email    string
	password string
	b        *browser
}

// seedOrg creates an organisation through the operator path and signs its new Owner in through
// the emailed setup link.
func (h *harness) seedOrg(name, slug, ownerEmail string) (string, *account) {
	h.t.Helper()
	res, err := h.svc.Orgs.CreateWithOwner(context.Background(), name, slug, orgs.OwnerSpec{Email: ownerEmail, DisplayName: "Owner of " + name})
	if err != nil {
		h.t.Fatalf("create org: %v", err)
	}
	if !res.NewAccount || res.SetupLink == "" {
		h.t.Fatalf("expected a new owner account with a setup link: %+v", res)
	}
	pw := "Owner-" + randomHex(h.t, 8)
	h.remember(pw)
	b := h.browser()
	b.do(http.MethodPost, "/auth/password/reset", map[string]string{"token": h.lastToken(ownerEmail, emails.AccountSetup), "new_password": pw}).
		expect(http.StatusNoContent)
	b.login(ownerEmail, pw)
	return res.Organisation.ID.String(), &account{email: ownerEmail, password: pw, b: b}
}

// invite sends an invitation from inviter and returns its token.
func (h *harness) invite(inviter *browser, org, email, role string) (string, string) {
	h.t.Helper()
	inv := inviter.do(http.MethodPost, "/orgs/"+org+"/invitations", map[string]string{"email": email, "role": role}).expect(http.StatusCreated).m()
	return inv["id"].(string), h.lastToken(email, emails.Invitation)
}

// join accepts an invitation as a new account and returns the signed-in account.
func (h *harness) join(token, email, name string) *account {
	h.t.Helper()
	pw := "Joiner-" + randomHex(h.t, 8)
	h.remember(pw)
	b := h.browser()
	res := b.do(http.MethodPost, "/invitations/accept", map[string]string{"token": token, "display_name": name, "password": pw}).expect(http.StatusOK).m()
	if res["signed_in"] != true {
		h.t.Fatalf("new account should be signed in: %v", res)
	}
	b.refresh()
	return &account{email: email, password: pw, b: b}
}

// totpCode computes the current code for a base32 secret at the harness clock.
func (h *harness) totpCode(secret string) string {
	h.t.Helper()
	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		h.t.Fatal(err)
	}
	return mfa.Code(raw, uint64(mfa.Step(h.clock.Now())))
}

// auditCount counts organisation audit entries with action.
func (h *harness) auditCount(org, action string) int {
	h.t.Helper()
	var n int
	if err := h.db.Owner.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE org_id = $1 AND action = $2`, org, action).Scan(&n); err != nil {
		h.t.Fatal(err)
	}
	return n
}
