// Package httpx contains the HTTP transport foundation: JSON helpers, RFC 9457 Problem Details,
// middleware and server lifecycle. Handlers stay thin; business rules live in services.
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
)

// ProblemContentType is the RFC 9457 media type.
const ProblemContentType = "application/problem+json"

// Problem is an RFC 9457 Problem Details document with MateCollab extensions.
type Problem struct {
	Type      string              `json:"type"`
	Title     string              `json:"title"`
	Status    int                 `json:"status"`
	Code      string              `json:"code"`
	Detail    string              `json:"detail,omitempty"`
	Instance  string              `json:"instance,omitempty"`
	RequestID string              `json:"request_id,omitempty"`
	Errors    []apperr.FieldError `json:"errors,omitempty"`
}

// problemType returns a stable URN; documentation URLs can be added once the docs domain exists.
func problemType(code string) string { return "urn:matecollab:problem:" + code }

var statusByKind = map[apperr.Kind]int{
	apperr.KindValidation:           http.StatusUnprocessableEntity,
	apperr.KindMalformed:            http.StatusBadRequest,
	apperr.KindUnauthenticated:      http.StatusUnauthorized,
	apperr.KindForbidden:            http.StatusForbidden,
	apperr.KindNotFound:             http.StatusNotFound,
	apperr.KindConflict:             http.StatusConflict,
	apperr.KindPreconditionFailed:   http.StatusPreconditionFailed,
	apperr.KindPreconditionRequired: http.StatusPreconditionRequired,
	apperr.KindTooLarge:             http.StatusRequestEntityTooLarge,
	apperr.KindRateLimited:          http.StatusTooManyRequests,
	apperr.KindUnavailable:          http.StatusServiceUnavailable,
}

// ProblemFor converts any error into a Problem. Unclassified errors become a generic 500 so
// internal details never reach clients.
func ProblemFor(r *http.Request, err error) Problem {
	p := Problem{Instance: r.URL.Path, RequestID: RequestIDFrom(r.Context())}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		err = apperr.TooLarge("The request body is too large.")
	}
	if e, ok := apperr.As(err); ok {
		status, known := statusByKind[e.Kind]
		if !known {
			status = http.StatusInternalServerError
		}
		p.Status, p.Code, p.Detail, p.Errors = status, e.Code, e.Detail, e.Fields
	} else {
		p.Status, p.Code, p.Detail = http.StatusInternalServerError, "internal_error", "An unexpected error occurred."
	}
	p.Type = problemType(p.Code)
	p.Title = http.StatusText(p.Status)
	return p
}

// WriteProblem writes err as Problem Details and logs server-side failures with their cause.
func WriteProblem(w http.ResponseWriter, r *http.Request, err error) {
	p := ProblemFor(r, err)
	if p.Status >= 500 {
		LoggerFrom(r.Context()).ErrorContext(r.Context(), "request failed", slog.Int("status", p.Status), slog.String("code", p.Code), slog.Any("error", err))
	}
	if e, ok := apperr.As(err); ok && e.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(e.RetryAfter.Seconds()))))
	}
	w.Header().Set("Content-Type", ProblemContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

// NotFoundHandler answers unknown routes with Problem Details.
func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	WriteProblem(w, r, apperr.NotFound())
}

// MethodNotAllowedHandler answers known routes with the wrong method.
func MethodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	p := Problem{
		Type: problemType("method_not_allowed"), Title: http.StatusText(http.StatusMethodNotAllowed),
		Status: http.StatusMethodNotAllowed, Code: "method_not_allowed", Detail: "This method is not supported for the resource.",
		Instance: r.URL.Path, RequestID: RequestIDFrom(r.Context()),
	}
	w.Header().Set("Content-Type", ProblemContentType)
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
