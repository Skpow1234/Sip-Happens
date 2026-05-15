package tui

import "strings"

// formatSelectableURL breaks a long URL into lines for easier terminal selection/copy.
func formatSelectableURL(url string, lineWidth int) string {
	url = strings.TrimSpace(url)
	if url == "" {
		return ""
	}
	if lineWidth < 20 {
		lineWidth = 60
	}
	if len(url) <= lineWidth {
		return url
	}
	var b strings.Builder
	for len(url) > lineWidth {
		cut := lineWidth
		// Prefer breaking at / or ?
		if i := strings.LastIndex(url[:lineWidth], "/"); i > lineWidth/2 {
			cut = i + 1
		}
		b.WriteString(url[:cut])
		b.WriteByte('\n')
		url = url[cut:]
	}
	b.WriteString(url)
	return b.String()
}
