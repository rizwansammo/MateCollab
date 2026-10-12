// Package cursor encodes opaque, signed pagination cursors bound to the query they came from
// (API-DESIGN.md §5): a cursor cannot be forged, edited or replayed with different filters.
package cursor

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
)

// ErrInvalid is the 400 returned for a bad cursor.
var ErrInvalid = &apperr.Error{Kind: apperr.KindMalformed, Code: "invalid_cursor", Detail: "The page cursor is invalid. Start from the first page."}

func sign(key []byte, scope string, payload []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(scope))
	mac.Write([]byte{0})
	mac.Write(payload)
	return mac.Sum(nil)[:16]
}

// Encode serialises v and signs it together with scope (e.g. the route and its filters).
func Encode(key []byte, scope string, v any) (string, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	b64 := base64.RawURLEncoding
	return b64.EncodeToString(payload) + "." + b64.EncodeToString(sign(key, scope, payload)), nil
}

// Decode verifies a cursor for scope and unmarshals it into v.
func Decode(key []byte, scope, s string, v any) error {
	if len(s) > 1024 {
		return ErrInvalid
	}
	p, sig, ok := strings.Cut(s, ".")
	if !ok {
		return ErrInvalid
	}
	b64 := base64.RawURLEncoding
	payload, err1 := b64.DecodeString(p)
	mac, err2 := b64.DecodeString(sig)
	if err1 != nil || err2 != nil || !hmac.Equal(mac, sign(key, scope, payload)) {
		return ErrInvalid
	}
	if json.Unmarshal(payload, v) != nil {
		return ErrInvalid
	}
	return nil
}
