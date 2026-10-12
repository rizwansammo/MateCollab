// Package password hashes and verifies account passwords with Argon2id and enforces the
// password policy (SECURITY-BASELINE.md §3).
package password

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"golang.org/x/text/unicode/norm"
)

// Params are the Argon2id cost parameters.
type Params struct {
	MemoryKiB uint32
	Time      uint32
	Threads   uint8
	SaltLen   int
	KeyLen    uint32
}

// DefaultParams: m = 64 MiB, t = 3, p = 1, 16-byte salt, 32-byte key (benchmarked to
// 150–300 ms on server hardware; well above OWASP's minimum).
var DefaultParams = Params{MemoryKiB: 64 * 1024, Time: 3, Threads: 1, SaltLen: 16, KeyLen: 32}

// Policy limits (characters, after NFKC normalisation).
const (
	MinLength = 12
	MaxLength = 128
)

// ErrMalformedHash is returned for a stored hash that is not a valid PHC Argon2id string.
var ErrMalformedHash = errors.New("password: malformed hash")

// Hasher hashes and verifies passwords. A semaphore bounds concurrent hashes so a burst of
// logins cannot exhaust memory.
type Hasher struct {
	params Params
	slots  chan struct{}

	dummyOnce sync.Once
	dummy     string
}

// NewHasher returns a hasher allowing at most concurrency simultaneous hash computations.
func NewHasher(p Params, concurrency int) *Hasher {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Hasher{params: p, slots: make(chan struct{}, concurrency)}
}

func (h *Hasher) acquire(ctx context.Context) error {
	select {
	case h.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Hasher) release() { <-h.slots }

// Normalize applies NFKC so visually identical input always hashes the same.
func Normalize(pw string) string { return norm.NFKC.String(pw) }

// Hash returns the PHC-encoded Argon2id hash of the normalised password.
func (h *Hasher) Hash(ctx context.Context, pw string) (string, error) {
	salt := make([]byte, h.params.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	defer h.release()
	key := argon2.IDKey([]byte(Normalize(pw)), salt, h.params.Time, h.params.MemoryKiB, h.params.Threads, h.params.KeyLen)
	return encode(h.params, salt, key), nil
}

// Verify reports whether pw matches the stored hash and whether the hash should be upgraded
// to the current parameters.
func (h *Hasher) Verify(ctx context.Context, pw, stored string) (ok, needsRehash bool, err error) {
	p, salt, key, err := decode(stored)
	if err != nil {
		return false, false, err
	}
	if err := h.acquire(ctx); err != nil {
		return false, false, err
	}
	defer h.release()
	got := argon2.IDKey([]byte(Normalize(pw)), salt, p.Time, p.MemoryKiB, p.Threads, uint32(len(key)))
	ok = subtle.ConstantTimeCompare(got, key) == 1
	needsRehash = p.MemoryKiB != h.params.MemoryKiB || p.Time != h.params.Time ||
		p.Threads != h.params.Threads || uint32(len(key)) != h.params.KeyLen
	return ok, needsRehash, nil
}

// VerifyDummy spends the same work as a real verification. Login calls it for unknown or
// password-less accounts so response timing does not reveal whether an account exists.
func (h *Hasher) VerifyDummy(ctx context.Context, pw string) {
	h.dummyOnce.Do(func() {
		salt := make([]byte, h.params.SaltLen)
		key := argon2.IDKey([]byte("matecollab-dummy"), salt, h.params.Time, h.params.MemoryKiB, h.params.Threads, h.params.KeyLen)
		h.dummy = encode(h.params, salt, key)
	})
	_, _, _ = h.Verify(ctx, pw, h.dummy)
}

func encode(p Params, salt, key []byte) string {
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.MemoryKiB, p.Time, p.Threads, b64.EncodeToString(salt), b64.EncodeToString(key))
}

func decode(s string) (Params, []byte, []byte, error) {
	parts := strings.Split(s, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return Params{}, nil, nil, ErrMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return Params{}, nil, nil, ErrMalformedHash
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Time, &p.Threads); err != nil {
		return Params{}, nil, nil, ErrMalformedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, ErrMalformedHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) < 16 {
		return Params{}, nil, nil, ErrMalformedHash
	}
	p.SaltLen = len(salt)
	p.KeyLen = uint32(len(key))
	return p, salt, key, nil
}

//go:embed common.txt
var commonList string

var common = func() map[string]struct{} {
	set := map[string]struct{}{}
	sc := bufio.NewScanner(strings.NewReader(commonList))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			set[strings.ToLower(line)] = struct{}{}
		}
	}
	return set
}()

// Problem codes returned by Check (part of the API contract as field error codes).
const (
	CodeTooShort = "too_short"
	CodeTooLong  = "too_long"
	CodeCommon   = "too_common"
	CodeSameAs   = "matches_email"
)

// Check applies the policy: 12–128 characters of any Unicode after NFKC normalisation, not on
// the offline list of common passwords and not the account's email address. There are no
// composition rules and no forced rotation (NIST SP 800-63B). It returns "" when acceptable.
func Check(pw, email string) string {
	n := Normalize(pw)
	length := utf8.RuneCountInString(n)
	switch {
	case length < MinLength:
		return CodeTooShort
	case length > MaxLength:
		return CodeTooLong
	}
	lower := strings.ToLower(n)
	if _, bad := common[lower]; bad {
		return CodeCommon
	}
	if email != "" {
		e := strings.ToLower(email)
		local, _, _ := strings.Cut(e, "@")
		if lower == e || (len(local) >= 4 && lower == local) {
			return CodeSameAs
		}
	}
	return ""
}

// Message returns a user-facing explanation for a Check code.
func Message(code string) string {
	switch code {
	case CodeTooShort:
		return fmt.Sprintf("Use at least %d characters.", MinLength)
	case CodeTooLong:
		return fmt.Sprintf("Use %d characters or fewer.", MaxLength)
	case CodeCommon:
		return "This password is too common. Choose something less predictable."
	case CodeSameAs:
		return "Your password cannot be your email address."
	}
	return "Choose a different password."
}
