package main

import (
	"context"
	"log"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/keygen"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	wishbubbletea "github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"

	"sip-happens/pkg/config"
	"sip-happens/pkg/healthcheck"
	"sip-happens/services/ssh-gateway/internal/api"
	"sip-happens/services/ssh-gateway/internal/security"
	"sip-happens/services/ssh-gateway/internal/tui"
)

func main() {
	cfg, err := config.LoadSSHGateway()
	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) >= 2 && os.Args[1] == "healthcheck" {
		ctx, cancel := context.WithTimeout(context.Background(), healthcheck.ProbeTimeout)
		defer cancel()
		if err := healthcheck.TCPDialOK(ctx, cfg.SSHAddr); err != nil {
			os.Exit(1)
		}
		return
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
	if cfg.DisablePortForwarding {
		opts = append(opts, security.SSHPortForwardingOptions()...)
	}
	if cfg.DisablePasswordAuth {
		opts = append(opts, security.SSHDisablePasswordAuth())
	}
	opts = append(opts,
		wish.WithMiddleware(
			logging.Middleware(),
			security.SessionRestrictMiddleware(),
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
				model := tui.NewRootModel(client, cfg.PrivacyPolicyURL)
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
