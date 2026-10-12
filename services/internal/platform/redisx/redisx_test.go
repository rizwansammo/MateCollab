package redisx

import "testing"

func TestKeyNamespacing(t *testing.T) {
	if got := Key("org", "1234", "presence"); got != "mc:org:1234:presence" {
		t.Fatalf("got %q", got)
	}
	if got := Key(); got != "mc" {
		t.Fatalf("got %q", got)
	}
}

func TestNewRejectsBadURL(t *testing.T) {
	if _, err := New("http://not-redis"); err == nil {
		t.Fatal("expected error")
	}
	if c, err := New("redis://:secret@localhost:6379/0"); err != nil || c == nil {
		t.Fatalf("valid URL rejected: %v", err)
	}
}
