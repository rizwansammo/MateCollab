// Package mailer sends transactional email through the platform SMTP server (Mailpit locally).
// Sending always happens from background jobs in production code paths; this package only knows
// how to deliver one message.
package mailer

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	gomail "github.com/wneessen/go-mail"

	"github.com/rizwansammo/MateCollab/services/internal/platform/config"
)

// Message is one outgoing email. Bodies are plain text plus an optional HTML alternative.
type Message struct {
	To        []string
	Subject   string
	Text      string
	HTML      string
	MessageID string // deterministic per logical email so retries can be de-duplicated downstream
}

// Sender delivers messages.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// SMTP is the go-mail based Sender.
type SMTP struct {
	cfg  config.SMTPConfig
	from *mail.Address
}

// New validates the configuration and returns an SMTP sender.
func New(cfg config.SMTPConfig) (*SMTP, error) {
	if cfg.Host == "" {
		return nil, errors.New("mailer: MC_SMTP_HOST is required")
	}
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("mailer: MC_SMTP_FROM is not a valid address: %w", err)
	}
	return &SMTP{cfg: cfg, from: from}, nil
}

func (s *SMTP) client() (*gomail.Client, error) {
	opts := []gomail.Option{gomail.WithPort(s.cfg.Port), gomail.WithTimeout(gomail.DefaultTimeout)}
	switch s.cfg.TLS {
	case "tls":
		opts = append(opts, gomail.WithSSL())
	case "starttls":
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSMandatory))
	default:
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	}
	if s.cfg.Username != "" {
		opts = append(opts,
			gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
			gomail.WithUsername(s.cfg.Username),
			gomail.WithPassword(s.cfg.Password))
	}
	return gomail.NewClient(s.cfg.Host, opts...)
}

// Send delivers one message.
func (s *SMTP) Send(ctx context.Context, m Message) error {
	if len(m.To) == 0 {
		return errors.New("mailer: no recipients")
	}
	for _, field := range append([]string{m.Subject, m.MessageID}, m.To...) {
		if strings.ContainsAny(field, "\r\n") {
			return errors.New("mailer: header values must not contain line breaks")
		}
	}
	msg := gomail.NewMsg()
	if err := msg.From(s.from.String()); err != nil {
		return err
	}
	if err := msg.To(m.To...); err != nil {
		return err
	}
	msg.Subject(m.Subject)
	if m.MessageID != "" {
		msg.SetMessageIDWithValue(m.MessageID)
	} else {
		msg.SetMessageID()
	}
	msg.SetBodyString(gomail.TypeTextPlain, m.Text)
	if m.HTML != "" {
		msg.AddAlternativeString(gomail.TypeTextHTML, m.HTML)
	}
	c, err := s.client()
	if err != nil {
		return err
	}
	return c.DialAndSendWithContext(ctx, msg)
}

// Ping opens and closes an SMTP session to verify the server is reachable.
func (s *SMTP) Ping(ctx context.Context) error {
	c, err := s.client()
	if err != nil {
		return err
	}
	if err := c.DialWithContext(ctx); err != nil {
		return err
	}
	return c.Close()
}
