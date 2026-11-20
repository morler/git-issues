package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

// watcherMsg carries a watcher created in Init back into the model, which Init
// cannot store itself because it runs on a value copy.
type watcherMsg struct {
	watcher *fsnotify.Watcher
	wait    tea.Cmd
}

// openWatcher starts watching issuesDir and returns the watcher plus the
// command that waits for the next change.
func openWatcher(issuesDir string) (*fsnotify.Watcher, tea.Cmd) {
	if issuesDir == "" {
		return nil, nil
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil
	}
	if err := w.Add(issuesDir); err != nil {
		_ = w.Close()
		return nil, nil
	}
	return w, waitForChange(w)
}

// watchFiles starts an fsnotify watcher on the issues directory and stores it
// on the model. Callers must keep the returned model so the field is retained.
func (m *Model) watchFiles() tea.Cmd {
	w, cmd := openWatcher(m.issuesDir)
	m.watcher = w
	return cmd
}

// waitForChange waits for a relevant filesystem event and debounces the burst
// into a single reloadMsg.
func waitForChange(w *fsnotify.Watcher) tea.Cmd {
	return func() tea.Msg {
		for {
			select {
			case ev, ok := <-w.Events:
				if !ok {
					return nil
				}
				if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
					time.Sleep(watchDebounce)
					// Drain any queued events.
					for {
						select {
						case _, ok := <-w.Events:
							if !ok {
								return reloadMsg{}
							}
						default:
							return reloadMsg{}
						}
					}
				}
			case _, ok := <-w.Errors:
				if !ok {
					return nil
				}
			}
		}
	}
}

// stopWatcher closes the file watcher, if any.
func (m *Model) stopWatcher() {
	if m.watcher != nil {
		_ = m.watcher.Close()
		m.watcher = nil
	}
}
