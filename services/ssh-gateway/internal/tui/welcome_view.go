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
		m.renderBrandWordmark(),
		styleMuted().Render(L.SplashTagline),
	}, "\n")
	return m.placeStackedCenter(body, "")
}

func (m Model) viewWelcome() string {
	warn := lipgloss.NewStyle().Foreground(colorWarn).Render(L.MinorAlcoholWarning)
	prompt := styleMuted().Render(L.WelcomeBirthPrompt)
	input := renderBirthDateInput(m.state.birthDateInput)
	keys := formatFooterHints(L.WelcomeContinue)

	priv := styleMuted().Render(L.WelcomePrivacyLabel) + " " + styleLink().Render(m.privacyPolicyURL)
	rights := styleMuted().Render(L.WelcomeConsumerRightsLabel) + " " + styleLink().Render(m.consumerRightsURL)

	formInner := strings.Join([]string{warn, "", prompt, input, "", keys}, "\n")
	form := styleWelcomeFrame().Render(formInner)

	var mainParts []string
	mainParts = append(mainParts, renderShellBrandWordmark())
	if !m.welcomeIsCompact() {
		mainParts = append(mainParts, "", renderWelcomeLogo(), renderWelcomeTagline())
	}
	mainParts = append(mainParts, "", form)
	main := strings.Join(mainParts, "\n")

	footer := strings.Join([]string{priv, rights}, "\n")
	return m.placeStackedCenter(main, footer)
}
