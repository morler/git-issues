package cmd

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

func displayWidth(s string) int {
	return runewidth.StringWidth(s)
}

func padRight(s string, width int) string {
	if w := displayWidth(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

func truncateDisplay(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if displayWidth(s) <= width {
		return s
	}
	return runewidth.Truncate(s, width, "…")
}
