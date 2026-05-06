package main

import (
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/keygen"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	wishbubbletea "github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"

	"sip-happens/services/ssh-gateway/internal/api"
	"sip-happens/services/ssh-gateway/internal/tui"
)

func main() {
	addr := env("SSH_ADDR", "0.0.0.0:2222")
	coreAPI := env("CORE_API_URL", "http://core-api:8080")
	hostKeyPath := env("SSH_HOST_KEY_PATH", "/data/ssh_host_ed25519_key")

	// Ensure a persistent host key exists on disk.
	if _, err := keygen.New(hostKeyPath, keygen.WithWrite()); err != nil {
		log.Fatal(err)
	}

	s, err := wish.NewServer(
		wish.WithAddress(addr),
		wish.WithHostKeyPath(hostKeyPath),
		wish.WithMiddleware(
			logging.Middleware(),
			wishbubbletea.Middleware(func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
				_ = s
				client := api.NewClient(coreAPI)
				model := tui.NewRootModel(client)
				opts := []tea.ProgramOption{tea.WithAltScreen()}
				return model, opts
			}),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("ssh-gateway listening on %s", addr)
	log.Fatal(s.ListenAndServe())
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

