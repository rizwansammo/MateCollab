// Package emails sends platform emails (verification, password reset, invitations, security
// notices) through a River job, so delivery is durable and retried, and only happens when the
// enclosing transaction commits (BACKEND.md §3, §7).
//
// Message data can contain one-time links, so it is sealed with the keyring before it is stored
// in the job table; only the worker can open it. Rendered content never includes message
// bodies or other organisation content (ADR-020).
package emails

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/rizwansammo/MateCollab/services/internal/platform/crypto"
	"github.com/rizwansammo/MateCollab/services/internal/platform/jobs"
	"github.com/rizwansammo/MateCollab/services/internal/platform/mailer"
)

// Template names a platform email.
type Template string

// Platform email templates.
const (
	PasswordReset            Template = "password_reset"
	AccountSetup             Template = "account_setup"
	EmailVerification        Template = "email_verification"
	Invitation               Template = "invitation"
	PasswordChanged          Template = "password_changed"
	MFAEnabled               Template = "mfa_enabled"
	MFADisabled              Template = "mfa_disabled"
	RecoveryCodeUsed         Template = "recovery_code_used"
	RecoveryCodesRegenerated Template = "recovery_codes_regenerated"
)

// Message is one email to send. Data values are plain strings; templates decide what to show.
type Message struct {
	Template Template
	To       string
	Data     map[string]string
}

// Args is the River job. Sealed holds the encrypted Data.
type Args struct {
	Template Template `json:"template"`
	To       string   `json:"to"`
	Sealed   []byte   `json:"sealed"`
}

// Kind is the River job kind.
func (Args) Kind() string { return "email.send" }

// InsertOpts routes emails to their own queue.
func (Args) InsertOpts() river.InsertOpts { return river.InsertOpts{Queue: "email", MaxAttempts: 8} }

func aad(t Template, to string) []byte {
	return []byte("email:" + string(t) + ":" + strings.ToLower(to))
}

// Outbox enqueues emails inside the caller's transaction.
type Outbox struct {
	jobs    jobs.Enqueuer
	keyring *crypto.Keyring
}

// NewOutbox returns an outbox writing jobs through enq.
func NewOutbox(enq jobs.Enqueuer, keyring *crypto.Keyring) *Outbox {
	return &Outbox{jobs: enq, keyring: keyring}
}

// Enqueue schedules m for delivery when tx commits.
func (o *Outbox) Enqueue(ctx context.Context, tx pgx.Tx, m Message) error {
	if _, err := render(m); err != nil { // fail fast on unknown templates or missing data
		return err
	}
	raw, err := json.Marshal(m.Data)
	if err != nil {
		return err
	}
	sealed, err := o.keyring.Seal(raw, aad(m.Template, m.To))
	if err != nil {
		return err
	}
	_, err = o.jobs.InsertTx(ctx, tx, Args{Template: m.Template, To: m.To, Sealed: sealed}, nil)
	return err
}

// Sender delivers rendered email (the platform SMTP mailer).
type Sender interface {
	Send(ctx context.Context, m mailer.Message) error
}

// Worker renders and sends queued emails.
type Worker struct {
	river.WorkerDefaults[Args]
	Sender  Sender
	Keyring *crypto.Keyring
	Logger  *slog.Logger
}

// Work opens, renders and sends one email. Addresses are never logged.
func (w *Worker) Work(ctx context.Context, job *river.Job[Args]) error {
	raw, _, err := w.Keyring.Open(job.Args.Sealed, aad(job.Args.Template, job.Args.To))
	if err != nil {
		// A key that can no longer open the payload will never succeed: stop retrying.
		return river.JobCancel(fmt.Errorf("email: cannot open payload: %w", err))
	}
	var data map[string]string
	if err := json.Unmarshal(raw, &data); err != nil {
		return river.JobCancel(err)
	}
	c, err := render(Message{Template: job.Args.Template, To: job.Args.To, Data: data})
	if err != nil {
		return river.JobCancel(err)
	}
	if err := w.Sender.Send(ctx, mailer.Message{To: []string{job.Args.To}, Subject: c.subject, Text: c.text(), HTML: c.html()}); err != nil {
		return err
	}
	w.Logger.InfoContext(ctx, "email sent", slog.String("template", string(job.Args.Template)), slog.Int64("job_id", job.ID))
	return nil
}

type content struct {
	subject     string
	greeting    string
	paragraphs  []string
	actionLabel string
	actionURL   string
	after       []string
}

var errMissing = errors.New("emails: missing template data")

func render(m Message) (content, error) {
	d := m.Data
	need := func(keys ...string) error {
		for _, k := range keys {
			if strings.TrimSpace(d[k]) == "" {
				return fmt.Errorf("%w: %s.%s", errMissing, m.Template, k)
			}
		}
		return nil
	}
	hi := "Hi " + d["name"] + ","
	switch m.Template {
	case PasswordReset:
		return content{
			subject: "Reset your MateCollab password", greeting: hi,
			paragraphs:  []string{"We received a request to reset the password for your MateCollab account. The link works once and expires in 60 minutes."},
			actionLabel: "Reset password", actionURL: d["link"],
			after: []string{"If you didn't ask for this, ignore this email. Your password stays the same."},
		}, need("name", "link")
	case AccountSetup:
		p := "An account has been created for you on MateCollab."
		if d["org"] != "" {
			p = "An account has been created for you in " + d["org"] + " on MateCollab."
		}
		return content{
			subject: "Set up your MateCollab account", greeting: hi,
			paragraphs:  []string{p, "Choose a password to get started. The link works once and expires in 72 hours."},
			actionLabel: "Choose a password", actionURL: d["link"],
		}, need("name", "link")
	case EmailVerification:
		return content{
			subject: "Confirm your email address", greeting: hi,
			paragraphs:  []string{"Confirm that " + d["email"] + " belongs to you. The link expires in 24 hours."},
			actionLabel: "Confirm email address", actionURL: d["link"],
			after: []string{"If you didn't add this address to MateCollab, ignore this email."},
		}, need("name", "email", "link")
	case Invitation:
		return content{
			subject:     d["inviter"] + " invited you to " + d["org"] + " on MateCollab",
			greeting:    "Hello,",
			paragraphs:  []string{d["inviter"] + " invited you to join " + d["org"] + " on MateCollab as " + d["role"] + ". The invitation expires in 7 days."},
			actionLabel: "Accept invitation", actionURL: d["link"],
			after: []string{"If you weren't expecting this invitation, you can ignore this email."},
		}, need("inviter", "org", "role", "link")
	case PasswordChanged:
		return content{
			subject: "Your MateCollab password was changed", greeting: hi,
			paragraphs: []string{"The password for your MateCollab account was changed. Your other sessions were signed out."},
			after:      []string{"If you didn't do this, reset your password straight away and contact your administrator."},
		}, need("name")
	case MFAEnabled:
		return content{
			subject: "Two-step verification is on", greeting: hi,
			paragraphs: []string{"Two-step verification is now on for your MateCollab account. Keep your recovery codes somewhere safe."},
			after:      []string{"If you didn't do this, contact your administrator."},
		}, need("name")
	case MFADisabled:
		return content{
			subject: "Two-step verification was turned off", greeting: hi,
			paragraphs: []string{"Two-step verification was turned off for your MateCollab account."},
			after:      []string{"If you didn't do this, reset your password and contact your administrator."},
		}, need("name")
	case RecoveryCodeUsed:
		return content{
			subject: "A recovery code was used to sign in", greeting: hi,
			paragraphs: []string{"A recovery code was used to sign in to your MateCollab account. Recovery codes left: " + d["remaining"] + "."},
			after:      []string{"If this wasn't you, reset your password and contact your administrator."},
		}, need("name", "remaining")
	case RecoveryCodesRegenerated:
		return content{
			subject: "New recovery codes were created", greeting: hi,
			paragraphs: []string{"New recovery codes were created for your MateCollab account. Your previous codes no longer work."},
		}, need("name")
	}
	return content{}, fmt.Errorf("emails: unknown template %q", m.Template)
}

func (c content) text() string {
	var b strings.Builder
	b.WriteString(c.greeting + "\n\n")
	for _, p := range c.paragraphs {
		b.WriteString(p + "\n\n")
	}
	if c.actionURL != "" {
		b.WriteString(c.actionLabel + ": " + c.actionURL + "\n\n")
	}
	for _, p := range c.after {
		b.WriteString(p + "\n\n")
	}
	b.WriteString("— MateCollab\n")
	return b.String()
}

func (c content) html() string {
	e := html.EscapeString
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="en-GB"><body style="margin:0;padding:24px;background:#f7f8fa;font-family:system-ui,-apple-system,'Segoe UI',Roboto,Arial,sans-serif;color:#253248;font-size:14px;line-height:1.6">`)
	b.WriteString(`<div style="max-width:520px;margin:0 auto;background:#ffffff;border:1px solid #e5e9ef;border-radius:8px;padding:28px">`)
	b.WriteString(`<p style="margin:0 0 20px;font-weight:650;color:#7047d1">MateCollab</p>`)
	b.WriteString(`<p style="margin:0 0 14px">` + e(c.greeting) + `</p>`)
	for _, p := range c.paragraphs {
		b.WriteString(`<p style="margin:0 0 14px">` + e(p) + `</p>`)
	}
	if c.actionURL != "" {
		b.WriteString(`<p style="margin:22px 0"><a href="` + e(c.actionURL) + `" style="display:inline-block;background:#7047d1;color:#ffffff;text-decoration:none;font-weight:550;padding:9px 16px;border-radius:6px">` + e(c.actionLabel) + `</a></p>`)
		b.WriteString(`<p style="margin:0 0 14px;font-size:12px;color:#667185">Or open this link: ` + e(c.actionURL) + `</p>`)
	}
	for _, p := range c.after {
		b.WriteString(`<p style="margin:0 0 14px;color:#667185">` + e(p) + `</p>`)
	}
	b.WriteString(`</div></body></html>`)
	return b.String()
}
