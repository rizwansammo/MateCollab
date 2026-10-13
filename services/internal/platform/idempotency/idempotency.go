// Package idempotency makes side-effecting POSTs safe to retry (API-DESIGN.md §7). A request
// carrying `Idempotency-Key` is recorded per user with a hash of its method, path and body;
// a retry with the same key and request replays the stored response, a different request with
// the same key answers 422 idempotency_key_reused, and a retry while the first attempt is still
// running answers 409 idempotency_conflict.
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

// TTL is how long a key and its response are kept.
const TTL = 24 * time.Hour

// Header is the request header carrying the key.
const Header = "Idempotency-Key"

var (
	errReused   = &apperr.Error{Kind: apperr.KindValidation, Code: "idempotency_key_reused", Detail: "This Idempotency-Key was already used for a different request."}
	errConflict = apperr.Conflict("idempotency_conflict", "A request with this Idempotency-Key is still being processed. Retry shortly.")
	errInvalid  = apperr.Validation(apperr.Field(Header, "invalid", "Use 1–128 printable characters, for example a UUID."))
)

// Store records keys in PostgreSQL, under the caller's user context (row-level security).
type Store struct {
	Pool *pgxpool.Pool
	// User identifies the caller (the session's user); requests without one are not deduplicated.
	User func(*http.Request) (uuid.UUID, bool)
	Now  func() time.Time
}

func validKey(k string) bool {
	if len(k) == 0 || len(k) > 128 {
		return false
	}
	for _, c := range k {
		if c < 0x21 || c > 0x7e {
			return false
		}
	}
	return true
}

// Middleware deduplicates the wrapped handler by Idempotency-Key.
func (s *Store) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get(Header)
		user, ok := s.User(r)
		if key == "" || !ok {
			next.ServeHTTP(w, r)
			return
		}
		if !validKey(key) {
			httpx.WriteProblem(w, r, errInvalid)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpx.DefaultBodyLimit))
		if err != nil {
			httpx.WriteProblem(w, r, apperr.TooLarge("The request body is too large."))
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		h := sha256.New()
		h.Write([]byte(r.Method + "\n" + r.URL.Path + "\n"))
		h.Write(body)
		sum := h.Sum(nil)

		claimed, err := s.claim(r.Context(), user, key, r, sum)
		if err != nil {
			httpx.WriteProblem(w, r, err)
			return
		}
		if !claimed {
			s.replay(w, r, user, key, sum)
			return
		}
		rec := &recorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.finish(context.WithoutCancel(r.Context()), user, key, rec)
	})
}

// claim inserts the key; false means it already exists (and has not expired).
func (s *Store) claim(ctx context.Context, user uuid.UUID, key string, r *http.Request, sum []byte) (bool, error) {
	var org *uuid.UUID
	if a, ok := tenant.ActorFrom(r.Context()); ok {
		id := uuid.UUID(a.OrgID)
		org = &id
	}
	now := s.Now()
	claimed := false
	err := db.InUserTx(ctx, s.Pool, tenant.UserID(user), func(tx *db.Tx) error {
		// An expired key is forgotten so the key can start a new request.
		if _, err := tx.Exec(ctx, `DELETE FROM idempotency_keys WHERE user_id = $1 AND key = $2 AND expires_at <= $3`, user, key, now); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO idempotency_keys (user_id, key, org_id, method, path, request_hash, state, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'in_progress', $7)
			ON CONFLICT (user_id, key) DO NOTHING`,
			user, key, org, r.Method, r.URL.Path, sum, now.Add(TTL))
		claimed = tag.RowsAffected() == 1
		return err
	})
	return claimed, err
}

func (s *Store) replay(w http.ResponseWriter, r *http.Request, user uuid.UUID, key string, sum []byte) {
	var (
		stored []byte
		state  string
		status *int32
		body   []byte
	)
	err := db.InUserTx(r.Context(), s.Pool, tenant.UserID(user), func(tx *db.Tx) error {
		return tx.QueryRow(r.Context(), `
			SELECT request_hash, state, response_status, response_body FROM idempotency_keys
			WHERE user_id = $1 AND key = $2`, user, key).Scan(&stored, &state, &status, &body)
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		httpx.WriteProblem(w, r, errConflict) // expired and removed concurrently: let the client retry
		return
	case err != nil:
		httpx.WriteProblem(w, r, err)
		return
	case !bytes.Equal(stored, sum):
		httpx.WriteProblem(w, r, errReused)
		return
	case state != "completed" || status == nil:
		httpx.WriteProblem(w, r, errConflict)
		return
	}
	contentType := "application/json; charset=utf-8"
	if *status >= 400 {
		contentType = httpx.ProblemContentType
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Idempotent-Replayed", "true")
	w.WriteHeader(int(*status))
	if len(body) > 0 && string(body) != "null" {
		_, _ = w.Write(body)
		_, _ = w.Write([]byte("\n"))
	}
}

// finish stores the response, or forgets the key after a server error so a retry can run again.
func (s *Store) finish(ctx context.Context, user uuid.UUID, key string, rec *recorder) {
	_ = db.InUserTx(ctx, s.Pool, tenant.UserID(user), func(tx *db.Tx) error {
		if rec.status >= 500 {
			_, err := tx.Exec(ctx, `DELETE FROM idempotency_keys WHERE user_id = $1 AND key = $2`, user, key)
			return err
		}
		var body any
		if b := bytes.TrimSpace(rec.body.Bytes()); len(b) > 0 && json.Valid(b) {
			body = json.RawMessage(b)
		}
		_, err := tx.Exec(ctx, `
			UPDATE idempotency_keys SET state = 'completed', response_status = $3, response_body = $4
			WHERE user_id = $1 AND key = $2`, user, key, rec.status, body)
		return err
	})
}

type recorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (r *recorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *recorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// ---- cleanup ------------------------------------------------------------------------------------

// CleanupArgs is the hourly job removing expired keys (BACKEND.md periodic jobs).
type CleanupArgs struct{}

// Kind is the River job kind.
func (CleanupArgs) Kind() string { return "idempotency.cleanup" }

// InsertOpts routes the cleanup to the maintenance queue.
func (CleanupArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: "maintenance", UniqueOpts: river.UniqueOpts{ByPeriod: 10 * time.Minute}}
}

// CleanupPeriodic schedules the cleanup hourly.
func CleanupPeriodic() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return CleanupArgs{}, nil }, nil)
}

// CleanupWorker deletes expired keys.
type CleanupWorker struct {
	river.WorkerDefaults[CleanupArgs]
	Pool *pgxpool.Pool
}

// Work deletes expired keys.
func (w *CleanupWorker) Work(ctx context.Context, _ *river.Job[CleanupArgs]) error {
	// Owner-run function: the job has no user context and the table is per-user under RLS.
	_, err := w.Pool.Exec(ctx, `SELECT mc_purge_idempotency_keys(now())`)
	return err
}
