package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
	"github.com/steviee/git-issues/internal/config"
	"github.com/steviee/git-issues/internal/git"
	"github.com/steviee/git-issues/internal/issue"
)

// statusOrder defines the column order (left to right).
var statusOrder = []string{"open", "in-progress", "closed", "wontfix"}

// statusIndex returns the index in statusOrder, or -1.
func statusIndex(status string) int {
	for i, s := range statusOrder {
		if s == status {
			return i
		}
	}
	return -1
}

// savedMsg is sent after an issue is saved to disk.
type savedMsg struct {
	err error
}

// reloadMsg signals the board should reload issues from disk.
type reloadMsg struct{}

// FilterFunc is a predicate for filtering issues in the board.
type FilterFunc func(*issue.Issue) bool

// Model is the main bubbletea model for the board.
type Model struct {
	columns    []*Column
	activeCol  int
	width      int
	height     int
	keys       KeyMap
	issuesDir  string
	cfg        *config.Config
	filters    []FilterFunc
	showDetail bool
	detailVP   viewport.Model
	err        error
	watcher    *fsnotify.Watcher
}

// NewModel creates a board model from the given issues.
func NewModel(issues []*issue.Issue, issuesDir string, cfg *config.Config, filters ...FilterFunc) Model {
	m := Model{
		columns:   make([]*Column, len(statusOrder)),
		keys:      DefaultKeyMap(),
		issuesDir: issuesDir,
		cfg:       cfg,
		filters:   filters,
	}

	m.populateColumns(issues)
	return m
}

// populateColumns clears columns and fills them from the issue list.
func (m *Model) populateColumns(issues []*issue.Issue) {
	// Preserve cursor positions per status
	cursors := make([]int, len(m.columns))
	for i, col := range m.columns {
		if col != nil {
			cursors[i] = col.Cursor
		}
	}

	m.columns = make([]*Column, len(statusOrder))
	for i, status := range statusOrder {
		m.columns[i] = NewColumn(status)
		for _, iss := range issues {
			if iss.Status == status {
				m.columns[i].Add(iss)
			}
		}
		// Restore cursor (clamped)
		if cursors[i] < len(m.columns[i].Issues) {
			m.columns[i].Cursor = cursors[i]
		}
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return m.watchFiles()
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.resize(msg), nil

	case savedMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		return m, nil

	case reloadMsg:
		return m.reload()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// resize stores the terminal size and re-fits an open detail viewport.
func (m Model) resize(msg tea.WindowSizeMsg) Model {
	m.width = msg.Width
	m.height = msg.Height
	if m.showDetail {
		if iss := m.columns[m.activeCol].Selected(); iss != nil {
			m.initDetailViewport(iss)
		}
	}
	return m
}

// reload re-reads the issues directory and restarts the file watcher.
func (m Model) reload() (tea.Model, tea.Cmd) {
	issues, err := issue.LoadAll(m.issuesDir)
	if err == nil {
		m.populateColumns(m.applyFilters(issues))
	}
	m.stopWatcher()
	return m, m.watchFiles()
}

// applyFilters keeps only the issues accepted by every configured predicate.
func (m Model) applyFilters(issues []*issue.Issue) []*issue.Issue {
	if len(m.filters) == 0 {
		return issues
	}

	filtered := make([]*issue.Issue, 0, len(issues))
	for _, iss := range issues {
		keep := true
		for _, filter := range m.filters {
			if !filter(iss) {
				keep = false
				break
			}
		}
		if keep {
			filtered = append(filtered, iss)
		}
	}
	return filtered
}

// handleKey dispatches a key press to the detail view or the board.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Clear the previous error once the user interacts.
	m.err = nil

	if m.showDetail {
		return m.handleDetailKey(msg)
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		m.stopWatcher()
		return m, tea.Quit

	case key.Matches(msg, m.keys.Left):
		if m.activeCol > 0 {
			m.activeCol--
		}

	case key.Matches(msg, m.keys.Right):
		if m.activeCol < len(m.columns)-1 {
			m.activeCol++
		}

	case key.Matches(msg, m.keys.Down):
		m.columns[m.activeCol].CursorDown()

	case key.Matches(msg, m.keys.Up):
		m.columns[m.activeCol].CursorUp()

	case key.Matches(msg, m.keys.MoveLeft):
		return m, m.moveIssue(-1)

	case key.Matches(msg, m.keys.MoveRight):
		return m, m.moveIssue(1)

	case key.Matches(msg, m.keys.Enter):
		if iss := m.columns[m.activeCol].Selected(); iss != nil {
			m.showDetail = true
			m.initDetailViewport(iss)
		}
	}

	return m, nil
}

// handleDetailKey handles keys while the detail view is open.
func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.showDetail = false
		return m, nil
	case key.Matches(msg, m.keys.Quit):
		m.stopWatcher()
		return m, tea.Quit
	default:
		var cmd tea.Cmd
		m.detailVP, cmd = m.detailVP.Update(msg)
		return m, cmd
	}
}

// moveIssue moves the selected issue to an adjacent column.
func (m *Model) moveIssue(direction int) tea.Cmd {
	targetCol := m.activeCol + direction
	if targetCol < 0 || targetCol >= len(m.columns) {
		return nil
	}

	col := m.columns[m.activeCol]
	iss := col.Selected()
	if iss == nil {
		return nil
	}

	// Remove from current column
	col.Remove(iss.ID)

	// Update status
	applyStatus(iss, statusOrder[targetCol])

	// Add to target column
	m.columns[targetCol].Add(iss)

	// Save async
	issuesDir := m.issuesDir
	autoStage := m.cfg.AutoStage
	filePath := iss.FilePath
	return func() tea.Msg {
		err := issue.Save(issuesDir, iss)
		if err == nil && autoStage {
			_ = git.Stage(filePath)
		}
		return savedMsg{err: err}
	}
}

// applyStatus sets the issue status and keeps the closed date in sync.
func applyStatus(iss *issue.Issue, status string) {
	iss.Status = status
	if status == "closed" || status == "wontfix" {
		iss.Closed = issue.Today()
	} else {
		iss.Closed = ""
	}
}
