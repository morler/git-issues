package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/steviee/git-issues/internal/issue"
)

// seedCheckGraphChain writes a small dependency graph:
//
//	#1 Root (open)        blocks #2, #3
//	#2 Mid (in-progress)  blocks #4
//	#3 Done (closed)      blocked by #1
//	#4 Leaf (open)        blocked by #2
//	#5 Lone (open)        isolated
//
// Both sides of every relation are written so the graph matches a checked db.
func seedCheckGraphChain(t *testing.T, repo *cliRepo) {
	t.Helper()
	repo.seed(t, &issue.Issue{
		ID: 1, Title: "Root", Status: "open", Priority: "high",
		Relations: issue.Relations{Blocks: []int{2, 3}},
	})
	repo.seed(t, &issue.Issue{
		ID: 2, Title: "Mid", Status: "in-progress", Priority: "medium",
		Relations: issue.Relations{Blocks: []int{4}, DependsOn: []int{1}},
	})
	repo.seed(t, &issue.Issue{
		ID: 3, Title: "Done", Status: "closed", Priority: "low", Closed: "2026-02-01",
		Relations: issue.Relations{DependsOn: []int{1}},
	})
	repo.seed(t, &issue.Issue{
		ID: 4, Title: "Leaf", Status: "open", Priority: "critical",
		Relations: issue.Relations{DependsOn: []int{2}},
	})
	repo.seed(t, &issue.Issue{ID: 5, Title: "Lone", Status: "open", Priority: "low"})
}

func TestGraphCLIEmpty(t *testing.T) {
	repo := newCLIRepo(t)

	stdout, stderr, err := repo.run(t, "graph")
	if err != nil {
		t.Fatalf("graph failed: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected no output, got:\n%s", stdout)
	}
	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
}

func TestGraphCLIRelationsAndIsolated(t *testing.T) {
	repo := newCLIRepo(t)
	seedCheckGraphChain(t, repo)

	stdout, stderr, err := repo.run(t, "graph")
	if err != nil {
		t.Fatalf("graph failed: %v", err)
	}
	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	wantContains(t, "stdout", stdout, "#1  Root [open, high]")
	wantContains(t, "stdout", stdout, "└── blocks:")
	wantContains(t, "stdout", stdout, "#2  Mid [in-progress, medium]")
	wantContains(t, "stdout", stdout, "#4  Leaf [open, critical]")
	wantContains(t, "stdout", stdout, "#3  Done [closed, low]")
	wantContains(t, "stdout", stdout, "#5  Lone [open, low]")
	wantContains(t, "stdout", stdout, "(no relations)")

	// #4 is a leaf below #2 and must be indented deeper than #2.
	if !strings.Contains(stdout, "    #2  Mid") {
		t.Errorf("#2 is not nested under #1:\n%s", stdout)
	}
	if !strings.Contains(stdout, "        #4  Leaf") {
		t.Errorf("#4 is not nested under #2:\n%s", stdout)
	}
}

func TestGraphCLIOpenOnly(t *testing.T) {
	repo := newCLIRepo(t)
	seedCheckGraphChain(t, repo)

	stdout, _, err := repo.run(t, "graph", "--open-only")
	if err != nil {
		t.Fatalf("graph failed: %v", err)
	}
	wantContains(t, "stdout", stdout, "#1  Root")
	wantContains(t, "stdout", stdout, "#2  Mid")
	wantContains(t, "stdout", stdout, "#4  Leaf")
	wantContains(t, "stdout", stdout, "#5  Lone")
	wantNotContains(t, "stdout", stdout, "#3")
	wantNotContains(t, "stdout", stdout, "Done")
}

func TestGraphCLIRootSubgraph(t *testing.T) {
	repo := newCLIRepo(t)
	seedCheckGraphChain(t, repo)

	stdout, _, err := repo.run(t, "graph", "--root", "2")
	if err != nil {
		t.Fatalf("graph failed: %v", err)
	}
	wantContains(t, "stdout", stdout, "#2  Mid")
	wantContains(t, "stdout", stdout, "#4  Leaf")
	for _, hidden := range []string{"#1", "#3", "#5"} {
		wantNotContains(t, "stdout", stdout, hidden)
	}
}

func TestGraphCLIRootIsolated(t *testing.T) {
	repo := newCLIRepo(t)
	seedCheckGraphChain(t, repo)

	stdout, _, err := repo.run(t, "graph", "--root", "5")
	if err != nil {
		t.Fatalf("graph failed: %v", err)
	}
	wantContains(t, "stdout", stdout, "#5  Lone [open, low]")
	wantContains(t, "stdout", stdout, "(no relations)")
	wantNotContains(t, "stdout", stdout, "blocks:")
}

func TestGraphCLIRootUnknown(t *testing.T) {
	repo := newCLIRepo(t)
	seedCheckGraphChain(t, repo)

	stdout, stderr, err := repo.run(t, "graph", "--root", "999")
	if err != nil {
		t.Fatalf("graph failed: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected no output for unknown root, got:\n%s", stdout)
	}
	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
}

func TestGraphCLIRootOpenOnlyHidesClosedRoot(t *testing.T) {
	repo := newCLIRepo(t)
	seedCheckGraphChain(t, repo)

	stdout, _, err := repo.run(t, "graph", "--open-only", "--root", "3")
	if err != nil {
		t.Fatalf("graph failed: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected closed root to be hidden, got:\n%s", stdout)
	}
}

func TestGraphCLIDiamondDAG(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Root", Relations: issue.Relations{Blocks: []int{2, 3}}})
	repo.seed(t, &issue.Issue{ID: 2, Title: "Left", Relations: issue.Relations{Blocks: []int{4}, DependsOn: []int{1}}})
	repo.seed(t, &issue.Issue{ID: 3, Title: "Right", Relations: issue.Relations{Blocks: []int{4}, DependsOn: []int{1}}})
	repo.seed(t, &issue.Issue{ID: 4, Title: "Leaf", Relations: issue.Relations{DependsOn: []int{2, 3}}})

	stdout, stderr, err := repo.run(t, "graph")
	if err != nil {
		t.Fatalf("graph failed: %v", err)
	}
	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	wantNotContains(t, "stdout", stdout, "cycle detected")
	wantContains(t, "stdout", stdout, "#1  Root")
	wantContains(t, "stdout", stdout, "#2  Left")
	wantContains(t, "stdout", stdout, "#3  Right")
	if got := strings.Count(stdout, "#4  Leaf"); got != 1 {
		t.Errorf("#4 printed %d times, want once:\n%s", got, stdout)
	}
	wantContains(t, "stdout", stdout, "#4  (already shown)")
}

func TestGraphCLICycleTruncated(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{
		ID: 1, Title: "Alpha", Relations: issue.Relations{Blocks: []int{2}, DependsOn: []int{2}},
	})
	repo.seed(t, &issue.Issue{
		ID: 2, Title: "Beta", Relations: issue.Relations{Blocks: []int{1}, DependsOn: []int{1}},
	})

	stdout, _, err := repo.run(t, "graph")
	if err != nil {
		t.Fatalf("graph failed: %v", err)
	}
	wantContains(t, "stdout", stdout, "#1  Alpha")
	wantContains(t, "stdout", stdout, "#2  Beta")
	wantContains(t, "stdout", stdout, "Warning: cycle detected at #1, truncating")
	if got := strings.Count(stdout, "#1  Alpha"); got != 1 {
		t.Errorf("#1 printed %d times, want exactly once:\n%s", got, stdout)
	}

	stdout, _, err = repo.run(t, "graph", "--root", "2")
	if err != nil {
		t.Fatalf("graph --root failed: %v", err)
	}
	wantContains(t, "stdout", stdout, "Warning: cycle detected at #2, truncating")
}

func TestGraphCLINoRepo(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	stdout, _, err := runCLI(t, rootCmd, "graph")
	if err == nil {
		t.Fatal("expected graph to fail without .issues/")
	}
	wantContains(t, "error", err.Error(), "no .issues/ directory found")
	if stdout != "" {
		t.Errorf("unexpected stdout: %s", stdout)
	}
}

func TestGraphCLIBrokenFileSkipped(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Alpha"})
	if err := os.WriteFile(filepath.Join(repo.issuesDir, "0002-garbage.md"), []byte("garbage\n"), 0o644); err != nil {
		t.Fatalf("write garbage file: %v", err)
	}

	stdout, errOut, err := repo.run(t, "graph")
	if err != nil {
		t.Fatalf("graph failed: %v", err)
	}
	wantContains(t, "stdout", stdout, "#1  Alpha [open, medium]")
	wantNotContains(t, "stdout", stdout, "#2")
	wantContains(t, "stderr", errOut, "Warning: skipping issue file 0002-garbage.md")
}

func TestGraphCLIUnreadableIssuesDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Alpha"})
	if err := os.Chmod(repo.issuesDir, 0o000); err != nil {
		t.Fatalf("chmod .issues: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(repo.issuesDir, 0o755) })

	stdout, _, err := repo.run(t, "graph")
	if err == nil {
		t.Fatal("expected an error for an unreadable .issues/ directory")
	}
	if stdout != "" {
		t.Errorf("unexpected stdout: %s", stdout)
	}
}
