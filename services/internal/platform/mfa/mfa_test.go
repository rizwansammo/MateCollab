package mfa

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

// RFC 6238 Appendix B test vectors (SHA-1 seed "12345678901234567890"); the RFC lists 8-digit
// values, of which the 6-digit code is the last six digits.
func TestRFC6238Vectors(t *testing.T) {
	secret := []byte("12345678901234567890")
	cases := map[int64]string{
		59:          "287082",
		1111111109:  "081804",
		1111111111:  "050471",
		1234567890:  "005924",
		2000000000:  "279037",
		20000000000: "353130",
	}
	for unix, want := range cases {
		if got := Code(secret, uint64(Step(time.Unix(unix, 0)))); got != want {
			t.Errorf("t=%d: got %s, want %s", unix, got, want)
		}
	}
}

func TestVerifyWindowAndReplay(t *testing.T) {
	secret, err := NewSecret()
	if err != nil || len(secret) != SecretBytes {
		t.Fatal("secret generation failed")
	}
	now := time.Unix(1_800_000_000, 0)
	current := Step(now)
	code := Code(secret, uint64(current))

	step, ok := Verify(secret, code, now, nil)
	if !ok || step != current {
		t.Fatalf("current code rejected")
	}
	// The previous and next step are accepted (clock drift), two steps away are not.
	if _, ok := Verify(secret, Code(secret, uint64(current-1)), now, nil); !ok {
		t.Fatal("previous step rejected")
	}
	if _, ok := Verify(secret, Code(secret, uint64(current+1)), now, nil); !ok {
		t.Fatal("next step rejected")
	}
	if _, ok := Verify(secret, Code(secret, uint64(current-2)), now, nil); ok {
		t.Fatal("code two steps old accepted")
	}
	// Replay: once a step is used, it and earlier steps are refused.
	if _, ok := Verify(secret, code, now, &current); ok {
		t.Fatal("replayed code accepted")
	}
	// Formatting tolerance and garbage.
	if _, ok := Verify(secret, " "+code[:3]+" "+code[3:]+" ", now, nil); !ok {
		t.Fatal("spaced code rejected")
	}
	for _, bad := range []string{"", "12345", "1234567", "abcdef"} {
		if _, ok := Verify(secret, bad, now, nil); ok {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestURIAndQR(t *testing.T) {
	secret := []byte("12345678901234567890")
	uri := URI("MateCollab", "ayesha@example.org", secret)
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "otpauth" || u.Host != "totp" {
		t.Fatalf("bad uri %q", uri)
	}
	if u.Query().Get("secret") != "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" || u.Query().Get("issuer") != "MateCollab" {
		t.Fatalf("bad query %q", u.RawQuery)
	}
	png, err := QRDataURL(uri)
	if err != nil || !strings.HasPrefix(png, "data:image/png;base64,") {
		t.Fatalf("qr: %v", err)
	}
}

func TestRecoveryCodes(t *testing.T) {
	codes, err := NewRecoveryCodes()
	if err != nil || len(codes) != RecoveryCodeCount {
		t.Fatal("generation failed")
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if len(c) != 11 || c[5] != '-' {
			t.Fatalf("bad format %q", c)
		}
		n := NormalizeRecoveryCode(c)
		if n == "" || seen[n] {
			t.Fatalf("normalise/unique failed for %q", c)
		}
		seen[n] = true
		// Lowercase, missing dash and spaces still match.
		if NormalizeRecoveryCode(strings.ToLower(strings.ReplaceAll(c, "-", " "))) != n {
			t.Fatalf("lenient input failed for %q", c)
		}
	}
	if NormalizeRecoveryCode("ABCDE-FGHIO") != "ABCDEFGH10" {
		t.Fatal("Crockford aliases not applied")
	}
	for _, bad := range []string{"", "ABCDE", "ABCDE-FGHIJ-K", "ABCDE-FGH!J"} {
		if NormalizeRecoveryCode(bad) != "" {
			t.Fatalf("accepted %q", bad)
		}
	}
}
