package email

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds email provider settings from environment variables.
type Config struct {
	Provider string // log | smtp
	From     string

	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
}

func LoadConfigFromEnv() (Config, error) {
	c := Config{
		Provider: strings.ToLower(strings.TrimSpace(os.Getenv("EMAIL_PROVIDER"))),
		From:     strings.TrimSpace(os.Getenv("EMAIL_FROM")),
		SMTPHost: strings.TrimSpace(os.Getenv("SMTP_HOST")),
		SMTPUser: strings.TrimSpace(os.Getenv("SMTP_USER")),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
	}
	if c.Provider == "" {
		c.Provider = "log"
	}
	if c.From == "" {
		c.From = "noreply@sip-happens.local"
	}
	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	if port == "" {
		c.SMTPPort = 587
	} else {
		p, err := strconv.Atoi(port)
		if err != nil {
			return c, fmt.Errorf("invalid SMTP_PORT: %w", err)
		}
		c.SMTPPort = p
	}
	return c, nil
}

// NewMailerFromEnv returns a Mailer for the configured provider.
func NewMailerFromEnv() (Mailer, error) {
	cfg, err := LoadConfigFromEnv()
	if err != nil {
		return nil, err
	}
	switch cfg.Provider {
	case "log":
		return LogMailer{}, nil
	case "smtp":
		if cfg.SMTPHost == "" {
			return nil, fmt.Errorf("SMTP_HOST is required when EMAIL_PROVIDER=smtp")
		}
		return SMTPMailer{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUser,
			Password: cfg.SMTPPassword,
			From:     cfg.From,
		}, nil
	default:
		return nil, fmt.Errorf("unknown EMAIL_PROVIDER %q (use log or smtp)", cfg.Provider)
	}
}
