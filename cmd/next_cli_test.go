package cmd

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/steviee/git-issues/internal/issue"
)

// nextTestIssue builds a full issue for next command tests.
func nextTestIssue(id int, title, status, priority string, labels []string, dependsOn []int) *issue.Issue {
	if labels == nil {
		labels = []string{}
	}
	return &issue.Issue{
		ID:       id,
		Title:    title,
		Status:   status,
		Priority: priority,
		Labels:   labels,
		Relations: issue.Relations{
			DependsOn: dependsOn,
		},
		Created: "2026-01-01",
	}
}

// nextRun executes the next command and fails on any returned error.
func nextRun(t *testing.T, r *cliRepo, args ...string) string {
	t.Helper()
	stdout, stderr, err := r.run(t, append([]string{"next"}, args...)...)
	if err != nil {
		t.Fatalf("next %v: %v (stderr: %q)", args, err, stderr)
	}
	return stdout
}

func TestNextRequiresIssuesDir(t *testing.T) {
	t.Chdir(t.TempDir())

	_, _, err := runCLI(t, rootCmd, "next")
	if err == nil {
		t.Fatal("next outside an issues repository returned no error")
	}
	wantContains(t, "next error", err.Error(), "no .issues/ directory found")
}

func TestNextUnreadableIssuesDir(t *testing.T) {
	r := newCLIRepo(t)
	r.seed(t, nextTestIssue(1, "Hidden", "open", "high", nil, nil))
	if err := os.Chmod(r.issuesDir, 0o000); err != nil {
		t.Fatalf("chmod .issues: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(r.issuesDir, 0o755) })
	if _, err := os.ReadDir(r.issuesDir); err == nil {
		t.Skip("directory permissions are not enforced for this user")
	}

	_, _, err := r.run(t, "next")
	if err == nil {
		t.Fatal("next with an unreadable .issues/ returned no error")
	}
	wantContains(t, "next error", err.Error(), "permission denied")
}

func TestNextSelectsHighestPriorityUnblockedIssue(t *testing.T) {
	r := newCLIRepo(t)
	r.seed(t, nextTestIssue(1, "Low docs", "open", "low", []string{"docs"}, nil))
	r.seed(t, nextTestIssue(2, "High login bug", "open", "high", []string{"bug"}, nil))
	// The critical issue is blocked by the open low issue, so it is skipped.
	r.seed(t, nextTestIssue(3, "Critical blocked", "open", "critical", []string{"auth"}, []int{1}))

	stdout := nextRun(t, r)
	if got, want := strings.TrimSpace(stdout), "2 high High login bug [bug]"; got != want {
		t.Errorf("next output = %q, want %q", got, want)
	}
	wantNotContains(t, "next output", stdout, "#3")
	wantNotContains(t, "next output", stdout, "Critical blocked")
}

func TestNextSkipsBlockedIssues(t *testing.T) {
	r := newCLIRepo(t)
	r.seed(t, nextTestIssue(1, "Open blocker", "open", "low", nil, nil))
	r.seed(t, nextTestIssue(2, "Blocked by open", "open", "critical", nil, []int{1}))
	r.seed(t, nextTestIssue(3, "In progress blocker", "in-progress", "low", nil, nil))
	r.seed(t, nextTestIssue(4, "Blocked by in-progress", "open", "high", nil, []int{3}))
	// A dependency on a missing issue does not block.
	r.seed(t, nextTestIssue(5, "Dangling dependency", "open", "medium", nil, []int{99}))

	stdout := nextRun(t, r)
	if got, want := strings.TrimSpace(stdout), "5 medium Dangling dependency []"; got != want {
		t.Errorf("next output = %q, want %q", got, want)
	}
	for _, blocked := range []string{"2 critical", "4 high"} {
		wantNotContains(t, "next output", stdout, blocked)
	}
}

func TestNextSelectsIssueWithClosedOrWontfixBlocker(t *testing.T) {
	r := newCLIRepo(t)
	r.seed(t, nextTestIssue(1, "Closed blocker", "closed", "critical", nil, nil))
	r.seed(t, nextTestIssue(2, "Wontfix blocker", "wontfix", "high", nil, nil))
	r.seed(t, nextTestIssue(3, "Depends on closed", "open", "critical", []string{"bug"}, []int{1}))
	r.seed(t, nextTestIssue(4, "Depends on wontfix", "open", "high", nil, []int{2}))

	stdout := nextRun(t, r)
	if got, want := strings.TrimSpace(stdout), "3 critical Depends on closed [bug]"; got != want {
		t.Errorf("next output = %q, want %q", got, want)
	}
	wantNotContains(t, "next output", stdout, "Depends on wontfix")
}

func TestNextNoActionableIssues(t *testing.T) {
	t.Run("empty repository", func(t *testing.T) {
		r := newCLIRepo(t)
		stdout := nextRun(t, r)
		if got, want := strings.TrimSpace(stdout), "No actionable issues found."; got != want {
			t.Errorf("next output = %q, want %q", got, want)
		}
	})

	t.Run("only closed or blocked issues", func(t *testing.T) {
		r := newCLIRepo(t)
		// Every open issue is blocked: #1 and #2 block each other, the rest is done.
		r.seed(t, nextTestIssue(1, "Circular one", "open", "high", nil, []int{2}))
		r.seed(t, nextTestIssue(2, "Circular two", "open", "critical", nil, []int{1}))
		r.seed(t, nextTestIssue(3, "Closed", "closed", "critical", nil, nil))
		r.seed(t, nextTestIssue(4, "Wontfix", "wontfix", "high", nil, nil))

		stdout, stderr, err := r.run(t, "next")
		if err != nil {
			t.Fatalf("next: %v (stderr: %q)", err, stderr)
		}
		if got, want := strings.TrimSpace(stdout), "No actionable issues found."; got != want {
			t.Errorf("next output = %q, want %q", got, want)
		}
	})
}

func TestNextPriorityFloorFilter(t *testing.T) {
	t.Run("issue above floor is selected", func(t *testing.T) {
		r := newCLIRepo(t)
		r.seed(t, nextTestIssue(1, "Critical", "open", "critical", []string{"bug"}, nil))
		r.seed(t, nextTestIssue(2, "High", "open", "high", nil, nil))

		stdout := nextRun(t, r, "--priority", "high")
		if got, want := strings.TrimSpace(stdout), "1 critical Critical [bug]"; got != want {
			t.Errorf("next --priority high output = %q, want %q", got, want)
		}
	})

	t.Run("issues below floor are excluded", func(t *testing.T) {
		r := newCLIRepo(t)
		r.seed(t, nextTestIssue(1, "Medium", "open", "medium", nil, nil))
		r.seed(t, nextTestIssue(2, "Low", "open", "low", nil, nil))

		stdout := nextRun(t, r, "--priority", "high")
		if got, want := strings.TrimSpace(stdout), "No actionable issues found."; got != want {
			t.Errorf("next --priority high output = %q, want %q", got, want)
		}

		stdout = nextRun(t, r, "--priority", "medium")
		if got, want := strings.TrimSpace(stdout), "1 medium Medium []"; got != want {
			t.Errorf("next --priority medium output = %q, want %q", got, want)
		}
	})
}

func TestNextLabelFilter(t *testing.T) {
	r := newCLIRepo(t)
	r.seed(t, nextTestIssue(1, "Docs issue", "open", "critical", []string{"docs"}, nil))
	r.seed(t, nextTestIssue(2, "Auth bug", "open", "high", []string{"bug", "auth"}, nil))
	r.seed(t, nextTestIssue(3, "Unlabeled", "open", "medium", nil, nil))

	stdout := nextRun(t, r, "--label", "bug")
	if got, want := strings.TrimSpace(stdout), "2 high Auth bug [bug,auth]"; got != want {
		t.Errorf("next --label bug output = %q, want %q", got, want)
	}

	// Multiple labels use OR logic, so the critical docs issue wins.
	stdout = nextRun(t, r, "--label", "bug", "--label", "docs")
	if got, want := strings.TrimSpace(stdout), "1 critical Docs issue [docs]"; got != want {
		t.Errorf("next --label bug --label docs output = %q, want %q", got, want)
	}

	stdout = nextRun(t, r, "--label", "missing")
	if got, want := strings.TrimSpace(stdout), "No actionable issues found."; got != want {
		t.Errorf("next --label missing output = %q, want %q", got, want)
	}
}

func TestNextJSONFormat(t *testing.T) {
	r := newCLIRepo(t)
	r.seed(t, nextTestIssue(1, "JSON winner", "in-progress", "critical", []string{"bug", "auth"}, nil))
	r.seed(t, nextTestIssue(2, "Low", "open", "low", nil, nil))

	stdout := nextRun(t, r, "--format", "json")

	var got issue.Issue
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("next --format json is not valid JSON: %v\n%s", err, stdout)
	}
	if got.ID != 1 {
		t.Errorf("json id = %d, want 1", got.ID)
	}
	if got.Title != "JSON winner" {
		t.Errorf("json title = %q, want %q", got.Title, "JSON winner")
	}
	if got.Status != "in-progress" {
		t.Errorf("json status = %q, want in-progress", got.Status)
	}
	if got.Priority != "critical" {
		t.Errorf("json priority = %q, want critical", got.Priority)
	}
	if want := []string{"bug", "auth"}; !slices.Equal(got.Labels, want) {
		t.Errorf("json labels = %v, want %v", got.Labels, want)
	}
}

func TestNextJoinLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		want   string
	}{
		{"nil labels", nil, ""},
		{"empty labels", []string{}, ""},
		{"single label", []string{"bug"}, "bug"},
		{"multiple labels", []string{"bug", "auth"}, "bug,auth"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinLabels(tt.labels); got != tt.want {
				t.Errorf("joinLabels(%v) = %q, want %q", tt.labels, got, tt.want)
			}
		})
	}
}

func TestNextTextOutputWithoutLabels(t *testing.T) {
	r := newCLIRepo(t)
	r.seed(t, nextTestIssue(1, "Plain issue", "open", "medium", nil, nil))

	stdout := nextRun(t, r)
	if got, want := strings.TrimSpace(stdout), "1 medium Plain issue []"; got != want {
		t.Errorf("next output = %q, want %q", got, want)
	}
}
