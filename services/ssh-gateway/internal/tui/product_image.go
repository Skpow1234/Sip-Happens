package tui

import "strings"

// asciiImagePlaceholder is a tiny terminal-friendly preview when a product has an image URL.
const asciiImagePlaceholder = `
 +------+
 | beer |
 +------+
`

func formatProductImageBlock(imageURL string) string {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return ""
	}
	link := truncateURL(imageURL, 72)
	return strings.TrimSpace(asciiImagePlaceholder) + "\nImage: " + link + "\n"
}

func truncateURL(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
