package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
)

// DefaultBodyLimit is the default maximum request body (API-DESIGN.md §4).
const DefaultBodyLimit int64 = 64 << 10

// WriteJSON writes v with the given status. Authenticated and personal responses are never cached.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// DecodeJSON strictly decodes a single JSON object: unknown fields, trailing data and oversized
// bodies are rejected so client bugs surface early.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any, limit int64) error {
	if limit <= 0 {
		limit = DefaultBodyLimit
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return apperr.TooLarge("The request body is too large.")
		}
		var unknown *json.UnmarshalTypeError
		if errors.As(err, &unknown) {
			return apperr.Validation(apperr.FieldError{Field: unknown.Field, Code: "invalid_type", Message: "Has the wrong type."})
		}
		return apperr.Malformed("The request body is not valid JSON for this endpoint.", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperr.Malformed("The request body must contain a single JSON object.", err)
	}
	return nil
}
