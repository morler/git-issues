package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/steviee/git-issues/internal/issue"
)

// workflowTestFreezeFile marks path read-only so the next Save fails. Root can
// always write, so the caller skips in that case.
func workflowTestFreezeFile(t *testing.T, path string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
}

// workflowTestBrokenConfig writes an unparsable .config.yml into the repo.
func workflowTestBrokenConfig(t *testing.T, issuesDir string) {
	t.Helper()
	path := filepath.Join(issuesDir, ".config.yml")
	if err := os.WriteFile(path, []byte("auto_stage: [broken\n"), 0o644); err != nil {
		t.Fatalf("write broken config: %v", err)
	}
}

func TestClaimCLISetsInProgress(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 3, Title: "Wire up auth", Status: "wontfix", Closed: "2026-01-02"})

	stdout, stderr, err := repo.run(t, "claim", "3")
	if err != nil {
		t.Fatalf("claim: %v (stderr: %s)", err, stderr)
	}
	wantContains(t, "stdout", stdout, "Claimed: #3 Wire up auth")

	got := repo.load(t, 3)
	if got.Status != "in-progress" {
		t.Errorf("status on disk = %q, want in-progress", got.Status)
	}
	if got.Closed != "" {
		t.Errorf("closed on disk = %q, want empty", got.Closed)
	}
}

func TestClaimCLIIssueNotFound(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Only issue"})

	_, _, err := repo.run(t, "claim", "42")
	if err == nil {
		t.Fatal("expected error for missing issue")
	}
	wantContains(t, "error", err.Error(), "issue #42 not found")
}

func TestClaimCLIInvalidID(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Only issue"})

	_, _, err := repo.run(t, "claim", "x1")
	if err == nil {
		t.Fatal("expected error for invalid ID")
	}
	wantContains(t, "error", err.Error(), "invalid issue ID: x1")
}

func TestDoneCLIMarksClosed(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 2, Title: "Write tests", Status: "in-progress"})

	stdout, stderr, err := repo.run(t, "done", "2")
	if err != nil {
		t.Fatalf("done: %v (stderr: %s)", err, stderr)
	}
	wantContains(t, "stdout", stdout, "Done: #2 Write tests")

	got := repo.load(t, 2)
	if got.Status != "closed" {
		t.Errorf("status on disk = %q, want closed", got.Status)
	}
	if got.Closed != issue.Today() {
		t.Errorf("closed on disk = %q, want %q", got.Closed, issue.Today())
	}
}

func TestDoneCLIIssueNotFound(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Only issue"})

	_, _, err := repo.run(t, "done", "42")
	if err == nil {
		t.Fatal("expected error for missing issue")
	}
	wantContains(t, "error", err.Error(), "issue #42 not found")
}

// TestClaimDoneCLIErrorPaths covers the error branches that claim and done
// share, since both call the same initialization sequence.
func TestClaimDoneCLIErrorPaths(t *testing.T) {
	for _, command := range []string{"claim", "done"} {
		t.Run(command+"/no issues repo", func(t *testing.T) {
			t.Chdir(t.TempDir())

			_, _, err := runCLI(t, rootCmd, command, "1")
			if err == nil {
				t.Fatal("expected error outside an issues repository")
			}
			wantContains(t, "error", err.Error(), "no .issues/ directory found")
		})

		t.Run(command+"/invalid id", func(t *testing.T) {
			repo := newCLIRepo(t)
			repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login"})

			_, _, err := repo.run(t, command, "1x")
			if err == nil {
				t.Fatal("expected error for invalid ID")
			}
			wantContains(t, "error", err.Error(), "invalid issue ID: 1x")
		})

		t.Run(command+"/broken config", func(t *testing.T) {
			repo := newCLIRepo(t)
			repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login"})
			workflowTestBrokenConfig(t, repo.issuesDir)

			_, _, err := repo.run(t, command, "1")
			if err == nil {
				t.Fatal("expected error for unparsable config")
			}
			wantContains(t, "error", err.Error(), "yaml")

			if got := repo.load(t, 1).Status; got != "open" {
				t.Errorf("status changed to %q despite config error", got)
			}
		})

		t.Run(command+"/save error", func(t *testing.T) {
			repo := newCLIRepo(t)
			seeded := repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login"})
			workflowTestFreezeFile(t, seeded.FilePath)

			_, _, err := repo.run(t, command, "1")
			if err == nil {
				t.Fatal("expected error when the issue file is not writable")
			}
			wantContains(t, "error", err.Error(), "permission denied")
		})

		t.Run(command+"/auto stage outside git", func(t *testing.T) {
			repo := newCLIRepo(t)
			repo.seed(t, &issue.Issue{ID: 1, Title: "Fix login"})
			repo.enableAutoStage(t)

			stdout, stderr, err := repo.run(t, command, "1")
			if err != nil {
				t.Fatalf("%s with auto_stage: %v (stderr: %s)", command, err, stderr)
			}
			wantContains(t, "stdout", stdout, "#1 Fix login")
			wantContains(t, "stderr", stderr, "could not stage file")
		})
	}
}

func TestBlockedCLIWithoutBlockedIssues(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Independent"})
	repo.seed(t, &issue.Issue{ID: 2, Title: "Also independent", Status: "in-progress"})

	stdout, stderr, err := repo.run(t, "blocked")
	if err != nil {
		t.Fatalf("blocked: %v (stderr: %s)", err, stderr)
	}
	wantContains(t, "stdout", stdout, "No blocked issues.")
	wantNotContains(t, "stdout", stdout, "#1")
	wantNotContains(t, "stdout", stdout, "#2")
}

func TestBlockedCLIListsBlockedIssues(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "First blocker", Status: "open"})
	repo.seed(t, &issue.Issue{ID: 2, Title: "Second blocker", Status: "in-progress"})
	repo.seed(t, &issue.Issue{ID: 3, Title: "Finished blocker", Status: "closed"})
	repo.seed(t, &issue.Issue{ID: 4, Title: "Blocked", Status: "open", Priority: "high", Relations: issue.Relations{DependsOn: []int{1, 2}}})
	repo.seed(t, &issue.Issue{ID: 5, Title: "Started", Status: "in-progress", Priority: "low", Relations: issue.Relations{DependsOn: []int{3}}})
	repo.seed(t, &issue.Issue{ID: 6, Title: "Done already", Status: "closed", Relations: issue.Relations{DependsOn: []int{1}}})
	repo.seed(t, &issue.Issue{ID: 7, Title: "Missing dep", Status: "open", Relations: issue.Relations{DependsOn: []int{99}}})

	stdout, stderr, err := repo.run(t, "blocked")
	if err != nil {
		t.Fatalf("blocked: %v (stderr: %s)", err, stderr)
	}
	wantContains(t, "stdout", stdout, "#4 Blocked [high] ← blocked by #1, #2")
	// #5 only depends on a closed issue, #6 is closed itself, #7 points at a
	// missing ID: none of them is reported.
	for _, unwanted := range []string{"#5", "#6", "#7", "No blocked issues."} {
		wantNotContains(t, "stdout", stdout, unwanted)
	}
}

func TestBlockedCLIWithOpenBlocker(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Blocker", Status: "in-progress"})
	repo.seed(t, &issue.Issue{ID: 2, Title: "Blocked", Status: "open", Relations: issue.Relations{DependsOn: []int{1}}})

	stdout, stderr, err := repo.run(t, "blocked", "2")
	if err != nil {
		t.Fatalf("blocked 2: %v (stderr: %s)", err, stderr)
	}
	wantContains(t, "stdout", stdout, "#1 Blocker [in-progress] ← OPEN")
	wantNotContains(t, "stdout", stdout, "is not blocked")
}

func TestBlockedCLIWithoutBlockers(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Independent"})

	stdout, stderr, err := repo.run(t, "blocked", "1")
	if err != nil {
		t.Fatalf("blocked 1: %v (stderr: %s)", err, stderr)
	}
	wantContains(t, "stdout", stdout, "#1 has no blockers.")
}

func TestBlockedCLIWithResolvedBlockers(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Done blocker", Status: "closed"})
	repo.seed(t, &issue.Issue{ID: 2, Title: "Wontfix blocker", Status: "wontfix"})
	repo.seed(t, &issue.Issue{ID: 3, Title: "Blocked", Relations: issue.Relations{DependsOn: []int{1, 2}}})

	stdout, stderr, err := repo.run(t, "blocked", "3")
	if err != nil {
		t.Fatalf("blocked 3: %v (stderr: %s)", err, stderr)
	}
	wantContains(t, "stdout", stdout, "#1 Done blocker [closed]")
	wantContains(t, "stdout", stdout, "#2 Wontfix blocker [wontfix]")
	wantContains(t, "stdout", stdout, "#3 is not blocked (all blockers are resolved).")
	wantNotContains(t, "stdout", stdout, "← OPEN")
}

func TestBlockedCLIWithMissingReferencedBlocker(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Blocked", Relations: issue.Relations{DependsOn: []int{99}}})

	stdout, stderr, err := repo.run(t, "blocked", "1")
	if err != nil {
		t.Fatalf("blocked 1: %v (stderr: %s)", err, stderr)
	}
	wantContains(t, "stdout", stdout, "#99 (not found)")
	wantContains(t, "stdout", stdout, "#1 is not blocked (all blockers are resolved).")
}

func TestBlockedCLIIssueNotFound(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Only issue"})

	_, _, err := repo.run(t, "blocked", "99")
	if err == nil {
		t.Fatal("expected error for missing issue")
	}
	wantContains(t, "error", err.Error(), "issue #99 not found")
}

func TestBlockedCLIInvalidID(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Only issue"})

	_, _, err := repo.run(t, "blocked", "abc")
	if err == nil {
		t.Fatal("expected error for invalid ID")
	}
	wantContains(t, "error", err.Error(), "invalid issue ID: abc")
}

func TestBlockedCLINoIssuesRepo(t *testing.T) {
	t.Chdir(t.TempDir())

	_, _, err := runCLI(t, rootCmd, "blocked")
	if err == nil {
		t.Fatal("expected error outside an issues repository")
	}
	wantContains(t, "error", err.Error(), "no .issues/ directory found")
}

func TestBlockedCLIUnreadableIssuesDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Independent"})
	if err := os.Chmod(repo.issuesDir, 0o000); err != nil {
		t.Fatalf("chmod .issues: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(repo.issuesDir, 0o755) })

	_, _, err := repo.run(t, "blocked")
	if err == nil {
		t.Fatal("expected error when .issues/ cannot be read")
	}
	wantContains(t, "error", err.Error(), "permission denied")
}
