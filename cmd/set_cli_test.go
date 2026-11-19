package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/steviee/git-issues/internal/issue"
)

func TestSetCLIPriority(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login", Priority: "low"})

	stdout, stderr, err := repo.run(t, "set", "1", "priority", "critical")
	if err != nil {
		t.Fatalf("set priority: %v (stderr: %s)", err, stderr)
	}
	wantContains(t, "stdout", stdout, "Updated #1: priority = critical")

	if got := repo.load(t, 1).Priority; got != "critical" {
		t.Errorf("priority on disk = %q, want critical", got)
	}
}

func TestSetCLIStatusStampsClosedDate(t *testing.T) {
	for _, status := range []string{"closed", "wontfix"} {
		t.Run(status, func(t *testing.T) {
			repo := newCLIRepo(t)
			repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login", Status: "open"})

			stdout, stderr, err := repo.run(t, "set", "1", "status", status)
			if err != nil {
				t.Fatalf("set status %s: %v (stderr: %s)", status, err, stderr)
			}
			wantContains(t, "stdout", stdout, "Updated #1: status = "+status)

			got := repo.load(t, 1)
			if got.Status != status {
				t.Errorf("status on disk = %q, want %q", got.Status, status)
			}
			if got.Closed != issue.Today() {
				t.Errorf("closed on disk = %q, want %q", got.Closed, issue.Today())
			}
		})
	}
}

func TestSetCLIStatusClearsClosedDate(t *testing.T) {
	for _, status := range []string{"open", "in-progress"} {
		t.Run(status, func(t *testing.T) {
			repo := newCLIRepo(t)
			repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login", Status: "wontfix", Closed: "2026-01-02"})

			_, stderr, err := repo.run(t, "set", "1", "status", status)
			if err != nil {
				t.Fatalf("set status %s: %v (stderr: %s)", status, err, stderr)
			}

			got := repo.load(t, 1)
			if got.Status != status {
				t.Errorf("status on disk = %q, want %q", got.Status, status)
			}
			if got.Closed != "" {
				t.Errorf("closed on disk = %q, want empty", got.Closed)
			}
		})
	}
}

func TestSetCLITitle(t *testing.T) {
	repo := newCLIRepo(t)
	seeded := repo.seed(t, &issue.Issue{ID: 1, Title: "Old title"})
	oldPath := seeded.FilePath

	stdout, stderr, err := repo.run(t, "set", "1", "title", "New login flow")
	if err != nil {
		t.Fatalf("set title: %v (stderr: %s)", err, stderr)
	}
	wantContains(t, "stdout", stdout, "Updated #1: title = New login flow")

	got := repo.load(t, 1)
	if got.Title != "New login flow" {
		t.Errorf("title on disk = %q, want %q", got.Title, "New login flow")
	}
	if got.FilePath != oldPath {
		t.Errorf("file path changed from %q to %q", oldPath, got.FilePath)
	}
}

func TestSetCLILabelAdd(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login", Labels: []string{"bug"}})

	stdout, stderr, err := repo.run(t, "set", "1", "label", "+auth")
	if err != nil {
		t.Fatalf("add label: %v (stderr: %s)", err, stderr)
	}
	wantContains(t, "stdout", stdout, "Updated #1: label = +auth")
	if got, want := repo.load(t, 1).Labels, []string{"bug", "auth"}; !slices.Equal(got, want) {
		t.Errorf("labels on disk = %v, want %v", got, want)
	}

	if _, _, err := repo.run(t, "set", "1", "label", "+bug"); err != nil {
		t.Fatalf("re-add existing label: %v", err)
	}
	if got, want := repo.load(t, 1).Labels, []string{"bug", "auth"}; !slices.Equal(got, want) {
		t.Errorf("labels after re-add = %v, want %v (no duplicate)", got, want)
	}
}

func TestSetCLILabelRemove(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login", Labels: []string{"bug", "auth"}})

	// A leading dash needs the "--" terminator, otherwise cobra reads it as a flag.
	stdout, stderr, err := repo.run(t, "set", "1", "label", "--", "-bug")
	if err != nil {
		t.Fatalf("remove label: %v (stderr: %s)", err, stderr)
	}
	wantContains(t, "stdout", stdout, "Updated #1: label = -bug")
	if got, want := repo.load(t, 1).Labels, []string{"auth"}; !slices.Equal(got, want) {
		t.Errorf("labels on disk = %v, want %v", got, want)
	}

	if _, _, err := repo.run(t, "set", "1", "label", "--", "-auth"); err != nil {
		t.Fatalf("remove last label: %v", err)
	}
	if got := repo.load(t, 1).Labels; len(got) != 0 {
		t.Errorf("labels after removing last = %v, want empty", got)
	}

	if _, _, err := repo.run(t, "set", "1", "label", "--", "-missing"); err != nil {
		t.Fatalf("remove unknown label: %v", err)
	}
	if got := repo.load(t, 1).Labels; len(got) != 0 {
		t.Errorf("labels after removing unknown = %v, want empty", got)
	}
}

func TestSetCLILabelReplace(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login", Labels: []string{"bug", "auth"}})

	stdout, stderr, err := repo.run(t, "set", "1", "label", "security, docs")
	if err != nil {
		t.Fatalf("replace labels: %v (stderr: %s)", err, stderr)
	}
	wantContains(t, "stdout", stdout, "Updated #1: label = security, docs")
	if got, want := repo.load(t, 1).Labels, []string{"security", "docs"}; !slices.Equal(got, want) {
		t.Errorf("labels on disk = %v, want %v", got, want)
	}
}

func TestSetCLIInvalidPriority(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login", Priority: "medium"})

	_, _, err := repo.run(t, "set", "1", "priority", "urgent")
	if err == nil {
		t.Fatal("expected error for invalid priority")
	}
	wantContains(t, "error", err.Error(), `invalid priority "urgent"`)
	wantContains(t, "error", err.Error(), "low, medium, high, critical")

	if got := repo.load(t, 1).Priority; got != "medium" {
		t.Errorf("priority changed to %q despite error", got)
	}
}

func TestSetCLIInvalidStatus(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login", Status: "open"})

	_, _, err := repo.run(t, "set", "1", "status", "blocked")
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
	wantContains(t, "error", err.Error(), `invalid status "blocked"`)
	wantContains(t, "error", err.Error(), "open, in-progress, closed, wontfix")

	if got := repo.load(t, 1).Status; got != "open" {
		t.Errorf("status changed to %q despite error", got)
	}
}

func TestSetCLIEmptyTitle(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login"})

	_, _, err := repo.run(t, "set", "1", "title", "   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only title")
	}
	wantContains(t, "error", err.Error(), "title must not be empty")

	if got := repo.load(t, 1).Title; got != "Fix login" {
		t.Errorf("title changed to %q despite error", got)
	}
}

func TestSetCLIUnknownField(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login"})

	_, _, err := repo.run(t, "set", "1", "assignee", "alice")
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	wantContains(t, "error", err.Error(), `unknown field "assignee"`)
	wantContains(t, "error", err.Error(), "priority, status, title, label")
}

func TestSetCLIIssueNotFound(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login"})

	_, _, err := repo.run(t, "set", "99", "priority", "high")
	if err == nil {
		t.Fatal("expected error for missing issue")
	}
	wantContains(t, "error", err.Error(), "issue #99 not found")
}

func TestSetCLIInvalidID(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login"})

	_, _, err := repo.run(t, "set", "abc", "priority", "high")
	if err == nil {
		t.Fatal("expected error for invalid ID")
	}
	wantContains(t, "error", err.Error(), "invalid issue ID: abc")
}

func TestSetCLISaveError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	repo := newCLIRepo(t)
	seeded := repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login", Priority: "low"})
	if err := os.Chmod(seeded.FilePath, 0o444); err != nil {
		t.Fatalf("chmod issue file: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(seeded.FilePath, 0o644) })

	_, _, err := repo.run(t, "set", "1", "priority", "high")
	if err == nil {
		t.Fatal("expected error when the issue file is not writable")
	}
	wantContains(t, "error", err.Error(), "permission denied")

	if got := repo.load(t, 1).Priority; got != "low" {
		t.Errorf("priority on disk = %q, want low", got)
	}
}

func TestSetCLINoIssuesRepo(t *testing.T) {
	t.Chdir(t.TempDir())

	_, _, err := runCLI(t, rootCmd, "set", "1", "priority", "high")
	if err == nil {
		t.Fatal("expected error outside an issues repository")
	}
	wantContains(t, "error", err.Error(), "no .issues/ directory found")
}

func TestSetCLIConfigLoadError(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login"})
	broken := filepath.Join(repo.issuesDir, ".config.yml")
	if err := os.WriteFile(broken, []byte("auto_stage: [broken\n"), 0o644); err != nil {
		t.Fatalf("write broken config: %v", err)
	}

	_, _, err := repo.run(t, "set", "1", "priority", "high")
	if err == nil {
		t.Fatal("expected error for unparsable config")
	}
	wantContains(t, "error", err.Error(), "yaml")

	if got := repo.load(t, 1).Priority; got != "medium" {
		t.Errorf("priority changed to %q despite config error", got)
	}
}

func TestSetCLIAutoStageOutsideGit(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login", Priority: "low"})
	repo.enableAutoStage(t)

	// The temp dir is not a git repository, so staging warns but must not fail.
	stdout, stderr, err := repo.run(t, "set", "1", "priority", "high")
	if err != nil {
		t.Fatalf("set with auto_stage: %v (stderr: %s)", err, stderr)
	}
	wantContains(t, "stdout", stdout, "Updated #1: priority = high")
	wantContains(t, "stderr", stderr, "could not stage file")

	if got := repo.load(t, 1).Priority; got != "high" {
		t.Errorf("priority on disk = %q, want high", got)
	}
}
