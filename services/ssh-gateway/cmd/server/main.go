package main

import (
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/keygen"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	wishbubbletea "github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"

	"sip-happens/pkg/config"
	"sip-happens/services/ssh-gateway/internal/api"
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

	s, err := wish.NewServer(
		wish.WithAddress(cfg.SSHAddr),
		wish.WithHostKeyPath(cfg.HostKeyPath),
		wish.WithMiddleware(
			logging.Middleware(),
			wishbubbletea.Middleware(func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
				_ = s
				client := api.NewClient(cfg.CoreAPIURL)
				model := tui.NewRootModel(client)
				opts := []tea.ProgramOption{tea.WithAltScreen()}
				return model, opts
			}),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("ssh-gateway listening on %s", cfg.SSHAddr)
	log.Fatal(s.ListenAndServe())
}

