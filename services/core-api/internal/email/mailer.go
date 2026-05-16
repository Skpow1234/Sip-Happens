package email

import (
	"bytes"
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

// Message is a single outbound email.
type Message struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string // optional; when set, sends multipart/alternative
}

// Mailer delivers email. Implementations must not log secrets or full payment URLs in production logs.
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

// LogMailer writes email metadata to the default logger (development default).
type LogMailer struct{}

func (LogMailer) Send(ctx context.Context, msg Message) error {
	_ = ctx
	// Redact long URLs in body preview.
	preview := msg.TextBody
	if i := strings.Index(preview, "http"); i >= 0 {
		preview = preview[:i] + "[payment-url-redacted]"
	}
	fmt.Printf("email (log) to=%s subject=%q body_preview=%q\n", msg.To, msg.Subject, truncate(preview, 120))
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// SMTPMailer sends mail via SMTP (PLAIN or STARTTLS on port 587).
type SMTPMailer struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

func (m SMTPMailer) Send(ctx context.Context, msg Message) error {
	_ = ctx
	if m.Host == "" {
		return fmt.Errorf("smtp host is required")
	}
	if m.From == "" {
		return fmt.Errorf("smtp from address is required")
	}
	if msg.To == "" {
		return fmt.Errorf("missing recipient")
	}
	addr := fmt.Sprintf("%s:%d", m.Host, m.Port)
	raw := buildMIMERaw(m.From, msg)

	var auth smtp.Auth
	if m.Username != "" {
		auth = smtp.PlainAuth("", m.Username, m.Password, m.Host)
	}
	return smtp.SendMail(addr, auth, m.From, []string{msg.To}, []byte(raw))
}

func buildMIMERaw(from string, msg Message) string {
	if strings.TrimSpace(msg.HTMLBody) == "" {
		return fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
			from, msg.To, msg.Subject, msg.TextBody)
	}
	boundary := "sip-happens-boundary"
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", from)
	fmt.Fprintf(&buf, "To: %s\r\n", msg.To)
	fmt.Fprintf(&buf, "Subject: %s\r\n", msg.Subject)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/alternative; boundary=%s\r\n\r\n", boundary)
	fmt.Fprintf(&buf, "--%s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n", boundary, msg.TextBody)
	fmt.Fprintf(&buf, "--%s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n", boundary, msg.HTMLBody)
	fmt.Fprintf(&buf, "--%s--\r\n", boundary)
	return buf.String()
}
