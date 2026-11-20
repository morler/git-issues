package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/steviee/git-issues/internal/issue"
)

// RenderCard renders an issue as a card for the board.
func RenderCard(iss *issue.Issue, width int, selected bool) string {
	style := cardStyle.Width(width - cardBorderWidth) // account for border
	if selected {
		style = selectedCardStyle.Width(width - cardBorderWidth)
	}

	// Priority indicator
	priColor, ok := priorityColors[iss.Priority]
	if !ok {
		priColor = lipgloss.Color("248")
	}
	priStyle := lipgloss.NewStyle().Foreground(priColor).Bold(true)
	priStr := priStyle.Render(strings.ToUpper(iss.Priority))

	// ID line
	idLine := fmt.Sprintf("#%-*d %s", cardIDWidth, iss.ID, priStr)

	// Title (truncate to fit, by display width so multi-byte text is not cut)
	maxTitleLen := width - cardTextInset
	title := iss.Title
	if maxTitleLen > 0 {
		title = runewidth.Truncate(title, maxTitleLen, "…")
	}

	// Labels
	var content string
	if len(iss.Labels) > 0 {
		labels := labelStyle.Render(strings.Join(iss.Labels, ", "))
		content = fmt.Sprintf("%s\n%s\n%s", idLine, title, labels)
	} else {
		content = fmt.Sprintf("%s\n%s", idLine, title)
	}

	return style.Render(content)
}
