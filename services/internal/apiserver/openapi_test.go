package apiserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/rizwansammo/MateCollab/services/internal/platform/config"
	"github.com/rizwansammo/MateCollab/services/internal/platform/telemetry"
)

// contract is the bundled OpenAPI document, used to check that the router serves exactly the
// documented operations and (in integration tests) that every response matches its schema.
type contract struct {
	paths     map[string]map[string]operation
	responses map[string]docResponse
	matchers  []pathMatcher
	compiler  *jsonschema.Compiler
	mu        sync.Mutex
	compiled  map[string]*jsonschema.Schema
}

type operation struct {
	Responses map[string]docResponse `json:"responses"`
}

type docResponse struct {
	Ref     string `json:"$ref"`
	Content map[string]struct {
		Schema struct {
			Ref string `json:"$ref"`
		} `json:"schema"`
	} `json:"content"`
}

type pathMatcher struct {
	template string
	re       *regexp.Regexp
}

var (
	contractOnce sync.Once
	contractDoc  *contract
	contractErr  error
)

func loadContract(t testing.TB) *contract {
	t.Helper()
	contractOnce.Do(func() { contractDoc, contractErr = readContract() })
	if contractErr != nil {
		t.Fatalf("OpenAPI bundle (run `pnpm gen`): %v", contractErr)
	}
	return contractDoc
}

var methods = map[string]bool{"get": true, "put": true, "post": true, "patch": true, "delete": true}

func readContract() (*contract, error) {
	raw, err := os.ReadFile(filepath.FromSlash(openAPIPath))
	if err != nil {
		return nil, err
	}
	var doc struct {
		Paths      map[string]map[string]json.RawMessage `json:"paths"`
		Components struct {
			Schemas   json.RawMessage        `json:"schemas"`
			Responses map[string]docResponse `json:"responses"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	c := &contract{paths: map[string]map[string]operation{}, responses: doc.Components.Responses, compiled: map[string]*jsonschema.Schema{}}
	for path, item := range doc.Paths {
		ops := map[string]operation{}
		for method, body := range item {
			if !methods[method] {
				continue
			}
			var op operation
			if err := json.Unmarshal(body, &op); err != nil {
				return nil, err
			}
			ops[strings.ToUpper(method)] = op
		}
		c.paths[path] = ops
		c.matchers = append(c.matchers, pathMatcher{template: path, re: regexp.MustCompile("^" + templateRegex(path) + "$")})
	}
	defs := strings.ReplaceAll(string(doc.Components.Schemas), "#/components/schemas/", "#/$defs/")
	root, err := jsonschema.UnmarshalJSON(strings.NewReader(`{"$schema":"https://json-schema.org/draft/2020-12/schema","$defs":` + defs + `}`))
	if err != nil {
		return nil, err
	}
	c.compiler = jsonschema.NewCompiler()
	c.compiler.AssertFormat()
	c.compiler.RegisterFormat(&jsonschema.Format{Name: "int64", Validate: func(any) error { return nil }})
	if err := c.compiler.AddResource("urn:matecollab:openapi:components", root); err != nil {
		return nil, err
	}
	return c, nil
}

// templateRegex turns "/orgs/{orgId}/members" into a regular expression for concrete paths.
func templateRegex(template string) string {
	var b strings.Builder
	for _, seg := range strings.Split(strings.TrimPrefix(template, "/"), "/") {
		b.WriteString("/")
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			b.WriteString(`[^/]+`)
		} else {
			b.WriteString(regexp.QuoteMeta(seg))
		}
	}
	return b.String()
}

func (c *contract) schema(component string) (*jsonschema.Schema, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.compiled[component]; ok {
		return s, nil
	}
	s, err := c.compiler.Compile("urn:matecollab:openapi:components#/$defs/" + component)
	if err != nil {
		return nil, err
	}
	c.compiled[component] = s
	return s, nil
}

// check validates one real response against the documented operation. path is relative to
// /api/v1 and may carry a query string.
func (c *contract) check(t testing.TB, method, path string, status int, body []byte) {
	t.Helper()
	path, _, _ = strings.Cut(path, "?")
	var op *operation
	for _, m := range c.matchers {
		if m.re.MatchString(path) {
			if o, ok := c.paths[m.template][method]; ok {
				op = &o
				break
			}
		}
	}
	if op == nil {
		t.Errorf("undocumented operation %s %s", method, path)
		return
	}
	res, ok := op.Responses[strconv.Itoa(status)]
	if !ok {
		res, ok = op.Responses[strconv.Itoa(status/100)+"XX"]
	}
	if !ok {
		t.Errorf("%s %s: status %d is not documented", method, path, status)
		return
	}
	if res.Ref != "" {
		res = c.responses[strings.TrimPrefix(res.Ref, "#/components/responses/")]
	}
	if len(res.Content) == 0 {
		if len(bytes.TrimSpace(body)) != 0 {
			t.Errorf("%s %s: status %d documents no body but returned %s", method, path, status, body)
		}
		return
	}
	for _, media := range res.Content {
		name := strings.TrimPrefix(media.Schema.Ref, "#/components/schemas/")
		s, err := c.schema(name)
		if err != nil {
			t.Fatalf("compile %s: %v", name, err)
		}
		v, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
		if err != nil {
			t.Errorf("%s %s: body is not JSON: %s", method, path, body)
			return
		}
		if err := s.Validate(v); err != nil {
			t.Errorf("%s %s %d does not match %s: %v\nbody: %s", method, path, status, name, err, body)
		}
		return
	}
}

// TestRouterServesExactlyTheDocumentedOperations walks the real router: every route must be
// in the OpenAPI document and every documented operation must be served.
func TestRouterServesExactlyTheDocumentedOperations(t *testing.T) {
	c := loadContract(t)
	cfg, err := config.Load(func(k string) string {
		return map[string]string{
			"MC_PUBLIC_URL":            "http://localhost:8080",
			"MC_ENCRYPTION_KEYRING":    "k1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
			"MC_ENCRYPTION_ACTIVE_KEY": "k1",
			"MC_HMAC_SECRET":           "0123456789abcdef0123456789abcdef",
		}[k]
	})
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := NewServices(Deps{Config: cfg, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	routes, ok := Router(cfg, logger, telemetry.NewMetrics("test"), svc).(chi.Routes)
	if !ok {
		t.Fatal("router does not expose its routes")
	}
	served := map[string]bool{}
	err = chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		route = strings.TrimPrefix(route, "/api/v1")
		if len(route) > 1 {
			route = strings.TrimSuffix(route, "/")
		}
		served[method+" "+route] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	documented := map[string]bool{}
	for path, ops := range c.paths {
		for method := range ops {
			documented[method+" "+path] = true
		}
	}
	var missing, extra []string
	for k := range served {
		if !documented[k] {
			extra = append(extra, k)
		}
	}
	for k := range documented {
		if !served[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(extra)
	sort.Strings(missing)
	if len(extra) > 0 {
		t.Errorf("routes missing from the OpenAPI document:\n  %s", strings.Join(extra, "\n  "))
	}
	if len(missing) > 0 {
		t.Errorf("documented operations the router does not serve:\n  %s", strings.Join(missing, "\n  "))
	}
	if len(served) < 30 {
		t.Fatalf("walked only %d routes", len(served))
	}
}

// recorder captures failures so the validator itself can be tested.
type recorder struct {
	testing.TB
	errors []string
}

func (r *recorder) Helper() {}
func (r *recorder) Errorf(format string, args ...any) {
	r.errors = append(r.errors, format)
}

func TestContractCheckRejectsDrift(t *testing.T) {
	c := loadContract(t)
	ok := `{"mfa_required":false}`
	for _, tc := range []struct {
		name, method, path string
		status             int
		body               string
		wantErr            bool
	}{
		{"documented", "POST", "/auth/login", 200, ok, false},
		{"undocumented field", "POST", "/auth/login", 200, `{"mfa_required":false,"token":"x"}`, true},
		{"wrong type", "POST", "/auth/login", 200, `{"mfa_required":"no"}`, true},
		{"problem", "GET", "/orgs/0199a6f0-0000-7000-8000-000000000000/members?limit=2", 404,
			`{"type":"urn:matecollab:problem:not_found","title":"Not Found","status":404,"code":"not_found"}`, false},
		{"bad problem", "GET", "/orgs/x/members", 404, `{"code":"not_found"}`, true},
		{"undocumented route", "GET", "/admin/secrets", 200, `{}`, true},
		{"body on 204", "POST", "/auth/logout", 204, `{"ok":true}`, true},
		{"undocumented method", "DELETE", "/me/preferences", 204, ``, true},
	} {
		r := &recorder{TB: t}
		c.check(r, tc.method, tc.path, tc.status, []byte(tc.body))
		if got := len(r.errors) > 0; got != tc.wantErr {
			t.Errorf("%s: errors=%v, want error=%t", tc.name, r.errors, tc.wantErr)
		}
	}
}
