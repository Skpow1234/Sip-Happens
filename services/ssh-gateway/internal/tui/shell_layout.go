package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	topBarCellCount   = 4
	topBarBorderChars = topBarCellCount - 1
	splitPaneGap      = 1
)

func (m Model) shellContentWidth() int {
	w := m.width
	if w < 40 {
		return 80
	}
	return w
}

func topBarCellWidth(total int) int {
	usable := total - topBarBorderChars
	if usable < topBarCellCount*8 {
		return 8
	}
	return usable / topBarCellCount
}

// catalogPaneWidths — narrow left rail like Terminal.shop (~¼ width).
func (m Model) catalogPaneWidths() (left, right int) {
	w := m.shellContentWidth()
	gap := splitPaneGap
	left = w / 4
	if left < 22 {
		left = 22
	}
	if left > 32 {
		left = 32
	}
	right = w - left - gap
	if right < 32 {
		left = w - gap - 32
		if left < 20 {
			left = 20
		}
		right = w - left - gap
	}
	return left, right
}

func fitFooterText(hints string, width int) string {
	if width <= 0 || len(hints) <= width {
		return hints
	}
	chunks := splitFooterChunks(hints)
	if len(chunks) == 0 {
		return truncate(hints, width)
	}
	var kept []string
	used := 0
	for _, c := range chunks {
		need := len(c)
		if len(kept) > 0 {
			need += 2
		}
		if used+need > width && len(kept) > 0 {
			break
		}
		kept = append(kept, c)
		used += need
	}
	if len(kept) == 0 {
		return truncate(hints, width)
	}
	return strings.Join(kept, "  ")
}

func clipLines(s string, maxLines int) string {
	if maxLines <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n")
}

func (m Model) shellChromeLines(opts layoutOpts) int {
	n := 0
	if opts.showTabs {
		n += 3
	}
	if m.renderProfileLinkBanner() != "" {
		n += 1
	}
	if opts.checkout != nil {
		n += 1
	}
	if m.notice != "" {
		n += 1
	}
	if opts.showPromo {
		n += 2
	}
	if opts.footer != "" {
		n += 2
	}
	return n
}

func (m Model) maxShellBodyLines(opts layoutOpts) int {
	if m.height <= 0 {
		return 0
	}
	max := m.height - m.shellChromeLines(opts)
	if max < 6 {
		return 6
	}
	return max
}

func splitHorizontal(leftWidth, rightWidth int, left, right string) string {
	l := lipgloss.NewStyle().Width(leftWidth).Render(left)
	sp := strings.Repeat(" ", splitPaneGap)
	r := lipgloss.NewStyle().Width(rightWidth).Render(right)
	return lipgloss.JoinHorizontal(lipgloss.Top, l, sp, r)
}
