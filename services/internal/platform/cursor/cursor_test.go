package cursor

import "testing"

type pos struct {
	Name string `json:"n"`
	ID   string `json:"i"`
}

func TestRoundTripAndTamperResistance(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	c, err := Encode(key, "members?role=admin", pos{Name: "ayesha", ID: "42"})
	if err != nil {
		t.Fatal(err)
	}
	var got pos
	if err := Decode(key, "members?role=admin", c, &got); err != nil || got.Name != "ayesha" {
		t.Fatalf("round trip: %v %+v", err, got)
	}
	if Decode(key, "members?role=guest", c, &got) == nil {
		t.Fatal("cursor reused with different filters")
	}
	if Decode([]byte("another-key-another-key-another-k"), "members?role=admin", c, &got) == nil {
		t.Fatal("cursor accepted under another key")
	}
	tampered := "eyJuIjoiemVkIiwiaSI6IjQyIn0" + c[len("eyJuIjoiYXllc2hhIiwiaSI6IjQyIn0"):]
	if Decode(key, "members?role=admin", tampered, &got) == nil {
		t.Fatal("tampered payload accepted")
	}
	for _, bad := range []string{"", "abc", "a.b"} {
		if Decode(key, "members", bad, &got) == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
