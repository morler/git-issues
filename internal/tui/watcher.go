package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

// watchFiles starts an fsnotify watcher on the issues directory.
func (m *Model) watchFiles() tea.Cmd {
	if m.issuesDir == "" {
		return nil
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil
	}
	if err := w.Add(m.issuesDir); err != nil {
		_ = w.Close()
		return nil
	}
	m.watcher = w
	return waitForChange(w)
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
