// Package crypto holds application-level key management (SECURITY-BASELINE.md §12):
//   - a versioned keyring for data persisted with a key ID (AES-256-GCM sealing and keyed MACs),
//   - HKDF subkeys of MC_HMAC_SECRET for ephemeral values only (CSRF, cursors, WS tickets),
//   - random token generation.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	sealVersion = 0x01
	keySize     = 32
)

var validKID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// Keyring holds named 256-bit keys and the active key used for new data.
type Keyring struct {
	keys   map[string][]byte
	active string
}

// ParseKeyring parses "kid:base64key,kid2:base64key" and selects the active key.
func ParseKeyring(spec, active string) (*Keyring, error) {
	k := &Keyring{keys: map[string][]byte{}, active: active}
	for _, entry := range strings.Split(spec, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		kid, b64, ok := strings.Cut(entry, ":")
		if !ok || !validKID.MatchString(kid) {
			return nil, fmt.Errorf("keyring: invalid key id in entry %q", kid)
		}
		key, err := base64.StdEncoding.DecodeString(b64)
		if err != nil || len(key) != keySize {
			return nil, fmt.Errorf("keyring: key %q must be %d bytes of base64", kid, keySize)
		}
		if _, dup := k.keys[kid]; dup {
			return nil, fmt.Errorf("keyring: duplicate key id %q", kid)
		}
		k.keys[kid] = key
	}
	if len(k.keys) == 0 {
		return nil, errors.New("keyring: no keys configured")
	}
	if _, ok := k.keys[active]; !ok {
		return nil, fmt.Errorf("keyring: active key %q is not in the keyring", active)
	}
	return k, nil
}

// ActiveKeyID returns the ID used for new seals and MACs.
func (k *Keyring) ActiveKeyID() string { return k.active }

// KeyIDs returns all configured key IDs, sorted.
func (k *Keyring) KeyIDs() []string {
	ids := make([]string, 0, len(k.keys))
	for id := range k.keys {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Seal encrypts plaintext with the active key. aad binds the ciphertext to its owner and purpose
// (e.g. org ID | integration ID | "integration-secret") so ciphertexts cannot be swapped.
// Layout: version(1) | len(kid)(1) | kid | nonce(12) | ciphertext+tag.
func (k *Keyring) Seal(plaintext, aad []byte) ([]byte, error) {
	gcm, err := k.aead(k.active)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, 2+len(k.active)+gcm.NonceSize()+len(plaintext)+gcm.Overhead())
	out = append(out, sealVersion, byte(len(k.active)))
	out = append(out, k.active...)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out = append(out, nonce...)
	return gcm.Seal(out, nonce, plaintext, aad), nil
}

// ErrOpen is returned for any decryption failure (tampering, wrong AAD or unknown key).
var ErrOpen = errors.New("keyring: ciphertext could not be opened")

// Open decrypts data produced by Seal and reports which key was used (for rotation).
func (k *Keyring) Open(sealed, aad []byte) (plaintext []byte, kid string, err error) {
	if len(sealed) < 2 || sealed[0] != sealVersion {
		return nil, "", ErrOpen
	}
	n := int(sealed[1])
	if len(sealed) < 2+n {
		return nil, "", ErrOpen
	}
	kid = string(sealed[2 : 2+n])
	gcm, err := k.aead(kid)
	if err != nil {
		return nil, "", ErrOpen
	}
	rest := sealed[2+n:]
	if len(rest) < gcm.NonceSize()+gcm.Overhead() {
		return nil, "", ErrOpen
	}
	pt, err := gcm.Open(nil, rest[:gcm.NonceSize()], rest[gcm.NonceSize():], aad)
	if err != nil {
		return nil, "", ErrOpen
	}
	return pt, kid, nil
}

// MAC computes HMAC-SHA-256 with a MAC key derived from the active key, returning the key ID to
// store beside the value (recovery codes keep working after rotation until regenerated).
func (k *Keyring) MAC(data []byte) (sum []byte, kid string, err error) {
	key, err := k.macKey(k.active)
	if err != nil {
		return nil, "", err
	}
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil), k.active, nil
}

// VerifyMAC checks a MAC produced by MAC under the given key ID in constant time.
func (k *Keyring) VerifyMAC(kid string, data, sum []byte) bool {
	key, err := k.macKey(kid)
	if err != nil {
		return false
	}
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return hmac.Equal(h.Sum(nil), sum)
}

func (k *Keyring) aead(kid string) (cipher.AEAD, error) {
	key, ok := k.keys[kid]
	if !ok {
		return nil, fmt.Errorf("keyring: unknown key %q", kid)
	}
	enc, err := hkdf.Key(sha256.New, key, nil, "matecollab/keyring/aead/v1", keySize)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(enc)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (k *Keyring) macKey(kid string) ([]byte, error) {
	key, ok := k.keys[kid]
	if !ok {
		return nil, fmt.Errorf("keyring: unknown key %q", kid)
	}
	return hkdf.Key(sha256.New, key, nil, "matecollab/keyring/mac/v1", keySize)
}

// GenerateKey returns a new random key encoded for MC_ENCRYPTION_KEYRING.
func GenerateKey() (string, error) {
	b := make([]byte, keySize)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
