package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/steviee/git-issues/internal/issue"
)

const (
	boardHelpText  = " h/l: column  j/k: navigate  H/L: move issue  Enter: details  q: quit"
	detailHelpText = " j/k: scroll  Esc: back  q: quit"
)

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}
	if m.showDetail {
		return m.detailView()
	}
	return m.boardView()
}

func (m Model) boardView() string {
	colWidth := m.width / len(m.columns)
	colHeight := m.height - boardStatusHeight

	cols := make([]string, len(m.columns))
	for i := range m.columns {
		cols[i] = m.columns[i].Render(colWidth, colHeight, i == m.activeCol)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, cols...) + "\n" + renderBoardStatus(m.err)
}

// renderBoardStatus renders the help line, prefixed by the last error if any.
func renderBoardStatus(err error) string {
	help := statusBarStyle.Render(boardHelpText)
	if err == nil {
		return help
	}
	return errorStyle.Render(fmt.Sprintf(" Error: %s", err)) + "\n" + help
}

func (m Model) detailView() string {
	if m.columns[m.activeCol].Selected() == nil {
		return "No issue selected"
	}

	content := m.detailVP.View() + "\n" + statusBarStyle.Render(detailHelpText)
	return detailBorderStyle.Width(m.width - detailFrameWidthMargin).Render(content)
}

// initDetailViewport sizes the viewport and fills it with the rendered issue.
func (m *Model) initDetailViewport(iss *issue.Issue) {
	contentWidth := m.width - detailContentInset
	if contentWidth < minDetailContentWidth {
		contentWidth = minDetailContentWidth
	}

	viewportHeight := m.height - detailViewportInset
	if viewportHeight < minDetailViewportHeight {
		viewportHeight = minDetailViewportHeight
	}

	m.detailVP = viewport.New(contentWidth, viewportHeight)
	m.detailVP.MouseWheelEnabled = true
	m.detailVP.SetContent(renderDetailContent(iss, contentWidth))
}

// renderDetailContent builds the detail header, metadata, relations and body.
func renderDetailContent(iss *issue.Issue, contentWidth int) string {
	separator := strings.Repeat("━", contentWidth)

	var b strings.Builder
	b.WriteString(separator + "\n")
	b.WriteString(detailTitleStyle.Render(
		fmt.Sprintf("Issue #%d  ·  %s  ·  %s", iss.ID, iss.Priority, iss.Status)) + "\n")
	b.WriteString(detailTitleStyle.Render(iss.Title) + "\n")
	b.WriteString(separator + "\n")

	if len(iss.Labels) > 0 {
		b.WriteString(detailFieldStyle.Render("Labels:   ") + strings.Join(iss.Labels, ", ") + "\n")
	}
	b.WriteString(detailFieldStyle.Render("Created:  ") + iss.Created + "\n")
	b.WriteString(detailFieldStyle.Render("Updated:  ") + iss.Updated + "\n")
	if iss.Closed != "" {
		b.WriteString(detailFieldStyle.Render("Closed:   ") + iss.Closed + "\n")
	}

	if hasRelations(iss.Relations) {
		b.WriteString("\n" + detailTitleStyle.Render("Relations") + "\n")
		writeRelation(&b, "  Blocks:     ", iss.Relations.Blocks)
		writeRelation(&b, "  Depends on: ", iss.Relations.DependsOn)
		writeRelation(&b, "  Related to: ", iss.Relations.RelatedTo)
		writeRelation(&b, "  Duplicates: ", iss.Relations.Duplicates)
	}

	if iss.Body != "" {
		b.WriteString("\n" + separator + "\n")
		b.WriteString(renderMarkdownBody(iss.Body, contentWidth))
	}

	return b.String()
}

func hasRelations(rel issue.Relations) bool {
	return len(rel.Blocks)+len(rel.DependsOn)+len(rel.RelatedTo)+len(rel.Duplicates) > 0
}

func writeRelation(b *strings.Builder, label string, ids []int) {
	if len(ids) == 0 {
		return
	}
	b.WriteString(detailFieldStyle.Render(label) + formatIDs(ids) + "\n")
}

// renderMarkdownBody renders the body with glamour and falls back to the raw
// text when glamour cannot render it.
func renderMarkdownBody(body string, width int) string {
	style := "dark"
	if !lipgloss.HasDarkBackground() {
		style = "light"
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return "\n" + body
	}
	rendered, err := renderer.Render(body)
	if err != nil {
		return "\n" + body
	}
	return rendered
}

func formatIDs(ids []int) string {
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = fmt.Sprintf("#%d", id)
	}
	return strings.Join(strs, ", ")
}
