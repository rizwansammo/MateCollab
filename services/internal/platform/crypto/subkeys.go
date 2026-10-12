package crypto

import (
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

// Subkeys are purpose-specific keys derived from MC_HMAC_SECRET. They protect ephemeral values
// only, so rotating the secret just forces clients to re-fetch those values.
type Subkeys struct {
	CSRF     []byte
	Cursor   []byte
	WSTicket []byte
}

// DeriveSubkeys expands the root secret with HKDF-SHA-256 and distinct info labels, so leaking
// one derived key does not allow forging values of another purpose.
func DeriveSubkeys(secret string) (Subkeys, error) {
	if len(secret) < 32 {
		return Subkeys{}, errors.New("crypto: MC_HMAC_SECRET must be at least 32 characters")
	}
	derive := func(label string) ([]byte, error) {
		return hkdf.Key(sha256.New, []byte(secret), nil, "matecollab/"+label+"/v1", 32)
	}
	var s Subkeys
	var err error
	if s.CSRF, err = derive("csrf"); err != nil {
		return s, err
	}
	if s.Cursor, err = derive("cursor"); err != nil {
		return s, err
	}
	if s.WSTicket, err = derive("ws-ticket"); err != nil {
		return s, err
	}
	return s, nil
}

// RandomToken returns n random bytes as unpadded base64url (sessions, one-time tokens).
func RandomToken(n int) (string, error) {
	if n < 16 {
		return "", errors.New("crypto: tokens need at least 128 bits")
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
