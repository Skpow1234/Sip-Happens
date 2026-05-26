package tui

import (
	"fmt"
	"strings"
	"time"

	"sip-happens/services/ssh-gateway/internal/api"
)

func tokenStatusLabel(t api.PersonalAccessTokenView, now time.Time) string {
	if t.RevokedAt != nil {
		return L.AccountTokenStatusRevoked
	}
	if t.ExpiresAt != nil && !t.ExpiresAt.After(now) {
		return L.AccountTokenStatusExpired
	}
	return L.AccountTokenStatusActive
}

// shortTokenPrefix shows the first 8 hex chars of the stored prefix so the
// list line stays narrow but is still recognisable next to the raw token a
// user might have in their clipboard / password manager.
func shortTokenPrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if len(prefix) <= 8 {
		return prefix
	}
	return prefix[:8]
}

func (m Model) accountTokensPane() string {
	if m.busy && len(m.state.tokens) == 0 {
		return L.Loading + "\n"
	}
	if len(m.state.tokens) == 0 {
		return L.AccountTokensEmpty + "\n\n" + L.AccountTokensHelp + "\n"
	}
	now := time.Now()
	var b strings.Builder
	for i, t := range m.state.tokens {
		line := L.AccountTokensListHeader(
			shortTokenPrefix(t.TokenPrefix),
			tokenDisplayName(t),
			len(t.Scopes),
			tokenStatusLabel(t, now),
		)
		b.WriteString(selectedLine("", line, i == m.state.tokenCursor))
	}
	b.WriteString("\n" + L.AccountTokensHelp + "\n")
	return b.String()
}

func tokenDisplayName(t api.PersonalAccessTokenView) string {
	if name := strings.TrimSpace(t.Name); name != "" {
		return name
	}
	return "(sin nombre)"
}

func (m Model) viewTokenCreate() string {
	d := m.state.tokenDraft
	cursor := "_"
	body := styleWordmark().Render(L.TokenCreateTitle) + "\n\n"
	body += L.TokenCreateNameLabel + "\n"
	body += "  " + d.name + cursor + "\n\n"
	body += L.TokenCreateScopesLabel + "\n"
	body += selectedLine("1", L.TokenCreatePresetRead, d.preset == tokenScopeReadOnly)
	body += selectedLine("2", L.TokenCreatePresetFull, d.preset == tokenScopeFullAccess)
	opts := m.layoutDefaults(tabAccount, L.TokenCreateFooter)
	return m.composeLayout(body, opts)
}

func (m Model) viewTokenShow() string {
	raw := strings.TrimSpace(m.state.lastTokenRaw)
	var body strings.Builder
	body.WriteString(styleWordmark().Render(L.TokenShowTitle) + "\n\n")
	body.WriteString(styleMuted().Render(L.TokenShowOnceWarning) + "\n\n")
	if v := m.state.lastTokenView; v != nil {
		body.WriteString(fmt.Sprintf("%s %s\n", L.TokenShowPrefixLabel, shortTokenPrefix(v.TokenPrefix)+"…"))
		body.WriteString(fmt.Sprintf("%s %s\n\n", L.TokenShowScopesLabel, strings.Join(v.Scopes, ", ")))
	}
	body.WriteString(L.TokenShowSecretLabel + "\n")
	body.WriteString("  " + formatSelectableURL(raw, m.width) + "\n\n")
	body.WriteString(styleMuted().Render(L.TokenShowCopyHint) + "\n")
	opts := m.layoutDefaults(tabAccount, L.TokenShowFooter)
	return m.composeLayout(body.String(), opts)
}
