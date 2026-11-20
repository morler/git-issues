package cmd

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/steviee/git-issues/internal/issue"
)

// checkCopyDoc duplicates id 1 without relations, so it only contributes the
// duplicate-ID finding.
const checkCopyDoc = `---
id: 1
title: Alpha copy
status: open
priority: medium
labels: []
created: 2026-01-01
updated: 2026-01-01
---
`

// checkBetaCopyDoc is a second file for id 2 that carries no relations of its
// own, so only its `updated` date would reveal a rewrite.
const checkBetaCopyDoc = `---
id: 2
title: Beta copy
status: open
priority: medium
labels: []
created: 2026-01-01
updated: 2020-02-02
---
`

func TestRunCheckCollectsFindings(t *testing.T) {
	repo := newCLIRepo(t)
	writeCheckIssueFile(t, repo.issuesDir, "0001-alpha.md", checkAlphaDoc)
	writeCheckIssueFile(t, repo.issuesDir, "0001-copy.md", checkCopyDoc)
	repo.seed(t, &issue.Issue{ID: 2, Title: "Beta", Status: "closed"})
	writeCheckIssueFile(t, repo.issuesDir, "0003-garbage.md", "not an issue file\n")

	report, err := runCheck(repo.issuesDir, false)
	if err != nil {
		t.Fatalf("runCheck: %v", err)
	}

	if len(report.errors) != 3 {
		t.Fatalf("errors = %v, want 3 findings", report.errors)
	}
	for _, want := range []string{
		"cannot parse ",
		"Duplicate ID #1 in files: ",
		"#1 blocks #2 but #2 does not have inverse depends-on #1",
	} {
		if !strings.Contains(strings.Join(report.errors, "\n"), want) {
			t.Errorf("errors missing %q: %v", want, report.errors)
		}
	}

	if len(report.warnings) != 1 {
		t.Fatalf("warnings = %v, want 1 finding", report.warnings)
	}
	if want := "#2: status is closed but closed date is empty"; report.warnings[0] != want {
		t.Errorf("warning = %q, want %q", report.warnings[0], want)
	}

	if len(report.fixed) != 0 {
		t.Errorf("runCheck without --fix fixed %v", report.fixed)
	}
}

func TestRunCheckFixSavesAndReports(t *testing.T) {
	repo := newCLIRepo(t)
	writeCheckIssueFile(t, repo.issuesDir, "0001-alpha.md", checkAlphaDoc)
	writeCheckIssueFile(t, repo.issuesDir, "0002-beta.md", checkBetaDoc)

	report, err := runCheck(repo.issuesDir, true)
	if err != nil {
		t.Fatalf("runCheck --fix: %v", err)
	}

	if len(report.fixed) != 1 {
		t.Fatalf("fixed = %v, want 1 finding", report.fixed)
	}
	if want := "#2: added missing depends-on #1"; report.fixed[0] != want {
		t.Errorf("fixed = %q, want %q", report.fixed[0], want)
	}
	if len(report.errors) != 1 {
		t.Errorf("errors = %v, want the symmetry error to stay reported", report.errors)
	}
	if got := repo.load(t, 2).Relations.DependsOn; !slices.Equal(got, []int{1}) {
		t.Errorf("#2 depends-on = %v, want [1]", got)
	}
}

func TestRunCheckFixReportsOnlySavedFixes(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}

	repo := newCLIRepo(t)
	writeCheckIssueFile(t, repo.issuesDir, "0001-alpha.md", checkAlphaDoc)
	betaPath := writeCheckIssueFile(t, repo.issuesDir, "0002-beta.md", checkBetaDoc)
	if err := os.Chmod(betaPath, 0o444); err != nil {
		t.Fatalf("chmod beta: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(betaPath, 0o644) })

	report, err := runCheck(repo.issuesDir, true)
	if err != nil {
		t.Fatalf("runCheck --fix: %v", err)
	}

	if len(report.fixed) != 0 {
		t.Errorf("fixed = %v, want nothing reported for an unwritable file", report.fixed)
	}
	if got := strings.Join(report.errors, "\n"); !strings.Contains(got, "#2: failed to save fix:") {
		t.Errorf("errors = %v, want a save failure for #2", report.errors)
	}
	if got := readFile(t, betaPath); strings.Contains(got, "depends-on") {
		t.Errorf("beta was modified despite the failed save:\n%s", got)
	}
}

func TestRunCheckFixLeavesUntouchedDuplicateIDFile(t *testing.T) {
	repo := newCLIRepo(t)
	writeCheckIssueFile(t, repo.issuesDir, "0001-alpha.md", checkAlphaDoc)
	writeCheckIssueFile(t, repo.issuesDir, "0002-beta-a.md", checkBetaDoc)
	copyPath := writeCheckIssueFile(t, repo.issuesDir, "0002-beta-b.md", checkBetaCopyDoc)
	before := readFile(t, copyPath)

	report, err := runCheck(repo.issuesDir, true)
	if err != nil {
		t.Fatalf("runCheck --fix: %v", err)
	}

	if len(report.fixed) != 1 {
		t.Fatalf("fixed = %v, want 1 finding", report.fixed)
	}
	if after := readFile(t, copyPath); after != before {
		t.Errorf("untouched duplicate-ID file was rewritten:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}
