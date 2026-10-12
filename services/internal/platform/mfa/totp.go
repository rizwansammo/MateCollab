// Package mfa implements time-based one-time passwords (RFC 6238 / RFC 4226) and recovery
// codes (SECURITY-BASELINE.md §3). It is pure: storage and encryption live in the auth module.
package mfa

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" // RFC 6238 default; required for authenticator-app compatibility
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"

	"rsc.io/qr"
)

// TOTP parameters: SHA-1, 6 digits, 30-second steps (what every authenticator app supports).
const (
	SecretBytes = 20 // 160 bits
	Digits      = 6
	Period      = 30 * time.Second
	// Skew accepts codes from one step before or after the current one (clock drift).
	Skew = 1
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewSecret returns a random 160-bit TOTP secret.
func NewSecret() ([]byte, error) {
	s := make([]byte, SecretBytes)
	_, err := rand.Read(s)
	return s, err
}

// EncodeSecret returns the base32 form users type into an authenticator app.
func EncodeSecret(secret []byte) string { return b32.EncodeToString(secret) }

// Step returns the RFC 6238 time step for t.
func Step(t time.Time) int64 { return t.Unix() / int64(Period/time.Second) }

// Code computes the HOTP value for a counter (RFC 4226 dynamic truncation).
func Code(secret []byte, counter uint64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", Digits, value%1_000_000)
}

// Verify checks a code against the current step ± Skew. Steps at or before lastUsedStep are
// rejected so a code cannot be replayed. It returns the matched step to store as the new
// lastUsedStep.
func Verify(secret []byte, code string, now time.Time, lastUsedStep *int64) (int64, bool) {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != Digits {
		return 0, false
	}
	current := Step(now)
	matched, ok := int64(0), false
	for step := current - Skew; step <= current+Skew; step++ {
		if step < 0 || (lastUsedStep != nil && step <= *lastUsedStep) {
			continue
		}
		// Compare every candidate (no early exit) to keep timing independent of the match.
		if subtle.ConstantTimeCompare([]byte(Code(secret, uint64(step))), []byte(code)) == 1 && !ok {
			matched, ok = step, true
		}
	}
	return matched, ok
}

// URI returns the otpauth:// enrolment URI shown as a QR code.
func URI(issuer, account string, secret []byte) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	q := url.Values{}
	q.Set("secret", EncodeSecret(secret))
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(Digits))
	q.Set("period", fmt.Sprint(int(Period/time.Second)))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// QRDataURL renders the enrolment URI as a PNG data URL (rendered server-side so the browser
// needs no QR library; the CSP allows data: images).
func QRDataURL(uri string) (string, error) {
	code, err := qr.Encode(uri, qr.M)
	if err != nil {
		return "", err
	}
	code.Scale = 6
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(code.PNG()), nil
}
