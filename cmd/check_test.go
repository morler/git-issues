package cmd

import (
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

	if len(report.fixed) != 0 || len(report.dirty) != 0 {
		t.Errorf("runCheck without --fix fixed %v (dirty %v)", report.fixed, report.dirty)
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
	if !report.dirty[2] {
		t.Errorf("dirty = %v, want #2", report.dirty)
	}
	if len(report.errors) != 1 {
		t.Errorf("errors = %v, want the symmetry error to stay reported", report.errors)
	}
	if got := repo.load(t, 2).Relations.DependsOn; !slices.Equal(got, []int{1}) {
		t.Errorf("#2 depends-on = %v, want [1]", got)
	}
}
