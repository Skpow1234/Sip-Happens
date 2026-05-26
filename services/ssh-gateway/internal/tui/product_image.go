package tui

import "strings"

// formatProductImageBlock renders the product detail's image hint.
//
// Terminals can't display real bitmaps, and the previous ASCII frame
// (`+------+ | beer |`) was a fake placeholder that looked broken on
// every product regardless of whether the image actually loaded. We
// now render a single muted "Imagen: <url>" line when an image URL is
// configured, and nothing at all otherwise — far less visual noise.
func formatProductImageBlock(imageURL string) string {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return ""
	}
	link := truncateURL(imageURL, 72)
	return styleMuted().Render(L.ProductImageLabel+" "+link) + "\n"
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
