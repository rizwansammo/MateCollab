package mfa

import (
	"crypto/rand"
	"strings"
)

// RecoveryCodeCount is how many single-use codes are issued at a time.
const RecoveryCodeCount = 10

// Crockford base32 (no I, L, O, U): readable aloud and unambiguous on paper.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewRecoveryCodes returns RecoveryCodeCount codes of 10 Crockford characters (50 bits each),
// formatted XXXXX-XXXXX for display.
func NewRecoveryCodes() ([]string, error) {
	codes := make([]string, RecoveryCodeCount)
	buf := make([]byte, 10)
	for i := range codes {
		if _, err := rand.Read(buf); err != nil {
			return nil, err
		}
		var sb strings.Builder
		for j, b := range buf {
			if j == 5 {
				sb.WriteByte('-')
			}
			sb.WriteByte(crockford[b&31])
		}
		codes[i] = sb.String()
	}
	return codes, nil
}

// NormalizeRecoveryCode maps user input to the canonical form that is hashed: uppercase,
// separators removed, and the Crockford aliases (O→0, I/L→1) applied. It returns "" for input
// that cannot be a recovery code.
func NormalizeRecoveryCode(in string) string {
	var sb strings.Builder
	for _, r := range strings.ToUpper(in) {
		switch {
		case r == '-' || r == ' ':
			continue
		case r == 'O':
			sb.WriteByte('0')
		case r == 'I' || r == 'L':
			sb.WriteByte('1')
		case strings.ContainsRune(crockford, r):
			sb.WriteRune(r)
		default:
			return ""
		}
	}
	if sb.Len() != 10 {
		return ""
	}
	return sb.String()
}
