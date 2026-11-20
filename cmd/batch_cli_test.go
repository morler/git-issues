package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/steviee/git-issues/internal/issue"
)

// newBatchIssue builds an issue for batch tests with a distinctive title.
func newBatchIssue(id int, title, status string, labels []string) *issue.Issue {
	return &issue.Issue{
		ID:       id,
		Title:    title,
		Status:   status,
		Priority: "medium",
		Labels:   labels,
		Created:  "2026-01-01",
	}
}

// batchSnapshot reads every issue file so a test can prove files were untouched.
func batchSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read issues dir: %v", err)
	}
	files := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		files[entry.Name()] = string(data)
	}
	return files
}

// wantBatchUnchanged fails when any snapshot file changed on disk.
func wantBatchUnchanged(t *testing.T, dir string, before map[string]string) {
	t.Helper()
	after := batchSnapshot(t, dir)
	if len(after) != len(before) {
		t.Fatalf("issue file count changed: before %v, after %v", len(before), len(after))
	}
	for name, data := range before {
		if after[name] != data {
			t.Errorf("issue file %s changed unexpectedly", name)
		}
	}
}

func TestBatchCloseDefaultsToOpenOnly(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, newBatchIssue(1, "Open one", "open", nil))
	repo.seed(t, newBatchIssue(2, "In progress two", "in-progress", nil))
	repo.seed(t, newBatchIssue(3, "Closed three", "closed", nil))
	repo.seed(t, newBatchIssue(4, "Open four", "open", nil))

	stdout, _, err := repo.run(t, "batch", "close")
	if err != nil {
		t.Fatalf("batch close: %v", err)
	}

	wantContains(t, "stdout", stdout, "Closed: #1 Open one")
	wantContains(t, "stdout", stdout, "Closed: #4 Open four")
	wantNotContains(t, "stdout", stdout, "Closed: #2")
	wantNotContains(t, "stdout", stdout, "Closed: #3")
	wantContains(t, "stdout", stdout, "\n2 issues closed.\n")

	for _, id := range []int{1, 4} {
		iss := repo.load(t, id)
		if iss.Status != "closed" {
			t.Errorf("#%d status = %q, want closed", id, iss.Status)
		}
		if iss.Closed != issue.Today() {
			t.Errorf("#%d closed = %q, want %q", id, iss.Closed, issue.Today())
		}
	}
	if iss := repo.load(t, 2); iss.Status != "in-progress" || iss.Closed != "" {
		t.Errorf("#2 = %q/%q, want in-progress with empty closed", iss.Status, iss.Closed)
	}
	if iss := repo.load(t, 3); iss.Status != "closed" || iss.Closed != "" {
		t.Errorf("#3 = %q/%q, want untouched closed with empty closed date", iss.Status, iss.Closed)
	}
}

func TestBatchCloseStatusAll(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, newBatchIssue(1, "Open", "open", nil))
	repo.seed(t, newBatchIssue(2, "In progress", "in-progress", nil))
	repo.seed(t, newBatchIssue(3, "Closed", "closed", nil))
	repo.seed(t, newBatchIssue(4, "Wontfix", "wontfix", []string{"bug"}))

	stdout, _, err := repo.run(t, "batch", "close", "--status", "all")
	if err != nil {
		t.Fatalf("batch close --status all: %v", err)
	}
	wantContains(t, "stdout", stdout, "\n4 issues closed.\n")

	for id := 1; id <= 4; id++ {
		iss := repo.load(t, id)
		if iss.Status != "closed" {
			t.Errorf("#%d status = %q, want closed", id, iss.Status)
		}
		if iss.Closed != issue.Today() {
			t.Errorf("#%d closed = %q, want %q", id, iss.Closed, issue.Today())
		}
	}
}

func TestBatchCloseLabelFilter(t *testing.T) {
	t.Run("single label", func(t *testing.T) {
		repo := newCLIRepo(t)
		repo.seed(t, newBatchIssue(1, "Auth only", "open", []string{"auth"}))
		repo.seed(t, newBatchIssue(2, "Bug only", "open", []string{"bug"}))
		repo.seed(t, newBatchIssue(3, "Auth and bug", "open", []string{"bug", "auth"}))
		repo.seed(t, newBatchIssue(4, "No labels", "open", nil))
		before := batchSnapshot(t, repo.issuesDir)

		stdout, _, err := repo.run(t, "batch", "close", "--label", "auth")
		if err != nil {
			t.Fatalf("batch close --label auth: %v", err)
		}
		wantContains(t, "stdout", stdout, "\n2 issues closed.\n")

		for _, id := range []int{1, 3} {
			if iss := repo.load(t, id); iss.Status != "closed" {
				t.Errorf("#%d status = %q, want closed", id, iss.Status)
			}
		}
		for _, id := range []int{2, 4} {
			if iss := repo.load(t, id); iss.Status != "open" {
				t.Errorf("#%d status = %q, want open", id, iss.Status)
			}
		}
		for _, name := range []string{"0002-bug-only.md", "0004-no-labels.md"} {
			if got := readFile(t, filepath.Join(repo.issuesDir, name)); got != before[name] {
				t.Errorf("%s was rewritten but should be untouched", name)
			}
		}
	})

	t.Run("label OR logic with status all", func(t *testing.T) {
		repo := newCLIRepo(t)
		repo.seed(t, newBatchIssue(1, "Auth open", "open", []string{"auth"}))
		repo.seed(t, newBatchIssue(2, "Feature closed", "closed", []string{"feature"}))
		repo.seed(t, newBatchIssue(3, "Docs open", "open", []string{"docs"}))
		repo.seed(t, newBatchIssue(4, "Security in progress", "in-progress", []string{"security"}))

		stdout, _, err := repo.run(t, "batch", "close", "--status", "all", "--label", "auth", "--label", "feature")
		if err != nil {
			t.Fatalf("batch close with two labels: %v", err)
		}
		wantContains(t, "stdout", stdout, "\n2 issues closed.\n")

		for id, want := range map[int]string{1: "closed", 2: "closed", 3: "open", 4: "in-progress"} {
			if iss := repo.load(t, id); iss.Status != want {
				t.Errorf("#%d status = %q, want %q", id, iss.Status, want)
			}
		}
	})
}

func TestBatchCloseWontfix(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, newBatchIssue(1, "Open one", "open", nil))
	repo.seed(t, newBatchIssue(2, "Open two", "open", []string{"bug"}))
	repo.seed(t, newBatchIssue(3, "In progress", "in-progress", nil))

	stdout, _, err := repo.run(t, "batch", "close", "--wontfix")
	if err != nil {
		t.Fatalf("batch close --wontfix: %v", err)
	}
	wantContains(t, "stdout", stdout, "\n2 issues closed.\n")
	wantNotContains(t, "stdout", stdout, "#3")

	for _, id := range []int{1, 2} {
		iss := repo.load(t, id)
		if iss.Status != "wontfix" {
			t.Errorf("#%d status = %q, want wontfix", id, iss.Status)
		}
		if iss.Closed != issue.Today() {
			t.Errorf("#%d closed = %q, want %q", id, iss.Closed, issue.Today())
		}
	}
	if iss := repo.load(t, 3); iss.Status != "in-progress" || iss.Closed != "" {
		t.Errorf("#3 = %q/%q, want unchanged in-progress", iss.Status, iss.Closed)
	}
}

func TestBatchCloseNoMatches(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, newBatchIssue(1, "In progress", "in-progress", nil))
	repo.seed(t, newBatchIssue(2, "Closed", "closed", []string{"bug"}))
	before := batchSnapshot(t, repo.issuesDir)

	stdout, _, err := repo.run(t, "batch", "close")
	if err != nil {
		t.Fatalf("batch close: %v", err)
	}
	wantContains(t, "stdout", stdout, "0 issues closed.")
	wantNotContains(t, "stdout", stdout, "Closed: #")

	wantBatchUnchanged(t, repo.issuesDir, before)
}

func TestBatchCloseRefusesOpenBlockers(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Root", Status: "open"})
	repo.seed(t, &issue.Issue{
		ID: 2, Title: "Blocked", Status: "open",
		Relations: issue.Relations{DependsOn: []int{1}},
	})
	repo.seed(t, &issue.Issue{ID: 3, Title: "Free", Status: "open"})
	before := batchSnapshot(t, repo.issuesDir)

	stdout, stderr, err := repo.run(t, "batch", "close")
	if err == nil {
		t.Fatal("batch close ignored an open blocker")
	}
	wantContains(t, "error", err.Error(), "refusing to close #2 with open blockers (use --force)")
	wantContains(t, "stderr", stderr, "Warning: #2 has open blocker: #1 (Root)")
	wantNotContains(t, "stdout", stdout, "Closed:")
	wantBatchUnchanged(t, repo.issuesDir, before)
}

func TestBatchCloseForceWithOpenBlockers(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Root", Status: "open"})
	repo.seed(t, &issue.Issue{
		ID: 2, Title: "Blocked", Status: "open",
		Relations: issue.Relations{DependsOn: []int{1}},
	})
	repo.seed(t, &issue.Issue{ID: 3, Title: "Free", Status: "open"})

	stdout, _, err := repo.run(t, "batch", "close", "--force")
	if err != nil {
		t.Fatalf("batch close --force: %v", err)
	}
	wantContains(t, "stdout", stdout, "\n3 issues closed.\n")
	for id := 1; id <= 3; id++ {
		if got := repo.load(t, id).Status; got != "closed" {
			t.Errorf("#%d status = %q, want closed", id, got)
		}
	}
}

func TestBatchCloseAllowsResolvedBlockers(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Root", Status: "closed"})
	repo.seed(t, &issue.Issue{
		ID: 2, Title: "Blocked", Status: "open",
		Relations: issue.Relations{DependsOn: []int{1}},
	})

	if _, _, err := repo.run(t, "batch", "close"); err != nil {
		t.Fatalf("batch close with resolved blocker: %v", err)
	}
	if got := repo.load(t, 2).Status; got != "closed" {
		t.Errorf("#2 status = %q, want closed", got)
	}
}

func TestBatchSetPriority(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, newBatchIssue(1, "Open one", "open", nil))
	repo.seed(t, newBatchIssue(2, "Open two", "open", []string{"bug"}))
	repo.seed(t, newBatchIssue(3, "In progress", "in-progress", nil))

	stdout, _, err := repo.run(t, "batch", "set", "priority", "critical")
	if err != nil {
		t.Fatalf("batch set priority: %v", err)
	}
	wantContains(t, "stdout", stdout, "Updated #1: priority = critical")
	wantContains(t, "stdout", stdout, "Updated #2: priority = critical")
	wantContains(t, "stdout", stdout, "\n2 issues updated.\n")
	wantNotContains(t, "stdout", stdout, "Updated #3")

	for _, id := range []int{1, 2} {
		iss := repo.load(t, id)
		if iss.Priority != "critical" {
			t.Errorf("#%d priority = %q, want critical", id, iss.Priority)
		}
		if iss.Status != "open" {
			t.Errorf("#%d status = %q, want open", id, iss.Status)
		}
	}
	if iss := repo.load(t, 3); iss.Priority != "medium" {
		t.Errorf("#3 priority = %q, want medium", iss.Priority)
	}
}

func TestBatchSetStatus(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, newBatchIssue(1, "One", "open", nil))
	repo.seed(t, newBatchIssue(2, "Two", "open", nil))

	stdout, _, err := repo.run(t, "batch", "set", "status", "in-progress")
	if err != nil {
		t.Fatalf("batch set status in-progress: %v", err)
	}
	wantContains(t, "stdout", stdout, "\n2 issues updated.\n")
	for _, id := range []int{1, 2} {
		iss := repo.load(t, id)
		if iss.Status != "in-progress" {
			t.Errorf("#%d status = %q, want in-progress", id, iss.Status)
		}
		if iss.Closed != "" {
			t.Errorf("#%d closed = %q, want empty", id, iss.Closed)
		}
	}

	stdout, _, err = repo.run(t, "batch", "set", "status", "closed", "--status", "all")
	if err != nil {
		t.Fatalf("batch set status closed: %v", err)
	}
	wantContains(t, "stdout", stdout, "\n2 issues updated.\n")
	for _, id := range []int{1, 2} {
		iss := repo.load(t, id)
		if iss.Status != "closed" {
			t.Errorf("#%d status = %q, want closed", id, iss.Status)
		}
		if iss.Closed != issue.Today() {
			t.Errorf("#%d closed = %q, want %q", id, iss.Closed, issue.Today())
		}
	}

	stdout, _, err = repo.run(t, "batch", "set", "status", "open", "--status", "closed")
	if err != nil {
		t.Fatalf("batch set status open: %v", err)
	}
	wantContains(t, "stdout", stdout, "\n2 issues updated.\n")
	for _, id := range []int{1, 2} {
		iss := repo.load(t, id)
		if iss.Status != "open" {
			t.Errorf("#%d status = %q, want open", id, iss.Status)
		}
		if iss.Closed != "" {
			t.Errorf("#%d closed = %q, want cleared", id, iss.Closed)
		}
	}
}

func TestBatchSetLabel(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, newBatchIssue(1, "Bug only", "open", []string{"bug"}))
	repo.seed(t, newBatchIssue(2, "Bug and auth", "open", []string{"bug", "auth"}))
	repo.seed(t, newBatchIssue(3, "No labels", "open", nil))

	stdout, _, err := repo.run(t, "batch", "set", "label", "+security")
	if err != nil {
		t.Fatalf("batch set label +security: %v", err)
	}
	wantContains(t, "stdout", stdout, "Updated #1: label = +security")
	wantContains(t, "stdout", stdout, "\n3 issues updated.\n")

	want := map[int][]string{
		1: {"bug", "security"},
		2: {"bug", "auth", "security"},
		3: {"security"},
	}
	for id, labels := range want {
		if got := repo.load(t, id).Labels; !slices.Equal(got, labels) {
			t.Errorf("#%d labels = %v, want %v", id, got, labels)
		}
	}

	// Adding an existing label must not duplicate it.
	if _, _, err := repo.run(t, "batch", "set", "label", "+security"); err != nil {
		t.Fatalf("batch set label +security again: %v", err)
	}
	if got := repo.load(t, 2).Labels; !slices.Equal(got, []string{"bug", "auth", "security"}) {
		t.Errorf("#2 labels after duplicate add = %v", got)
	}

	// The leading dash makes cobra parse "-bug" as a flag, so it needs "--".
	stdout, _, err = repo.run(t, "batch", "set", "--", "label", "-bug")
	if err != nil {
		t.Fatalf("batch set label -bug: %v", err)
	}
	wantContains(t, "stdout", stdout, "Updated #1: label = -bug")
	if got := repo.load(t, 1).Labels; !slices.Equal(got, []string{"security"}) {
		t.Errorf("#1 labels after -bug = %v, want [security]", got)
	}
	if got := repo.load(t, 2).Labels; !slices.Equal(got, []string{"auth", "security"}) {
		t.Errorf("#2 labels after -bug = %v, want [auth security]", got)
	}

	stdout, _, err = repo.run(t, "batch", "set", "label", "bug, auth")
	if err != nil {
		t.Fatalf("batch set label replace: %v", err)
	}
	wantContains(t, "stdout", stdout, "Updated #3: label = bug, auth")
	for id := 1; id <= 3; id++ {
		if got := repo.load(t, id).Labels; !slices.Equal(got, []string{"bug", "auth"}) {
			t.Errorf("#%d labels after replace = %v, want [bug auth]", id, got)
		}
	}
}

func TestBatchSetUnknownField(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, newBatchIssue(1, "Open", "open", nil))
	before := batchSnapshot(t, repo.issuesDir)

	_, _, err := repo.run(t, "batch", "set", "title", "New title")
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	wantContains(t, "error", err.Error(), `unknown field "title" for batch set`)
	wantContains(t, "error", err.Error(), "priority, status, label")

	wantBatchUnchanged(t, repo.issuesDir, before)
}

func TestBatchSetInvalidPriority(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, newBatchIssue(1, "Open", "open", nil))
	before := batchSnapshot(t, repo.issuesDir)

	_, _, err := repo.run(t, "batch", "set", "priority", "urgent")
	if err == nil {
		t.Fatal("expected error for invalid priority")
	}
	wantContains(t, "error", err.Error(), `invalid priority "urgent"`)
	wantContains(t, "error", err.Error(), "low, medium, high, critical")

	wantBatchUnchanged(t, repo.issuesDir, before)
}

func TestBatchSetInvalidStatus(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, newBatchIssue(1, "Open", "open", nil))
	before := batchSnapshot(t, repo.issuesDir)

	_, _, err := repo.run(t, "batch", "set", "status", "done")
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
	wantContains(t, "error", err.Error(), `invalid status "done"`)
	wantContains(t, "error", err.Error(), "open, in-progress, closed, wontfix")

	wantBatchUnchanged(t, repo.issuesDir, before)
}

func TestBatchSetNoMatchesStillValidates(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"invalid priority", []string{"batch", "set", "priority", "urgent", "--status", "closed"}, `invalid priority "urgent"`},
		{"invalid status", []string{"batch", "set", "status", "done", "--status", "closed"}, `invalid status "done"`},
		{"unknown field", []string{"batch", "set", "title", "New title", "--status", "closed"}, `unknown field "title" for batch set`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newCLIRepo(t)
			repo.seed(t, newBatchIssue(1, "Open only", "open", nil))
			before := batchSnapshot(t, repo.issuesDir)

			stdout, _, err := repo.run(t, tt.args...)
			if err == nil {
				t.Fatalf("%v accepted invalid input when no issue matched", tt.args)
			}
			wantContains(t, "error", err.Error(), tt.want)
			wantNotContains(t, "stdout", stdout, "issues updated")
			wantBatchUnchanged(t, repo.issuesDir, before)
		})
	}
}

func TestBatchSetLabelFilterNoMatches(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, newBatchIssue(1, "Bug", "open", []string{"bug"}))
	repo.seed(t, newBatchIssue(2, "Auth", "open", []string{"auth"}))
	before := batchSnapshot(t, repo.issuesDir)

	stdout, _, err := repo.run(t, "batch", "set", "priority", "high", "--label", "missing")
	if err != nil {
		t.Fatalf("batch set priority --label missing: %v", err)
	}
	wantContains(t, "stdout", stdout, "0 issues updated.")
	wantNotContains(t, "stdout", stdout, "Updated #")
	wantBatchUnchanged(t, repo.issuesDir, before)
}

func TestBatchSetNoMatches(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, newBatchIssue(1, "Closed", "closed", nil))
	repo.seed(t, newBatchIssue(2, "Wontfix", "wontfix", []string{"bug"}))
	before := batchSnapshot(t, repo.issuesDir)

	stdout, _, err := repo.run(t, "batch", "set", "priority", "high")
	if err != nil {
		t.Fatalf("batch set priority: %v", err)
	}
	wantContains(t, "stdout", stdout, "0 issues updated.")
	wantNotContains(t, "stdout", stdout, "Updated #")

	wantBatchUnchanged(t, repo.issuesDir, before)
}

// newBatchRawIssue writes an issue file verbatim, bypassing validation so a
// command can be forced to hit its Save error path.
func newBatchRawIssue(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write raw issue %s: %v", name, err)
	}
}

func TestBatchSaveErrors(t *testing.T) {
	const invalid = "---\nid: 9\ntitle: \"\"\nstatus: open\npriority: medium\ncreated: 2026-01-01\n---\n"

	t.Run("close", func(t *testing.T) {
		repo := newCLIRepo(t)
		repo.seed(t, newBatchIssue(1, "Valid", "open", nil))
		newBatchRawIssue(t, repo.issuesDir, "0009-broken.md", invalid)

		stdout, _, err := repo.run(t, "batch", "close")
		if err != nil {
			t.Fatalf("batch close: %v", err)
		}
		wantContains(t, "stdout", stdout, "Error closing #9: title must not be empty")
		wantContains(t, "stdout", stdout, "\n1 issues closed.\n")
		if iss := repo.load(t, 1); iss.Status != "closed" {
			t.Errorf("#1 status = %q, want closed", iss.Status)
		}
		if got := readFile(t, filepath.Join(repo.issuesDir, "0009-broken.md")); got != invalid {
			t.Errorf("broken issue file was rewritten:\n%s", got)
		}
	})

	t.Run("set", func(t *testing.T) {
		repo := newCLIRepo(t)
		repo.seed(t, newBatchIssue(1, "Valid", "open", nil))
		newBatchRawIssue(t, repo.issuesDir, "0009-broken.md", invalid)

		stdout, _, err := repo.run(t, "batch", "set", "priority", "high")
		if err != nil {
			t.Fatalf("batch set priority: %v", err)
		}
		wantContains(t, "stdout", stdout, "Error updating #9: title must not be empty")
		wantContains(t, "stdout", stdout, "\n1 issues updated.\n")
		if iss := repo.load(t, 1); iss.Priority != "high" {
			t.Errorf("#1 priority = %q, want high", iss.Priority)
		}
	})
}

func TestBatchConfigLoadError(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, newBatchIssue(1, "Open", "open", nil))
	if err := os.WriteFile(filepath.Join(repo.issuesDir, ".config.yml"), []byte("auto_stage: [broken\n"), 0o644); err != nil {
		t.Fatalf("write broken config: %v", err)
	}

	for _, args := range [][]string{{"batch", "close"}, {"batch", "set", "priority", "high"}} {
		if _, _, err := repo.run(t, args...); err == nil {
			t.Errorf("%v: expected config error", args)
		}
	}
}

func TestBatchLoadAllError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("directory permissions are not enforced for root")
	}
	repo := newCLIRepo(t)
	repo.seed(t, newBatchIssue(1, "Open", "open", nil))

	// Search permission keeps .config.yml readable while making ReadDir fail.
	if err := os.Chmod(repo.issuesDir, 0o111); err != nil {
		t.Fatalf("chmod .issues: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(repo.issuesDir, 0o755); err != nil {
			t.Errorf("restore .issues permissions: %v", err)
		}
	})

	for _, args := range [][]string{{"batch", "close"}, {"batch", "set", "priority", "high"}} {
		if _, _, err := repo.run(t, args...); err == nil {
			t.Errorf("%v: expected directory read error", args)
		}
	}
}

func TestBatchOutsideRepo(t *testing.T) {
	dir := t.TempDir()
	for probe := dir; ; probe = filepath.Dir(probe) {
		if _, err := os.Stat(filepath.Join(probe, ".issues")); err == nil {
			t.Skipf("%s has an .issues directory", probe)
		}
		if filepath.Dir(probe) == probe {
			break
		}
	}
	t.Chdir(dir)

	for _, args := range [][]string{{"batch", "close"}, {"batch", "set", "priority", "high"}} {
		_, _, err := runCLI(t, rootCmd, args...)
		if err == nil {
			t.Fatalf("%v: expected error outside a repository", args)
		}
		wantContains(t, "error", err.Error(), "no .issues/ directory found")
	}
}

func TestBatchAutoStage(t *testing.T) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}

	tests := []struct {
		name   string
		args   []string
		status string
	}{
		{"close", []string{"batch", "close"}, "closed"},
		{"set", []string{"batch", "set", "priority", "high"}, "open"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newCLIRepo(t)
			repo.enableAutoStage(t)
			cmd := exec.Command(gitBin, "init", "-q")
			cmd.Dir = repo.root
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git init: %v: %s", err, out)
			}
			repo.seed(t, newBatchIssue(1, "Open", "open", nil))

			_, stderr, err := repo.run(t, tt.args...)
			if err != nil {
				t.Fatalf("batch %s: %v", tt.name, err)
			}
			wantNotContains(t, "stderr", stderr, "Warning: could not stage file")
			if iss := repo.load(t, 1); iss.Status != tt.status {
				t.Errorf("#1 status = %q, want %q", iss.Status, tt.status)
			}

			statusCmd := exec.Command(gitBin, "status", "--porcelain")
			statusCmd.Dir = repo.root
			out, err := statusCmd.Output()
			if err != nil {
				t.Fatalf("git status: %v", err)
			}
			if !strings.HasPrefix(strings.TrimSpace(string(out)), "A") {
				t.Errorf("issue file not staged, git status:\n%s", out)
			}
		})
	}
}
