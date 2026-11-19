package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/steviee/git-issues/internal/issue"
)

func TestReopenCmdRestoresOpen(t *testing.T) {
	tests := []struct {
		name     string
		closeArg []string
		want     string
	}{
		{"closed", nil, "closed"},
		{"wontfix", []string{"--wontfix"}, "wontfix"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newCLIRepo(t)
			iss := r.seed(t, &issue.Issue{
				ID:       7,
				Title:    "Reopen me",
				Priority: "high",
				Labels:   []string{"bug"},
				Body:     "Context that must survive.",
			})

			closeArgs := append([]string{"close", "7"}, tt.closeArg...)
			if _, stderr, err := r.run(t, closeArgs...); err != nil {
				t.Fatalf("close failed: %v (stderr: %q)", err, stderr)
			}
			closed := r.load(t, 7)
			if closed.Status != tt.want {
				t.Fatalf("status after close = %q, want %q", closed.Status, tt.want)
			}
			if closed.Closed != issue.Today() {
				t.Fatalf("closed date after close = %q, want %q", closed.Closed, issue.Today())
			}

			stdout, stderr, err := r.run(t, "reopen", "7")
			if err != nil {
				t.Fatalf("reopen failed: %v (stderr: %q)", err, stderr)
			}
			if want := "Reopened: #7\n"; stdout != want {
				t.Errorf("stdout = %q, want %q", stdout, want)
			}

			got := r.load(t, 7)
			if got.Status != "open" {
				t.Errorf("status = %q, want open", got.Status)
			}
			if got.Closed != "" {
				t.Errorf("closed = %q, want empty", got.Closed)
			}
			if got.Updated != issue.Today() {
				t.Errorf("updated = %q, want %q", got.Updated, issue.Today())
			}
			if got.Priority != iss.Priority || got.Title != iss.Title {
				t.Errorf("reopen changed fields: title=%q priority=%q", got.Title, got.Priority)
			}
			if len(got.Labels) != 1 || got.Labels[0] != "bug" {
				t.Errorf("labels = %v, want [bug]", got.Labels)
			}
			// The body must survive the close/reopen round-trip unchanged.
			if got.Body != iss.Body {
				t.Errorf("body = %q, want %q", got.Body, iss.Body)
			}
			wantNotContains(t, "file", readFile(t, got.FilePath), "closed:")
		})
	}
}

func TestReopenCmdAlreadyOpen(t *testing.T) {
	r := newCLIRepo(t)
	r.seed(t, &issue.Issue{ID: 3, Title: "Still open", Status: "open"})

	stdout, stderr, err := r.run(t, "reopen", "3")
	if err != nil {
		t.Fatalf("reopen of an open issue failed: %v (stderr: %q)", err, stderr)
	}
	if want := "Reopened: #3\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	if got := r.load(t, 3); got.Status != "open" {
		t.Errorf("status = %q, want open", got.Status)
	}
}

func TestReopenCmdErrors(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{"missing issue", "99", "issue #99 not found"},
		{"invalid id", "abc", "invalid issue ID: abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newCLIRepo(t)
			r.seed(t, &issue.Issue{ID: 1, Title: "Unrelated", Status: "closed", Closed: "2026-01-05"})

			stdout, _, err := r.run(t, "reopen", tt.id)
			if err == nil {
				t.Fatalf("reopen %q succeeded, want error containing %q", tt.id, tt.want)
			}
			wantContains(t, "error", err.Error(), tt.want)
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if got := r.load(t, 1); got.Status != "closed" || got.Closed != "2026-01-05" {
				t.Errorf("unrelated issue changed: status=%q closed=%q", got.Status, got.Closed)
			}
		})
	}
}

// reopenTestHasGitAncestor reports whether dir or a parent holds a .git entry.
func reopenTestHasGitAncestor(t *testing.T, dir string) bool {
	t.Helper()
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

func TestReopenCmdAutoStageOutsideRepo(t *testing.T) {
	r := newCLIRepo(t)
	if reopenTestHasGitAncestor(t, r.root) {
		t.Skipf("%s is inside a git work tree, staging would succeed", r.root)
	}
	r.enableAutoStage(t)
	r.seed(t, &issue.Issue{ID: 5, Title: "Staged reopen", Status: "closed", Closed: "2026-01-05"})

	stdout, stderr, err := r.run(t, "reopen", "5")
	if err != nil {
		t.Fatalf("reopen failed: %v (stderr: %q)", err, stderr)
	}
	if want := "Reopened: #5\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	// The temp dir is not a git repository, so staging fails with a warning
	// but must not abort the command.
	wantContains(t, "stderr", stderr, "Warning: could not stage file")
	if got := r.load(t, 5); got.Status != "open" {
		t.Errorf("status = %q, want open", got.Status)
	}
}

func TestReopenCmdBrokenConfig(t *testing.T) {
	r := newCLIRepo(t)
	r.seed(t, &issue.Issue{ID: 4, Title: "Closed issue", Status: "closed", Closed: "2026-01-05"})
	if err := os.WriteFile(filepath.Join(r.issuesDir, ".config.yml"), []byte("invalid: [\n"), 0o644); err != nil {
		t.Fatalf("write broken config: %v", err)
	}

	stdout, _, err := r.run(t, "reopen", "4")
	if err == nil {
		t.Fatalf("reopen succeeded with a broken config, stdout: %q", stdout)
	}
	wantContains(t, "error", err.Error(), "yaml:")
	if got := r.load(t, 4); got.Status != "closed" {
		t.Errorf("status = %q, want closed (unchanged)", got.Status)
	}
}

func TestReopenCmdSaveFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}

	r := newCLIRepo(t)
	iss := r.seed(t, &issue.Issue{ID: 6, Title: "Read-only issue", Status: "closed", Closed: "2026-01-05"})
	if err := os.Chmod(iss.FilePath, 0o444); err != nil {
		t.Fatalf("chmod issue file: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(iss.FilePath, 0o644); err != nil {
			t.Fatalf("restore issue file permissions: %v", err)
		}
	})

	stdout, _, err := r.run(t, "reopen", "6")
	if err == nil {
		t.Fatalf("reopen succeeded on a read-only file, stdout: %q", stdout)
	}
	wantContains(t, "error", err.Error(), "permission denied")
	wantNotContains(t, "stdout", stdout, "Reopened")
	if got := r.load(t, 6); got.Status != "closed" || got.Closed != "2026-01-05" {
		t.Errorf("issue changed on failed save: status=%q closed=%q", got.Status, got.Closed)
	}
}

func TestReopenCmdNoIssuesDir(t *testing.T) {
	t.Chdir(t.TempDir())
	if dir, err := issue.IssuesDir(); err == nil {
		t.Skipf("temp dir is inside an initialized repository: %s", dir)
	}

	stdout, _, err := runCLI(t, rootCmd, "reopen", "1")
	if err == nil {
		t.Fatalf("reopen succeeded without .issues/, stdout: %q", stdout)
	}
	wantContains(t, "error", err.Error(), "no .issues/ directory found (run 'issues init')")
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}
