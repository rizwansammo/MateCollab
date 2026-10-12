package emails

import (
	"strings"
	"testing"
)

func TestEveryTemplateRendersWithItsData(t *testing.T) {
	full := map[string]string{
		"name": "Ayesha", "link": "http://localhost:8080/reset-password#token=abc", "org": "Northwind",
		"email": "ayesha@example.org", "inviter": "Daniel Chen", "role": "a member", "remaining": "9",
	}
	for _, tpl := range []Template{PasswordReset, AccountSetup, EmailVerification, Invitation, PasswordChanged,
		MFAEnabled, MFADisabled, RecoveryCodeUsed, RecoveryCodesRegenerated} {
		c, err := render(Message{Template: tpl, To: "a@example.org", Data: full})
		if err != nil {
			t.Fatalf("%s: %v", tpl, err)
		}
		if c.subject == "" || !strings.Contains(c.text(), "MateCollab") || !strings.HasPrefix(c.html(), "<!doctype html>") {
			t.Fatalf("%s rendered badly", tpl)
		}
	}
}

func TestMissingDataAndUnknownTemplatesFail(t *testing.T) {
	if _, err := render(Message{Template: PasswordReset, Data: map[string]string{"name": "A"}}); err == nil {
		t.Fatal("missing link accepted")
	}
	if _, err := render(Message{Template: "nope", Data: map[string]string{}}); err == nil {
		t.Fatal("unknown template accepted")
	}
}

func TestHTMLEscapesUserControlledValues(t *testing.T) {
	c, err := render(Message{Template: Invitation, Data: map[string]string{
		"inviter": `<script>alert(1)</script>`, "org": `"Org" & Co`, "role": "a member", "link": `http://x/invite#token="a"`,
	}})
	if err != nil {
		t.Fatal(err)
	}
	h := c.html()
	if strings.Contains(h, "<script>") || strings.Contains(h, `href="http://x/invite#token="a""`) {
		t.Fatalf("unescaped output: %s", h)
	}
}
