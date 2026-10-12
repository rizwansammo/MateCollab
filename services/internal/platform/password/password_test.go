package password

import (
	"context"
	"strings"
	"testing"
)

// Fast parameters keep the suite quick; production uses DefaultParams.
var testParams = Params{MemoryKiB: 1024, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32}

func TestHashAndVerify(t *testing.T) {
	ctx := context.Background()
	h := NewHasher(testParams, 2)
	stored, err := h.Hash(ctx, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "$argon2id$v=19$m=1024,t=1,p=1$") {
		t.Fatalf("unexpected PHC string %q", stored)
	}
	if ok, rehash, err := h.Verify(ctx, "correct horse battery", stored); err != nil || !ok || rehash {
		t.Fatalf("verify = %v %v %v", ok, rehash, err)
	}
	if ok, _, _ := h.Verify(ctx, "correct horse batterY", stored); ok {
		t.Fatal("wrong password accepted")
	}
	// Two hashes of the same password differ (random salt).
	again, _ := h.Hash(ctx, "correct horse battery")
	if again == stored {
		t.Fatal("salt not random")
	}
}

func TestNormalisationMakesEquivalentInputMatch(t *testing.T) {
	ctx := context.Background()
	h := NewHasher(testParams, 1)
	stored, _ := h.Hash(ctx, "ﬁnancial planning") // U+FB01 ligature
	if ok, _, _ := h.Verify(ctx, "financial planning", stored); !ok {
		t.Fatal("NFKC-equivalent password rejected")
	}
}

func TestRehashWhenParametersChange(t *testing.T) {
	ctx := context.Background()
	old := NewHasher(testParams, 1)
	stored, _ := old.Hash(ctx, "a long enough secret")
	stronger := testParams
	stronger.Time = 2
	if ok, rehash, _ := NewHasher(stronger, 1).Verify(ctx, "a long enough secret", stored); !ok || !rehash {
		t.Fatalf("ok=%v rehash=%v", ok, rehash)
	}
}

func TestMalformedHashes(t *testing.T) {
	h := NewHasher(testParams, 1)
	for _, bad := range []string{"", "$bcrypt$", "$argon2id$v=18$m=1,t=1,p=1$c2FsdA$a2V5", "$argon2id$v=19$m=x$a$b"} {
		if _, _, err := h.Verify(context.Background(), "x", bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestDummyVerifyDoesNotPanic(t *testing.T) {
	NewHasher(testParams, 1).VerifyDummy(context.Background(), "anything")
}

func TestPolicy(t *testing.T) {
	cases := map[string]struct {
		pw, email, want string
	}{
		"ok":           {"a long and memorable phrase", "", ""},
		"unicode ok":   {"পাসওয়ার্ড-লম্বা-বাক্য", "", ""},
		"too short":    {"short-one", "", CodeTooShort},
		"too long":     {strings.Repeat("x", 129), "", CodeTooLong},
		"common":       {"Password1234", "", CodeCommon},
		"email":        {"ayesha@example.org", "Ayesha@Example.org", CodeSameAs},
		"email local":  {"ayesha.rahman", "ayesha.rahman@example.org", CodeSameAs},
		"exactly 12":   {"twelve chars", "", ""},
		"exactly 128":  {strings.Repeat("y", 128), "", ""},
		"brand common": {"MateCollab1234", "", CodeCommon},
	}
	for name, tc := range cases {
		if got := Check(tc.pw, tc.email); got != tc.want {
			t.Errorf("%s: Check(%q) = %q, want %q", name, tc.pw, got, tc.want)
		}
	}
}
