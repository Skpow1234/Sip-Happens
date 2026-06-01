package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const brandCursorBlinkInterval = 530 * time.Millisecond

type brandCursorBlinkMsg struct{}

func brandCursorBlinkCmd() tea.Cmd {
	return tea.Tick(brandCursorBlinkInterval, func(time.Time) tea.Msg {
		return brandCursorBlinkMsg{}
	})
}

func (m *Model) leaveBootstrap() {
	if m.screen == screenBootstrap {
		m.screen = screenCatalog
	}
}

func (m Model) renderBrandWordmark() string {
	wordmark := renderShellBrandWordmark()
	if !m.brandCursorOn {
		return wordmark
	}
	return wordmark + styleBrandCursor().Render("█")
}

func (m Model) viewBootstrap() string {
	body := strings.Join([]string{
		m.renderBrandWordmark(),
		"",
		renderWelcomeLogo(),
		renderWelcomeTagline(),
		"",
		styleMuted().Render(L.BootstrapHint),
	}, "\n")
	return m.placeStackedCenter(body, "")
}
