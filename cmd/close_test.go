package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/steviee/git-issues/internal/issue"
)

func saveTestIssue(t *testing.T, dir string, id int, status string, dependsOn []int) *issue.Issue {
	t.Helper()
	iss := &issue.Issue{
		ID:       id,
		Title:    "Test issue",
		Status:   status,
		Priority: "medium",
		Labels:   []string{},
		Relations: issue.Relations{
			DependsOn: dependsOn,
		},
		Created: "2026-01-01",
	}
	if err := issue.Save(dir, iss); err != nil {
		t.Fatalf("failed to save test issue: %v", err)
	}
	return iss
}

func TestOpenBlockers(t *testing.T) {
	dir := t.TempDir()
	saveTestIssue(t, dir, 1, "open", nil)
	saveTestIssue(t, dir, 2, "in-progress", nil)
	saveTestIssue(t, dir, 3, "closed", nil)
	saveTestIssue(t, dir, 4, "wontfix", nil)

	iss := saveTestIssue(t, dir, 5, "open", []int{1, 2, 3, 4, 99})

	blockers := openBlockers(dir, iss)
	if len(blockers) != 2 {
		t.Fatalf("expected 2 open blockers, got %d: %v", len(blockers), blockers)
	}
	if !strings.Contains(blockers[0], "#1") || !strings.Contains(blockers[1], "#2") {
		t.Errorf("unexpected blockers: %v", blockers)
	}
}

func TestCloseWithBlockerNonInteractive(t *testing.T) {
	root := t.TempDir()
	issuesDir := filepath.Join(root, ".issues")
	if err := os.MkdirAll(issuesDir, 0755); err != nil {
		t.Fatalf("mkdir .issues: %v", err)
	}
	saveTestIssue(t, issuesDir, 1, "open", nil)
	saveTestIssue(t, issuesDir, 2, "open", []int{1})

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Errorf("chdir back: %v", err)
		}
	}()

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer func() { _ = devNull.Close() }()
	oldStdin := os.Stdin
	os.Stdin = devNull
	defer func() { os.Stdin = oldStdin }()

	if err := closeCmd.Flags().Set("force", "false"); err != nil {
		t.Fatal(err)
	}
	if err := closeCmd.Flags().Set("wontfix", "false"); err != nil {
		t.Fatal(err)
	}
	rootCmd.SetArgs([]string{"close", "2"})

	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected close to fail with open blockers in non-interactive mode")
	}

	loaded, err := issue.LoadByID(issuesDir, 2)
	if err != nil {
		t.Fatalf("reload issue: %v", err)
	}
	if loaded.Status != "open" {
		t.Errorf("issue #2 status = %q, want open", loaded.Status)
	}
}
