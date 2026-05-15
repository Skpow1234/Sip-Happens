package tui

import (
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// asciiQR renders a payment URL as a terminal-friendly QR code (2 cols per module).
func asciiQR(content string, maxWidth int) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	qr, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return ""
	}
	bitmap := qr.Bitmap()
	if len(bitmap) == 0 {
		return ""
	}
	// Each module uses 2 terminal columns; skip if wider than terminal.
	qrWidth := len(bitmap[0]) * 2
	if maxWidth > 0 && qrWidth > maxWidth {
		return "(QR too wide for terminal — use the URL below)\n"
	}
	var b strings.Builder
	for _, row := range bitmap {
		for _, on := range row {
			if on {
				b.WriteString("██")
			} else {
				b.WriteString("  ")
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}
