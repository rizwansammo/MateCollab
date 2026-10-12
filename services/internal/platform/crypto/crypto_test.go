package crypto

import (
	"bytes"
	"strings"
	"testing"
)

func testKeyring(t *testing.T, spec, active string) *Keyring {
	t.Helper()
	k, err := ParseKeyring(spec, active)
	if err != nil {
		t.Fatalf("ParseKeyring: %v", err)
	}
	return k
}

func newKey(t *testing.T) string {
	t.Helper()
	k, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestSealOpenRoundTripAndAADBinding(t *testing.T) {
	k := testKeyring(t, "k1:"+newKey(t), "k1")
	aad := []byte("org-1|integration-1|integration-secret")
	sealed, err := k.Seal([]byte("smtp-password"), aad)
	if err != nil {
		t.Fatal(err)
	}
	pt, kid, err := k.Open(sealed, aad)
	if err != nil || kid != "k1" || string(pt) != "smtp-password" {
		t.Fatalf("Open = %q, %q, %v", pt, kid, err)
	}
	if bytes.Contains(sealed, []byte("smtp-password")) {
		t.Fatal("ciphertext contains plaintext")
	}
	if _, _, err := k.Open(sealed, []byte("org-2|integration-1|integration-secret")); err != ErrOpen {
		t.Fatalf("swapped AAD must fail, got %v", err)
	}
	sealed[len(sealed)-1] ^= 0xff
	if _, _, err := k.Open(sealed, aad); err != ErrOpen {
		t.Fatalf("tampered ciphertext must fail, got %v", err)
	}
}

func TestRotationKeepsOldDataReadable(t *testing.T) {
	old, next := newKey(t), newKey(t)
	k1 := testKeyring(t, "k1:"+old, "k1")
	sealed, _ := k1.Seal([]byte("secret"), nil)
	sum, kid, _ := k1.MAC([]byte("recovery-code"))

	k2 := testKeyring(t, "k1:"+old+",k2:"+next, "k2")
	if pt, gotKID, err := k2.Open(sealed, nil); err != nil || gotKID != "k1" || string(pt) != "secret" {
		t.Fatalf("old ciphertext after rotation: %q %q %v", pt, gotKID, err)
	}
	if !k2.VerifyMAC(kid, []byte("recovery-code"), sum) {
		t.Fatal("MAC under the old key must still verify after rotation")
	}
	if _, newKID, _ := k2.MAC([]byte("x")); newKID != "k2" {
		t.Fatalf("new MACs must use the active key, got %q", newKID)
	}
}

func TestParseKeyringRejectsBadInput(t *testing.T) {
	cases := map[string][2]string{
		"short key":      {"k1:" + "c2hvcnQ=", "k1"},
		"unknown active": {"k1:" + newKey(t), "k9"},
		"bad kid":        {"K 1:" + newKey(t), "K 1"},
		"empty":          {"", "k1"},
	}
	for name, c := range cases {
		if _, err := ParseKeyring(c[0], c[1]); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestSubkeysAreDistinctAndRequireLength(t *testing.T) {
	if _, err := DeriveSubkeys("too-short"); err == nil {
		t.Fatal("short secret must be rejected")
	}
	s, err := DeriveSubkeys(strings.Repeat("x", 48))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(s.CSRF, s.Cursor) || bytes.Equal(s.CSRF, s.WSTicket) || bytes.Equal(s.Cursor, s.WSTicket) {
		t.Fatal("subkeys must differ per purpose")
	}
}

func TestRandomToken(t *testing.T) {
	a, _ := RandomToken(32)
	b, _ := RandomToken(32)
	if a == b || len(a) != 43 {
		t.Fatalf("unexpected tokens %q %q", a, b)
	}
	if _, err := RandomToken(8); err == nil {
		t.Fatal("tokens below 128 bits must be rejected")
	}
}
