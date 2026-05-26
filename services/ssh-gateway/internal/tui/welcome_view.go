package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// viewSplash is the brief first-paint shown before the age form. It contains
// no legal copy on purpose — the alcohol warning still appears 600 ms later
// on the welcome screen, this is just the brand "sip-happens█" + tagline so
// the very first frame the user sees is unmistakably ours.
func (m Model) viewSplash() string {
	body := strings.Join([]string{
		renderWelcomeWordmark(),
		styleMuted().Render(L.SplashTagline),
	}, "\n")
	if m.width > 0 {
		return lipgloss.Place(m.width, 0, lipgloss.Center, lipgloss.Top, body)
	}
	return body
}

func (m Model) viewWelcome() string {
	warn := lipgloss.NewStyle().Foreground(colorWarn).Render(L.MinorAlcoholWarning)
	prompt := styleMuted().Render(L.WelcomeBirthPrompt)
	input := styleText().Render("  ") + styleInputCursor().Render(m.state.birthDateInput+"█")
	keys := formatFooterHints(L.WelcomeContinue)

	priv := styleMuted().Render(L.WelcomePrivacyLabel) + " " + styleLink().Render(m.privacyPolicyURL)
	rights := styleMuted().Render(L.WelcomeConsumerRightsLabel) + " " + styleLink().Render(m.consumerRightsURL)

	formInner := strings.Join([]string{warn, "", prompt, input, "", keys}, "\n")
	form := styleWelcomeFrame().Render(formInner)

	body := strings.Join([]string{
		renderWelcomeWordmark(),
		"",
		renderWelcomeLogo(),
		renderWelcomeTagline(),
		"",
		form,
		"",
		priv,
		rights,
	}, "\n")

	if m.width > 0 {
		return lipgloss.Place(m.width, 0, lipgloss.Center, lipgloss.Top, body)
	}
	return body
}
