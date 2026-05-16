package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) viewWelcome() string {
	logo := renderWelcomeLogo()
	tagline := renderWelcomeTagline()
	warn := lipgloss.NewStyle().Foreground(colorWarn).Bold(true).Render(L.MinorAlcoholWarning)

	prompt := styleText().Render(L.WelcomeBirthPrompt)
	input := styleInputCursor().Render("  " + m.state.birthDateInput + "█")
	keys := styleMuted().Render(L.WelcomeContinue)

	priv := fmt.Sprintf("%s %s",
		styleMuted().Render(L.WelcomePrivacyLabel),
		styleLink().Render(m.privacyPolicyURL))
	rights := fmt.Sprintf("%s %s",
		styleMuted().Render(L.WelcomeConsumerRightsLabel),
		styleLink().Render(m.consumerRightsURL))

	formInner := strings.Join([]string{
		warn,
		"",
		prompt,
		input,
		"",
		keys,
	}, "\n")
	form := renderWelcomeFrame(formInner)

	footer := strings.Join([]string{priv, rights}, "\n")

	var parts []string
	parts = append(parts, logo)
	parts = append(parts, tagline)
	parts = append(parts, "")
	parts = append(parts, form)
	parts = append(parts, "")
	parts = append(parts, footer)

	return strings.Join(parts, "\n")
}
