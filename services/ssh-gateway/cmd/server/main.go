package main

import (
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/keygen"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	wishbubbletea "github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"

	"sip-happens/pkg/config"
	"sip-happens/services/ssh-gateway/internal/api"
	"sip-happens/services/ssh-gateway/internal/security"
	"sip-happens/services/ssh-gateway/internal/tui"
)

func main() {
	cfg, err := config.LoadSSHGateway()
	if err != nil {
		log.Fatal(err)
	}

	// Ensure a persistent host key exists on disk.
	if _, err := keygen.New(cfg.HostKeyPath, keygen.WithWrite()); err != nil {
		log.Fatal(err)
	}

	opts := []ssh.Option{
		wish.WithAddress(cfg.SSHAddr),
		wish.WithHostKeyPath(cfg.HostKeyPath),
	}
	opts = append(opts, security.SSHOptions(cfg.RequireAuthorizedKeys, cfg.AuthorizedKeysPath)...)
	opts = append(opts,
		wish.WithMiddleware(
			logging.Middleware(),
			(&security.SSHRateLimiter{Window: time.Minute, Limit: 30}).Middleware(),
			wishbubbletea.Middleware(func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
				fp := security.SessionFingerprint(s)
				if fp == "" && cfg.DevFingerprint != "" {
					fp = cfg.DevFingerprint
				}
				var client *api.Client
				if cfg.CoreAPIHMACKeyID != "" && len(cfg.CoreAPIHMACKeys) > 0 {
					if sec, ok := cfg.CoreAPIHMACKeys[cfg.CoreAPIHMACKeyID]; ok && sec != "" {
						client = api.NewSignedClientV2(cfg.CoreAPIURL, cfg.CoreAPIHMACKeyID, sec, fp)
					}
				}
				if client == nil && cfg.CoreAPIHMACSecret != "" {
					client = api.NewSignedClient(cfg.CoreAPIURL, cfg.CoreAPIHMACSecret, fp)
				}
				if client == nil {
					client = api.NewClient(cfg.CoreAPIURL)
				}
				model := tui.NewRootModel(client)
				opts := []tea.ProgramOption{tea.WithAltScreen()}
				return model, opts
			}),
		),
	)

	s, err := wish.NewServer(opts...)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("ssh-gateway listening on %s (authorized_keys=%v)", cfg.SSHAddr, cfg.RequireAuthorizedKeys)
	log.Fatal(s.ListenAndServe())
}
