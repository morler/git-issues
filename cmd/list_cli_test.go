package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/steviee/git-issues/internal/issue"
)

// listRun executes `issues list` with args and requires it to succeed.
func listRun(t *testing.T, repo *cliRepo, args ...string) (stdout, stderr string) {
	t.Helper()
	stdout, stderr, err := repo.run(t, append([]string{"list"}, args...)...)
	if err != nil {
		t.Fatalf("issues list %v: %v (stderr: %s)", args, err, stderr)
	}
	return stdout, stderr
}

// listRow returns the table line that starts with idPrefix.
func listRow(t *testing.T, table, idPrefix string) string {
	t.Helper()
	for _, line := range strings.Split(table, "\n") {
		if strings.HasPrefix(line, idPrefix) {
			return line
		}
	}
	t.Fatalf("table has no row starting with %q:\n%s", idPrefix, table)
	return ""
}

// listWriteIssueFile writes an issue file without calling issue.Save, so tests
// can control the updated timestamp.
func listWriteIssueFile(t *testing.T, repo *cliRepo, iss *issue.Issue) {
	t.Helper()
	if iss.Labels == nil {
		iss.Labels = []string{}
	}
	path := filepath.Join(repo.issuesDir, issue.Filename(iss.ID, issue.GenerateSlug(iss.Title)))
	if err := os.WriteFile(path, issue.Marshal(iss), 0o644); err != nil {
		t.Fatalf("write issue #%d: %v", iss.ID, err)
	}
}

func TestListCommandTable(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{
		ID: 7, Title: "Login fails with empty password", Status: "open", Priority: "high",
		Labels:    []string{"bug", "auth"},
		Relations: issue.Relations{Blocks: []int{8, 9}},
	})
	repo.seed(t, &issue.Issue{
		ID: 8, Title: "Session timeout", Status: "open", Priority: "medium",
		Labels: []string{"feature"},
	})
	repo.seed(t, &issue.Issue{
		ID: 9, Title: "Password reset", Status: "closed", Priority: "low",
	})

	stdout, _ := listRun(t, repo)

	wantContains(t, "table header", stdout, "ID    PRI")
	wantContains(t, "table header", stdout, "STATUS")
	wantContains(t, "table header", stdout, "TITLE")
	wantContains(t, "table header", stdout, "LABELS")

	row7 := listRow(t, stdout, "0007")
	wantContains(t, "row 0007", row7, "0007  high      open")
	wantContains(t, "row 0007", row7, "Login fails with empty password")
	wantContains(t, "row 0007", row7, "[bug, auth]")
	wantContains(t, "row 0007", row7, "⬛ 8")
	wantNotContains(t, "row 0007", row7, "⬛ 8,9")

	row8 := listRow(t, stdout, "0008")
	wantContains(t, "row 0008", row8, "Session timeout")
	wantNotContains(t, "row 0008", row8, "⬛")
	wantNotContains(t, "row 0008", row8, "⬜")

	// The closed issue is hidden by the default --status open.
	wantNotContains(t, "table output", stdout, "Password reset")
}

func TestListCommandEmpty(t *testing.T) {
	repo := newCLIRepo(t)

	stdout, _ := listRun(t, repo)
	if got := strings.TrimSpace(stdout); got != "No open issues found." {
		t.Errorf("empty table = %q, want %q", got, "No open issues found.")
	}

	// Closed issues do not turn the default view into an empty database.
	repo.seed(t, &issue.Issue{ID: 1, Title: "Only closed", Status: "closed", Closed: "2026-02-02"})
	stdout, _ = listRun(t, repo)
	if got := strings.TrimSpace(stdout); got != "No open issues found." {
		t.Errorf("closed-only table = %q, want %q", got, "No open issues found.")
	}

	// A filter that matches nothing names the filter instead of the database.
	stdout, _ = listRun(t, repo, "--label", "missing")
	if got := strings.TrimSpace(stdout); got != "No matching issues found." {
		t.Errorf("filtered empty table = %q, want %q", got, "No matching issues found.")
	}

	stdout, _ = listRun(t, repo, "--status", "closed", "--priority", "high")
	if got := strings.TrimSpace(stdout); got != "No matching issues found." {
		t.Errorf("combined filter empty table = %q, want %q", got, "No matching issues found.")
	}

	// Only an unfiltered empty database reports no issues at all.
	empty := newCLIRepo(t)
	stdout, _ = listRun(t, empty, "--status", "all")
	if got := strings.TrimSpace(stdout); got != "No issues found." {
		t.Errorf("unfiltered empty table = %q, want %q", got, "No issues found.")
	}
}

func TestListCommandSymbols(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 4, Title: "Depends on 7", Status: "open", Relations: issue.Relations{DependsOn: []int{7}}})
	repo.seed(t, &issue.Issue{ID: 5, Title: "Both symbols", Status: "open", Relations: issue.Relations{
		Blocks:    []int{8},
		DependsOn: []int{7},
	}})
	repo.seed(t, &issue.Issue{ID: 6, Title: "Only closed dependencies", Status: "open", Relations: issue.Relations{
		DependsOn: []int{9, 10},
	}})
	repo.seed(t, &issue.Issue{ID: 7, Title: "Blocks 8 and 9", Status: "open", Relations: issue.Relations{Blocks: []int{8, 9}}})
	repo.seed(t, &issue.Issue{ID: 8, Title: "Blocked and open", Status: "open"})
	repo.seed(t, &issue.Issue{ID: 9, Title: "Closed blockee", Status: "closed"})
	repo.seed(t, &issue.Issue{ID: 10, Title: "Wontfix blockee", Status: "wontfix"})

	stdout, _ := listRun(t, repo)

	row4 := listRow(t, stdout, "0004")
	wantContains(t, "row 0004", row4, "⬜ 7")
	wantNotContains(t, "row 0004", row4, "⬛")

	row5 := listRow(t, stdout, "0005")
	wantContains(t, "row 0005", row5, "⬛ 8  ⬜ 7")

	row6 := listRow(t, stdout, "0006")
	wantNotContains(t, "row 0006", row6, "⬜")

	row7 := listRow(t, stdout, "0007")
	wantContains(t, "row 0007", row7, "⬛ 8")
	wantNotContains(t, "row 0007", row7, "⬛ 8,9")

	wantContains(t, "legend", stdout, "⬛ = blocks open issues   ⬜ = blocked by open issue")
}

func TestListCommandNoSymbolsNoLegend(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Plain one"})
	repo.seed(t, &issue.Issue{ID: 2, Title: "Plain two"})

	stdout, _ := listRun(t, repo)

	wantContains(t, "table", stdout, "Plain one")
	wantNotContains(t, "table", stdout, "⬛")
	wantNotContains(t, "table", stdout, "⬜")
	wantNotContains(t, "table", stdout, "blocks open issues")
}

func TestListCommandJSON(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{
		ID: 1, Title: "JSON issue", Status: "open", Priority: "high",
		Labels:    []string{"bug", "auth"},
		Relations: issue.Relations{Blocks: []int{2}},
		Created:   "2026-01-15",
	})
	repo.seed(t, &issue.Issue{
		ID: 2, Title: "Closed issue", Status: "closed", Priority: "critical",
		Labels:  []string{"feature"},
		Created: "2026-02-20",
	})

	stdout, _ := listRun(t, repo, "--format", "json", "--status", "all", "--sort", "id")

	if trimmed := strings.TrimSpace(stdout); !strings.HasPrefix(trimmed, "[") {
		t.Fatalf("json output is not an array: %s", stdout)
	}

	var got []issue.Issue
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal list json: %v\n%s", err, stdout)
	}
	if len(got) != 2 {
		t.Fatalf("got %d issues, want 2: %s", len(got), stdout)
	}

	first := got[0]
	if first.ID != 1 || first.Title != "JSON issue" || first.Status != "open" || first.Priority != "high" {
		t.Errorf("first issue = %+v, want #1 JSON issue open high", first)
	}
	if !reflect.DeepEqual(first.Labels, []string{"bug", "auth"}) {
		t.Errorf("first issue labels = %v, want [bug auth]", first.Labels)
	}
	if !reflect.DeepEqual(first.Relations.Blocks, []int{2}) {
		t.Errorf("first issue blocks = %v, want [2]", first.Relations.Blocks)
	}
	if first.Created != "2026-01-15" {
		t.Errorf("first issue created = %q, want 2026-01-15", first.Created)
	}
	if first.Updated != issue.Today() {
		t.Errorf("first issue updated = %q, want %q", first.Updated, issue.Today())
	}

	if got[1].ID != 2 || got[1].Status != "closed" || got[1].Priority != "critical" {
		t.Errorf("second issue = %+v, want #2 closed critical", got[1])
	}

	wantNotContains(t, "json output", stdout, "file_path")
	wantNotContains(t, "json output", stdout, "FilePath")
}

func TestListCommandJSONEmptyIsArray(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Open issue", Status: "open", Priority: "medium"})

	stdout, _ := listRun(t, repo, "--format", "json", "--status", "all", "--priority", "critical")
	if got := strings.TrimSpace(stdout); got != "[]" {
		t.Errorf("json output with no matches = %q, want []", got)
	}
}

func TestListCommandIDs(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Open one"})
	repo.seed(t, &issue.Issue{ID: 2, Title: "Closed one", Status: "closed"})
	repo.seed(t, &issue.Issue{ID: 3, Title: "Open three"})

	stdout, _ := listRun(t, repo, "--format", "ids")
	if got := strings.TrimSpace(stdout); got != "1 3" {
		t.Errorf("ids (open) = %q, want %q", got, "1 3")
	}

	stdout, _ = listRun(t, repo, "--format", "ids", "--status", "all", "--sort", "id")
	if got := strings.TrimSpace(stdout); got != "1 2 3" {
		t.Errorf("ids (all) = %q, want %q", got, "1 2 3")
	}

	// No matches prints an empty line rather than an error.
	stdout, _ = listRun(t, repo, "--format", "ids", "--status", "wontfix")
	if got := strings.TrimSpace(stdout); got != "" {
		t.Errorf("ids (no match) = %q, want empty", got)
	}
}

func TestListCommandSort(t *testing.T) {
	t.Run("id", func(t *testing.T) {
		repo := newCLIRepo(t)
		for _, id := range []int{3, 1, 2} {
			repo.seed(t, &issue.Issue{ID: id, Title: "Issue"})
		}

		stdout, _ := listRun(t, repo, "--format", "ids", "--sort", "id")
		if got := strings.TrimSpace(stdout); got != "1 2 3" {
			t.Errorf("ids sorted by id = %q, want %q", got, "1 2 3")
		}
	})

	t.Run("priority", func(t *testing.T) {
		repo := newCLIRepo(t)
		repo.seed(t, &issue.Issue{ID: 1, Title: "Low", Priority: "low"})
		repo.seed(t, &issue.Issue{ID: 2, Title: "Critical one", Priority: "critical"})
		repo.seed(t, &issue.Issue{ID: 3, Title: "Critical two", Priority: "critical"})
		repo.seed(t, &issue.Issue{ID: 4, Title: "High", Priority: "high"})
		repo.seed(t, &issue.Issue{ID: 5, Title: "Medium", Priority: "medium"})

		stdout, _ := listRun(t, repo, "--format", "ids")
		if got := strings.TrimSpace(stdout); got != "2 3 4 5 1" {
			t.Errorf("ids sorted by priority = %q, want %q", got, "2 3 4 5 1")
		}
	})

	t.Run("created", func(t *testing.T) {
		repo := newCLIRepo(t)
		repo.seed(t, &issue.Issue{ID: 1, Title: "January", Created: "2026-01-01"})
		repo.seed(t, &issue.Issue{ID: 2, Title: "March", Created: "2026-03-01"})
		repo.seed(t, &issue.Issue{ID: 3, Title: "February", Created: "2026-02-01"})

		stdout, _ := listRun(t, repo, "--format", "ids", "--sort", "created")
		if got := strings.TrimSpace(stdout); got != "2 3 1" {
			t.Errorf("ids sorted by created = %q, want %q", got, "2 3 1")
		}
	})

	t.Run("updated", func(t *testing.T) {
		repo := newCLIRepo(t)
		listWriteIssueFile(t, repo, &issue.Issue{
			ID: 1, Title: "Oldest", Status: "open", Priority: "medium", Created: "2026-01-01", Updated: "2026-01-05",
		})
		listWriteIssueFile(t, repo, &issue.Issue{
			ID: 2, Title: "Newest", Status: "open", Priority: "medium", Created: "2026-01-01", Updated: "2026-04-01",
		})
		listWriteIssueFile(t, repo, &issue.Issue{
			ID: 3, Title: "Middle", Status: "open", Priority: "medium", Created: "2026-01-01", Updated: "2026-02-10",
		})

		stdout, _ := listRun(t, repo, "--format", "ids", "--sort", "updated")
		if got := strings.TrimSpace(stdout); got != "2 3 1" {
			t.Errorf("ids sorted by updated = %q, want %q", got, "2 3 1")
		}
	})
}

func TestListCommandFilters(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{
		ID: 1, Title: "Bug high", Status: "open", Priority: "high",
		Labels:    []string{"bug"},
		Relations: issue.Relations{Blocks: []int{2}},
	})
	repo.seed(t, &issue.Issue{
		ID: 2, Title: "Feature medium", Status: "open", Priority: "medium",
		Labels:    []string{"feature"},
		Relations: issue.Relations{DependsOn: []int{1}},
	})
	repo.seed(t, &issue.Issue{
		ID: 3, Title: "Closed critical", Status: "closed", Priority: "critical",
		Labels: []string{"bug", "auth"},
	})
	repo.seed(t, &issue.Issue{
		ID: 4, Title: "Started low", Status: "in-progress", Priority: "low",
		Labels: []string{"docs"},
	})
	repo.seed(t, &issue.Issue{
		ID: 5, Title: "Bug auth high", Status: "open", Priority: "high",
		Labels: []string{"bug", "auth"},
	})

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"default open", nil, "1 2 5"},
		{"all statuses", []string{"--status", "all"}, "1 2 3 4 5"},
		{"closed", []string{"--status", "closed"}, "3"},
		{"in-progress", []string{"--status", "in-progress"}, "4"},
		{"wontfix", []string{"--status", "wontfix"}, ""},
		{"priority", []string{"--status", "all", "--priority", "critical"}, "3"},
		{"single label", []string{"--status", "all", "--label", "auth"}, "3 5"},
		{"label OR logic", []string{"--status", "all", "--label", "bug", "--label", "feature"}, "1 2 3 5"},
		{"unknown label", []string{"--status", "all", "--label", "missing"}, ""},
		{"blocks", []string{"--status", "all", "--blocks", "2"}, "1"},
		{"depends-on", []string{"--status", "all", "--depends-on", "1"}, "2"},
		{"combined", []string{"--priority", "high", "--label", "auth"}, "5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"--format", "ids", "--sort", "id"}, tt.args...)
			stdout, _ := listRun(t, repo, args...)
			if got := strings.TrimSpace(stdout); got != tt.want {
				t.Errorf("list %v ids = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestListCommandSkipsBrokenFile(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Valid issue"})
	broken := filepath.Join(repo.issuesDir, "0099-broken.md")
	if err := os.WriteFile(broken, []byte("not an issue file\n"), 0o644); err != nil {
		t.Fatalf("write broken issue file: %v", err)
	}

	stdout, stderr := listRun(t, repo)

	wantContains(t, "table", stdout, "Valid issue")
	wantContains(t, "stderr", stderr, "Warning: skipping issue file")
}

func TestListCommandUnreadableIssuesDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Hidden"})
	if err := os.Chmod(repo.issuesDir, 0o000); err != nil {
		t.Fatalf("chmod .issues: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(repo.issuesDir, 0o755) })

	_, _, err := repo.run(t, "list")
	if err == nil {
		t.Fatal("expected list to fail when .issues/ is unreadable")
	}
	wantContains(t, "error", err.Error(), "permission denied")
}

func TestListCommandNoRepo(t *testing.T) {
	repo := newCLIRepo(t)
	if err := os.RemoveAll(repo.issuesDir); err != nil {
		t.Fatalf("remove .issues: %v", err)
	}

	_, _, err := repo.run(t, "list")
	if err == nil {
		t.Fatal("expected list to fail without .issues/")
	}
	wantContains(t, "error", err.Error(), "no .issues/ directory found")
}
