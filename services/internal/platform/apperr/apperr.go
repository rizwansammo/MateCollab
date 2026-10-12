// Package apperr defines the typed errors services return. The HTTP layer maps them to
// RFC 9457 Problem Details; nothing else needs to know about status codes.
package apperr

import (
	"errors"
	"fmt"
)

// Kind classifies an error for transport mapping.
type Kind int

// Error kinds.
const (
	KindInternal Kind = iota
	KindValidation
	KindMalformed
	KindUnauthenticated
	KindForbidden
	KindNotFound
	KindConflict
	KindPreconditionFailed
	KindTooLarge
	KindRateLimited
	KindUnavailable
)

// FieldError describes one invalid input field.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error is a classified application error with a stable machine-readable code.
type Error struct {
	Kind   Kind
	Code   string // stable, snake_case, part of the API contract
	Detail string // safe to show to API clients
	Fields []FieldError
	Err    error // wrapped cause; never shown to clients
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Detail, e.Err)
	}
	return e.Code + ": " + e.Detail
}

func (e *Error) Unwrap() error { return e.Err }

// As extracts an *Error from err.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// Validation reports invalid input fields.
func Validation(fields ...FieldError) *Error {
	return &Error{Kind: KindValidation, Code: "validation_failed", Detail: "One or more fields are invalid.", Fields: fields}
}

// Malformed reports a request body that could not be decoded.
func Malformed(detail string, cause error) *Error {
	return &Error{Kind: KindMalformed, Code: "malformed_json", Detail: detail, Err: cause}
}

// NotFound reports a missing or invisible resource (never distinguishes the two).
func NotFound() *Error {
	return &Error{Kind: KindNotFound, Code: "not_found", Detail: "The requested resource was not found."}
}

// Forbidden reports a visible resource the caller may not act on.
func Forbidden(code, detail string) *Error {
	if code == "" {
		code = "forbidden"
	}
	return &Error{Kind: KindForbidden, Code: code, Detail: detail}
}

// Conflict reports a state conflict such as a duplicate name.
func Conflict(code, detail string) *Error {
	return &Error{Kind: KindConflict, Code: code, Detail: detail}
}

// TooLarge reports an oversized request body.
func TooLarge(detail string) *Error {
	return &Error{Kind: KindTooLarge, Code: "payload_too_large", Detail: detail}
}

// Unavailable reports a dependency outage.
func Unavailable(cause error) *Error {
	return &Error{Kind: KindUnavailable, Code: "service_unavailable", Detail: "The service is temporarily unavailable.", Err: cause}
}
