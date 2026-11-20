package cmd

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/steviee/git-issues/internal/issue"
)

// showRun executes `issues show` with args and requires it to succeed.
func showRun(t *testing.T, repo *cliRepo, args ...string) string {
	t.Helper()
	stdout, stderr, err := repo.run(t, append([]string{"show"}, args...)...)
	if err != nil {
		t.Fatalf("issues show %v: %v (stderr: %s)", args, err, stderr)
	}
	return stdout
}

func TestShowCommandText(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{
		ID: 7, Title: "Login fails with empty password", Status: "open", Priority: "high",
		Labels:  []string{"bug", "auth"},
		Created: "2026-01-15",
		Body:    "Reproduced on staging.\n\n## Notes\nNeeds a regression test.",
	})

	stdout := showRun(t, repo, "7")

	wantContains(t, "header", stdout, "Issue #7  ·  high  ·  open")
	wantContains(t, "title", stdout, "Login fails with empty password")
	wantContains(t, "labels", stdout, "Labels:   bug, auth")
	wantContains(t, "created", stdout, "Created:  2026-01-15")
	wantContains(t, "updated", stdout, "Updated:  "+issue.Today())
	wantContains(t, "body", stdout, "Reproduced on staging.")
	wantContains(t, "body", stdout, "Needs a regression test.")
	wantContains(t, "rule", stdout, strings.Repeat("━", 54))

	// Without relations the section header is omitted entirely.
	wantNotContains(t, "relations", stdout, "Relations")
	wantNotContains(t, "closed", stdout, "Closed:")
}

func TestShowCommandLabelsOmittedWhenEmpty(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "No labels"})

	stdout := showRun(t, repo, "1")
	wantNotContains(t, "labels line", stdout, "Labels:")
}

func TestShowCommandBlockers(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Open blocker", Status: "open"})
	repo.seed(t, &issue.Issue{ID: 2, Title: "Started blocker", Status: "in-progress"})
	repo.seed(t, &issue.Issue{ID: 3, Title: "Finished blocker", Status: "closed"})
	repo.seed(t, &issue.Issue{ID: 4, Title: "Wontfix blocker", Status: "wontfix"})
	repo.seed(t, &issue.Issue{ID: 5, Title: "Blocked issue", Status: "open", Relations: issue.Relations{
		DependsOn: []int{1, 2, 3, 4},
	}})

	stdout := showRun(t, repo, "5")

	wantContains(t, "section", stdout, "Depends on:")
	for _, want := range []string{"#1", "Open blocker", "#2", "Started blocker", "#3", "Finished blocker", "#4", "Wontfix blocker"} {
		wantContains(t, "relation entry", stdout, want)
	}

	lines := strings.Split(stdout, "\n")
	blocked := map[string]string{}
	for _, line := range lines {
		for _, id := range []string{"#1", "#2", "#3", "#4"} {
			if strings.Contains(line, id+" ") {
				blocked[id] = line
			}
		}
	}
	if !strings.Contains(blocked["#1"], "← BLOCKER") {
		t.Errorf("open dependency #1 missing BLOCKER marker: %q", blocked["#1"])
	}
	if !strings.Contains(blocked["#2"], "← BLOCKER") {
		t.Errorf("in-progress dependency #2 missing BLOCKER marker: %q", blocked["#2"])
	}
	if strings.Contains(blocked["#3"], "BLOCKER") {
		t.Errorf("closed dependency #3 must not be a blocker: %q", blocked["#3"])
	}
	if strings.Contains(blocked["#4"], "BLOCKER") {
		t.Errorf("wontfix dependency #4 must not be a blocker: %q", blocked["#4"])
	}
	wantContains(t, "status", blocked["#1"], "[open]")
	wantContains(t, "status", blocked["#2"], "[in-progress]")
}

func TestShowCommandRelationSections(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Target open", Status: "open"})
	repo.seed(t, &issue.Issue{ID: 2, Title: "Target closed", Status: "closed"})
	repo.seed(t, &issue.Issue{
		ID: 3, Title: "Central issue", Status: "open",
		Relations: issue.Relations{
			Blocks:     []int{1},
			DependsOn:  []int{2},
			RelatedTo:  []int{4},
			Duplicates: []int{5},
		},
	})
	repo.seed(t, &issue.Issue{ID: 4, Title: "Loose relation", Status: "open"})
	repo.seed(t, &issue.Issue{ID: 5, Title: "Duplicate theme", Status: "open"})

	stdout := showRun(t, repo, "3")

	wantContains(t, "section", stdout, "Relations")
	wantContains(t, "section", stdout, "Blocks:")
	wantContains(t, "section", stdout, "Depends on:")
	wantContains(t, "section", stdout, "Related to:")
	wantContains(t, "section", stdout, "Duplicates:")
	wantContains(t, "entry", stdout, "Loose relation")
	wantContains(t, "entry", stdout, "Duplicate theme")
	wantContains(t, "entry", stdout, "Target closed")

	// Only depends-on entries report blockers.
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Target open") {
			if strings.Contains(line, "BLOCKER") {
				t.Errorf("blocks target must not be marked BLOCKER: %q", line)
			}
		}
		if strings.Contains(line, "Target closed") && strings.Contains(line, "BLOCKER") {
			t.Errorf("closed dependency must not be marked BLOCKER: %q", line)
		}
	}
}

func TestShowCommandMissingRelationTarget(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{
		ID: 1, Title: "Dangling relations", Status: "open",
		Relations: issue.Relations{Blocks: []int{42}, DependsOn: []int{99}},
	})

	stdout := showRun(t, repo, "1")

	wantContains(t, "missing target", stdout, "#42")
	wantContains(t, "missing target", stdout, "#99")
	if got := strings.Count(stdout, "(not found)"); got != 2 {
		t.Errorf("(not found) occurrences = %d, want 2:\n%s", got, stdout)
	}
	wantNotContains(t, "missing target", stdout, "BLOCKER")
}

func TestShowCommandTruncatesLongRelationTitle(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{
		ID: 1, Title: "A very long related issue title that is definitely longer than forty characters", Status: "open",
	})
	repo.seed(t, &issue.Issue{ID: 2, Title: "Short", Status: "open", Relations: issue.Relations{Blocks: []int{1}}})

	stdout := showRun(t, repo, "2")

	wantContains(t, "truncated title", stdout, "…")
	wantNotContains(t, "truncated title", stdout, "definitely longer than forty characters")
}

func TestShowCommandClosedDate(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{
		ID: 1, Title: "Finished work", Status: "closed", Priority: "low",
		Closed: "2026-02-03",
	})

	stdout := showRun(t, repo, "1")

	wantContains(t, "status", stdout, "Issue #1  ·  low  ·  closed")
	wantContains(t, "closed date", stdout, "Closed:   2026-02-03")
}

func TestShowCommandJSON(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{
		ID: 3, Title: "JSON issue", Status: "in-progress", Priority: "critical",
		Labels:    []string{"bug", "auth"},
		Relations: issue.Relations{Blocks: []int{1}, Duplicates: []int{2}},
		Created:   "2026-01-15",
		Body:      "body text that json output omits",
	})

	stdout := showRun(t, repo, "3", "--format", "json")
	if trimmed := strings.TrimSpace(stdout); !strings.HasPrefix(trimmed, "{") {
		t.Fatalf("json output is not an object: %s", stdout)
	}

	var got issue.Issue
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal show json: %v\n%s", err, stdout)
	}
	if got.ID != 3 || got.Title != "JSON issue" || got.Status != "in-progress" || got.Priority != "critical" {
		t.Errorf("json issue = %+v, want #3 JSON issue in-progress critical", got)
	}
	if !reflect.DeepEqual(got.Labels, []string{"bug", "auth"}) {
		t.Errorf("labels = %v, want [bug auth]", got.Labels)
	}
	if !reflect.DeepEqual(got.Relations.Blocks, []int{1}) || !reflect.DeepEqual(got.Relations.Duplicates, []int{2}) {
		t.Errorf("relations = %+v, want blocks [1] duplicates [2]", got.Relations)
	}
	if got.Created != "2026-01-15" || got.Updated != issue.Today() {
		t.Errorf("dates = created %q updated %q, want 2026-01-15 and %q", got.Created, got.Updated, issue.Today())
	}
	wantNotContains(t, "json output", stdout, "body text")
	wantNotContains(t, "json output", stdout, "Issue #3")
}

func TestShowCommandErrors(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Existing"})

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing issue", []string{"99"}, "issue #99 not found"},
		{"non-numeric id", []string{"abc"}, "invalid issue ID: abc"},
		{"mixed id", []string{"7x"}, "invalid issue ID: 7x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := repo.run(t, append([]string{"show"}, tt.args...)...)
			if err == nil {
				t.Fatalf("show %v: expected error", tt.args)
			}
			wantContains(t, "error", err.Error(), tt.want)
		})
	}
}

func TestShowCommandNoRepo(t *testing.T) {
	repo := newCLIRepo(t)
	if err := os.RemoveAll(repo.issuesDir); err != nil {
		t.Fatalf("remove .issues: %v", err)
	}

	_, _, err := repo.run(t, "show", "1")
	if err == nil {
		t.Fatal("expected show to fail without .issues/")
	}
	wantContains(t, "error", err.Error(), "no .issues/ directory found")
}

func TestShowCommandReportsLoadFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}

	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Existing"})
	if err := os.Chmod(repo.issuesDir, 0o111); err != nil {
		t.Fatalf("chmod .issues: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(repo.issuesDir, 0o755) })

	_, _, err := repo.run(t, "show", "1")
	if err == nil {
		t.Fatal("expected show to fail on an unreadable .issues/")
	}
	wantContains(t, "error", err.Error(), "permission denied")
	wantNotContains(t, "error", err.Error(), "not found")
}

func TestShowCommandRequiresArgument(t *testing.T) {
	repo := newCLIRepo(t)

	_, _, err := repo.run(t, "show")
	if err == nil {
		t.Fatal("expected show without arguments to fail")
	}
	wantContains(t, "error", err.Error(), "accepts 1 arg(s)")
}
