package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/telemetry"
)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	clientIPKey
	loggerKey
)

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,128}$`)

// RequestIDFrom returns the request ID stored by RequestID.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// ClientIPFrom returns the client address resolved by RealIP.
func ClientIPFrom(ctx context.Context) netip.Addr {
	ip, _ := ctx.Value(clientIPKey).(netip.Addr)
	return ip
}

// LoggerFrom returns the request-scoped logger (falls back to the default logger).
func LoggerFrom(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// RequestID accepts a well-formed X-Request-ID (Nginx sets one) or generates a new one, and echoes it.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !validRequestID.MatchString(id) {
			var b [16]byte
			_, _ = rand.Read(b[:])
			id = hex.EncodeToString(b[:])
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// RealIP resolves the client address. X-Forwarded-For is only trusted when the direct peer is a
// trusted proxy, and is walked right-to-left skipping further trusted hops.
func RealIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	isTrusted := func(a netip.Addr) bool {
		for _, p := range trusted {
			if p.Contains(a) {
				return true
			}
		}
		return false
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := peerAddr(r.RemoteAddr)
			if ip.IsValid() && isTrusted(ip) {
				hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
				for i := len(hops) - 1; i >= 0; i-- {
					hop, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
					if err != nil {
						break
					}
					ip = hop.Unmap()
					if !isTrusted(ip) {
						break
					}
				}
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientIPKey, ip)))
		})
	}
}

func peerAddr(remote string) netip.Addr {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap()
}

// Observe attaches a request logger and records access logs and metrics. Paths are logged without
// query strings; metrics use the route pattern to keep cardinality bounded.
func Observe(base *slog.Logger, m *telemetry.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			logger := base.With("request_id", RequestIDFrom(r.Context()))
			ctx := context.WithValue(r.Context(), loggerKey, logger)
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r.WithContext(ctx))

			route := "unmatched"
			if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
				route = rc.RoutePattern()
			}
			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			elapsed := time.Since(start)
			if m != nil {
				m.HTTPRequests.WithLabelValues(route, r.Method, strconv.Itoa(status/100)+"xx").Inc()
				m.HTTPDuration.WithLabelValues(route, r.Method).Observe(elapsed.Seconds())
			}
			logger.LogAttrs(r.Context(), slog.LevelInfo, "http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("route", route),
				slog.Int("status", status),
				slog.Int("bytes", ww.BytesWritten()),
				slog.Duration("duration", elapsed),
				slog.String("client_ip", ClientIPFrom(r.Context()).String()),
			)
		})
	}
}

// Recover turns panics into a logged 500 Problem Details response.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				LoggerFrom(r.Context()).ErrorContext(r.Context(), "panic recovered", slog.Any("panic", rec), slog.String("stack", string(debug.Stack())))
				WriteProblem(w, r, fmt.Errorf("panic: %v", rec))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// APISecurityHeaders applies the API response headers from SECURITY-BASELINE.md §11.
func APISecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// BodyLimit caps request bodies; routes that need more set their own limit when decoding.
func BodyLimit(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > n {
				WriteProblem(w, r, apperr.TooLarge("The request body is too large."))
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}

// ClientMeta describes who made a request, for audit records and session metadata. It never
// carries cookies, tokens or bodies.
type ClientMeta struct {
	IP        netip.Addr
	UserAgent string
	RequestID string
}

// ClientMetaFrom extracts ClientMeta from a request (after RequestID and RealIP ran).
func ClientMetaFrom(r *http.Request) ClientMeta {
	ua := r.UserAgent()
	if len(ua) > 512 {
		ua = ua[:512]
	}
	return ClientMeta{IP: ClientIPFrom(r.Context()), UserAgent: ua, RequestID: RequestIDFrom(r.Context())}
}

// IPPtr and UserAgentPtr adapt ClientMeta to nullable columns.
func (m ClientMeta) IPPtr() *netip.Addr {
	if !m.IP.IsValid() {
		return nil
	}
	ip := m.IP
	return &ip
}

func (m ClientMeta) UserAgentPtr() *string {
	if m.UserAgent == "" {
		return nil
	}
	ua := m.UserAgent
	return &ua
}

func (m ClientMeta) RequestIDPtr() *string {
	if m.RequestID == "" {
		return nil
	}
	id := m.RequestID
	return &id
}
