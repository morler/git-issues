package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/steviee/git-issues/internal/issue"
)

// checkAlphaDoc blocks #2 but #2 has no inverse relation, and carries an old
// updated date so tests can detect that check --fix leaves it untouched.
const checkAlphaDoc = `---
id: 1
title: Alpha
status: open
priority: medium
labels: []
relations:
  blocks: [2]
created: 2026-01-01
updated: 2020-01-01
---
`

const checkBetaDoc = `---
id: 2
title: Beta
status: open
priority: medium
labels: []
created: 2026-01-01
updated: 2020-02-02
---
`

// checkBrokenAlphaDoc blocks #3, whose file cannot be parsed, so the relation
// must be reported but never auto-fixed.
const checkBrokenAlphaDoc = `---
id: 1
title: Alpha
status: open
priority: medium
labels: []
relations:
  blocks: [3]
created: 2026-01-01
updated: 2026-01-01
---
`

// writeCheckIssueFile writes a raw issue file, bypassing validation, so tests
// can create states the CLI itself never writes.
func writeCheckIssueFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestCheckCLIConsistent(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Alpha", Relations: issue.Relations{Blocks: []int{2}}})
	repo.seed(t, &issue.Issue{ID: 2, Title: "Beta", Relations: issue.Relations{DependsOn: []int{1}}})

	stdout, stderr, err := repo.run(t, "check")
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	wantContains(t, "stdout", stdout, "No problems found. Database is consistent.")
	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
}

func TestCheckCLIDuplicateID(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Alpha"})
	writeCheckIssueFile(t, repo.issuesDir, "0001-copy.md", `---
id: 1
title: Alpha copy
status: open
priority: medium
labels: []
created: 2026-01-01
updated: 2026-01-01
---
`)

	stdout, stderr, code := runCLISubprocess(t, repo.root, "check")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
	wantContains(t, "stderr", stderr, "Errors (1):")
	wantContains(t, "stderr", stderr, "Duplicate ID #1 in files: ")
	wantContains(t, "stderr", stderr, "0001-alpha.md")
	wantContains(t, "stderr", stderr, "0001-copy.md")
	wantNotContains(t, "stdout", stdout, "Database is consistent")
}

func TestCheckCLIOrphanFix(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Alpha", Relations: issue.Relations{Blocks: []int{99}}})

	stdout, stderr, code := runCLISubprocess(t, repo.root, "check")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
	wantContains(t, "stderr", stderr, "Errors (1):")
	wantContains(t, "stderr", stderr, "#1: blocks references non-existent #99")
	wantNotContains(t, "stdout", stdout, "Fixed")

	stdout, stderr, code = runCLISubprocess(t, repo.root, "check", "--fix")
	if code != 1 {
		t.Fatalf("exit code after fix = %d, want 1 (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
	wantContains(t, "stderr", stderr, "#1: blocks references non-existent #99")
	wantContains(t, "stdout", stdout, "Fixed (1):")
	wantContains(t, "stdout", stdout, "#1: removed orphan blocks reference to #99")
	if got := repo.load(t, 1).Relations.Blocks; len(got) != 0 {
		t.Errorf("orphan relation still on disk: %v", got)
	}

	stdout, stderr, code = runCLISubprocess(t, repo.root, "check")
	if code != 0 {
		t.Fatalf("third run exit code = %d, want 0 (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
	wantContains(t, "stdout", stdout, "No problems found. Database is consistent.")
	if stderr != "" {
		t.Errorf("unexpected stderr on third run: %s", stderr)
	}
}

func TestCheckCLIFixSymmetry(t *testing.T) {
	repo := newCLIRepo(t)
	alphaPath := writeCheckIssueFile(t, repo.issuesDir, "0001-alpha.md", checkAlphaDoc)
	betaPath := writeCheckIssueFile(t, repo.issuesDir, "0002-beta.md", checkBetaDoc)

	_, stderr, code := runCLISubprocess(t, repo.root, "check")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr: %s)", code, stderr)
	}
	wantContains(t, "stderr", stderr, "#1 blocks #2 but #2 does not have inverse depends-on #1")

	stdout, stderr, code := runCLISubprocess(t, repo.root, "check", "--fix")
	if code != 1 {
		t.Fatalf("exit code after fix = %d, want 1 (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
	wantContains(t, "stdout", stdout, "Fixed (1):")
	wantContains(t, "stdout", stdout, "#2: added missing depends-on #1")

	if got := readFile(t, alphaPath); got != checkAlphaDoc {
		t.Errorf("unmodified issue #1 was rewritten:\n%s", got)
	}
	beta := repo.load(t, 2)
	if len(beta.Relations.DependsOn) != 1 || beta.Relations.DependsOn[0] != 1 {
		t.Errorf("#2 depends-on = %v, want [1]", beta.Relations.DependsOn)
	}
	wantContains(t, "issue #2 file", readFile(t, betaPath), issue.Today())

	stdout, stderr, code = runCLISubprocess(t, repo.root, "check")
	if code != 0 {
		t.Fatalf("third run exit code = %d, want 0 (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
	wantContains(t, "stdout", stdout, "No problems found. Database is consistent.")
}

func TestCheckCLIValidationError(t *testing.T) {
	repo := newCLIRepo(t)
	writeCheckIssueFile(t, repo.issuesDir, "0001-alpha.md", `---
id: 1
title: Alpha
status: bogus
priority: medium
labels: []
created: 2026-01-01
updated: 2026-01-01
---
`)

	stdout, stderr, code := runCLISubprocess(t, repo.root, "check")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
	wantContains(t, "stderr", stderr, "Errors (1):")
	wantContains(t, "stderr", stderr, `#1: validation error: invalid status "bogus", must be one of: open, in-progress, closed, wontfix`)
	wantNotContains(t, "stdout", stdout, "Database is consistent")
}

func TestCheckCLIBrokenFile(t *testing.T) {
	repo := newCLIRepo(t)
	alphaPath := writeCheckIssueFile(t, repo.issuesDir, "0001-alpha.md", checkBrokenAlphaDoc)
	writeCheckIssueFile(t, repo.issuesDir, "0003-garbage.md", "not an issue file\n")

	stdout, stderr, code := runCLISubprocess(t, repo.root, "check")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
	wantContains(t, "stderr", stderr, "Errors (2):")
	wantContains(t, "stderr", stderr, "cannot parse ")
	wantContains(t, "stderr", stderr, "0003-garbage.md")
	wantContains(t, "stderr", stderr, "file must start with ---")
	wantContains(t, "stderr", stderr, "#1: blocks references broken issue #3")

	// An unparsable file cannot be fixed, so --fix must keep reporting it.
	stdout, stderr, code = runCLISubprocess(t, repo.root, "check", "--fix")
	if code != 1 {
		t.Fatalf("exit code with fix = %d, want 1 (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
	wantNotContains(t, "stdout", stdout, "Fixed")
	wantContains(t, "stderr", stderr, "#1: blocks references broken issue #3")
	if got := readFile(t, alphaPath); got != checkBrokenAlphaDoc {
		t.Errorf("#1 was rewritten although broken references are not auto-fixable:\n%s", got)
	}
}

func TestCheckCLIWarnings(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Alpha", Status: "closed"})
	repo.seed(t, &issue.Issue{ID: 2, Title: "Beta", Closed: "2026-02-02"})
	repo.seed(t, &issue.Issue{ID: 3, Title: "Gamma", Labels: []string{"bug", "bug"}})

	stdout, stderr, err := repo.run(t, "check")
	if err != nil {
		t.Fatalf("check failed on warnings only: %v", err)
	}
	if stdout != "" {
		t.Errorf("unexpected stdout: %s", stdout)
	}
	wantContains(t, "stderr", stderr, "Warnings (3):")
	wantContains(t, "stderr", stderr, "#1: status is closed but closed date is empty")
	wantContains(t, "stderr", stderr, "#2: status is open but has closed date 2026-02-02")
	wantContains(t, "stderr", stderr, `#3: duplicate label "bug"`)
	wantNotContains(t, "stderr", stderr, "Errors")
}

func TestCheckCLIFixConfigError(t *testing.T) {
	repo := newCLIRepo(t)
	writeCheckIssueFile(t, repo.issuesDir, "0001-alpha.md", checkAlphaDoc)
	writeCheckIssueFile(t, repo.issuesDir, "0002-beta.md", checkBetaDoc)
	writeCheckIssueFile(t, repo.issuesDir, ".config.yml", "auto_stage: [oops\n")

	if _, _, err := repo.run(t, "check", "--fix"); err == nil {
		t.Fatal("expected an error for an unreadable config")
	}
	if got := readFile(t, filepath.Join(repo.issuesDir, "0002-beta.md")); got != checkBetaDoc {
		t.Errorf("issue #2 rewritten despite config error:\n%s", got)
	}
}

func TestCheckCLISymmetryAllRelations(t *testing.T) {
	repo := newCLIRepo(t)
	writeCheckIssueFile(t, repo.issuesDir, "0001-alpha.md", `---
id: 1
title: Alpha
status: open
priority: medium
labels: []
relations:
  duplicates: [2]
  depends-on: [3]
  related-to: [4]
created: 2026-01-01
updated: 2020-01-01
---
`)
	repo.seed(t, &issue.Issue{ID: 2, Title: "Beta", Created: "2026-01-01"})
	repo.seed(t, &issue.Issue{ID: 3, Title: "Gamma", Created: "2026-01-01"})
	repo.seed(t, &issue.Issue{ID: 4, Title: "Delta", Created: "2026-01-01"})

	stdout, stderr, code := runCLISubprocess(t, repo.root, "check", "--fix")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
	wantContains(t, "stderr", stderr, "#1 duplicates #2 but #2 does not have inverse duplicates #1")
	wantContains(t, "stderr", stderr, "#1 depends-on #3 but #3 does not have inverse blocks #1")
	wantContains(t, "stderr", stderr, "#1 related-to #4 but #4 does not have inverse related-to #1")
	wantContains(t, "stdout", stdout, "Fixed (3):")
	wantContains(t, "stdout", stdout, "#2: added missing duplicates #1")
	wantContains(t, "stdout", stdout, "#3: added missing blocks #1")
	wantContains(t, "stdout", stdout, "#4: added missing related-to #1")

	if got := repo.load(t, 2).Relations.Duplicates; len(got) != 1 || got[0] != 1 {
		t.Errorf("#2 duplicates = %v, want [1]", got)
	}
	if got := repo.load(t, 3).Relations.Blocks; len(got) != 1 || got[0] != 1 {
		t.Errorf("#3 blocks = %v, want [1]", got)
	}
	if got := repo.load(t, 4).Relations.RelatedTo; len(got) != 1 || got[0] != 1 {
		t.Errorf("#4 related-to = %v, want [1]", got)
	}
	if _, stderr, code = runCLISubprocess(t, repo.root, "check"); code != 0 {
		t.Fatalf("follow-up check exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
}

func TestCheckCLIFixFailureKeepsIssue(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	repo := newCLIRepo(t)
	writeCheckIssueFile(t, repo.issuesDir, "0001-alpha.md", checkAlphaDoc)
	betaPath := writeCheckIssueFile(t, repo.issuesDir, "0002-beta.md", checkBetaDoc)
	if err := os.Chmod(betaPath, 0o444); err != nil {
		t.Fatalf("chmod beta: %v", err)
	}

	_, stderr, code := runCLISubprocess(t, repo.root, "check", "--fix")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr: %s)", code, stderr)
	}
	wantContains(t, "stderr", stderr, "#2: failed to save fix: ")
	if got := readFile(t, betaPath); got != checkBetaDoc {
		t.Errorf("read-only issue file changed:\n%s", got)
	}
}

func TestCheckCLIFixAutoStageWarnsOutsideGit(t *testing.T) {
	repo := newCLIRepo(t)
	repo.enableAutoStage(t)
	writeCheckIssueFile(t, repo.issuesDir, "0001-alpha.md", checkAlphaDoc)
	writeCheckIssueFile(t, repo.issuesDir, "0002-beta.md", checkBetaDoc)

	stdout, stderr, code := runCLISubprocess(t, repo.root, "check", "--fix")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
	wantContains(t, "stdout", stdout, "#2: added missing depends-on #1")
	wantContains(t, "stderr", stderr, "Warning: could not stage file (not a git repo?)")
}

func TestCheckCLINoRepo(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	stdout, _, err := runCLI(t, rootCmd, "check")
	if err == nil {
		t.Fatal("expected check to fail without .issues/")
	}
	wantContains(t, "error", err.Error(), "no .issues/ directory found")
	if stdout != "" {
		t.Errorf("unexpected stdout: %s", stdout)
	}

	// The root command turns the error into exit code 1 for the real CLI.
	_, _, code := runCLISubprocess(t, dir, "check")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestCheckCLIUnreadableIssuesDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Alpha"})
	if err := os.Chmod(repo.issuesDir, 0o000); err != nil {
		t.Fatalf("chmod .issues: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(repo.issuesDir, 0o755) })

	if _, _, err := repo.run(t, "check"); err == nil {
		t.Fatal("expected an error for an unreadable .issues/ directory")
	}
}

func TestCheckGetRelSlice(t *testing.T) {
	iss := &issue.Issue{Relations: issue.Relations{
		Blocks:     []int{1},
		DependsOn:  []int{2},
		RelatedTo:  []int{3},
		Duplicates: []int{4},
	}}

	tests := map[string]int{
		"blocks":     1,
		"depends-on": 2,
		"related-to": 3,
		"duplicates": 4,
	}
	for relation, want := range tests {
		t.Run(relation, func(t *testing.T) {
			got := getRelSlice(iss, relation)
			if len(got) != 1 || got[0] != want {
				t.Errorf("getRelSlice(%q) = %v, want [%d]", relation, got, want)
			}
		})
	}
	if got := getRelSlice(iss, "unknown"); got != nil {
		t.Errorf("getRelSlice(unknown) = %v, want nil", got)
	}
}
