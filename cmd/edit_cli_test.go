package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/steviee/git-issues/internal/issue"
)

// editTestEditor writes a fake $EDITOR: content is stored in a payload file and
// the script copies it over the path passed by `issues edit "$1"`.
func editTestEditor(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	payload := filepath.Join(dir, "payload.md")
	if err := os.WriteFile(payload, []byte(content), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	script := filepath.Join(dir, "editor.sh")
	body := "#!/bin/sh\ncat '" + payload + "' > \"$1\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}
	t.Setenv("EDITOR", script)
}

// editTestContent re-serializes the issue file after mutate, so tests hand a
// complete edited document to the fake editor.
func editTestContent(t *testing.T, path string, mutate func(*issue.Issue)) string {
	t.Helper()
	iss, err := issue.ParseFile(path)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	mutate(iss)
	return string(issue.Marshal(iss))
}

func TestEditRewritesTitleLabelsAndBody(t *testing.T) {
	repo := newCLIRepo(t)
	orig := repo.seed(t, &issue.Issue{
		ID:     1,
		Title:  "Original title",
		Labels: []string{"bug"},
		Body:   "Original body.\n",
	})

	editTestEditor(t, editTestContent(t, orig.FilePath, func(iss *issue.Issue) {
		iss.Title = "Edited title"
		iss.Priority = "high"
		iss.Labels = []string{"bug", "docs"}
		iss.Body = "Edited body text.\n"
	}))

	stdout, _, err := repo.run(t, "edit", "1")
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	wantContains(t, "stdout", stdout, "Updated: #1")

	loaded := repo.load(t, 1)
	if loaded.Title != "Edited title" {
		t.Errorf("title = %q, want %q", loaded.Title, "Edited title")
	}
	if loaded.Priority != "high" {
		t.Errorf("priority = %q, want high", loaded.Priority)
	}
	if !slices.Equal(loaded.Labels, []string{"bug", "docs"}) {
		t.Errorf("labels = %v, want [bug docs]", loaded.Labels)
	}
	wantContains(t, "body", loaded.Body, "Edited body text.")
	wantNotContains(t, "body", loaded.Body, "Original body.")
	if loaded.Updated != issue.Today() {
		t.Errorf("updated = %q, want %q", loaded.Updated, issue.Today())
	}

	// The filename is derived from the title only at creation time, so a title
	// edit must not add a second file for the same ID.
	matches, err := filepath.Glob(filepath.Join(repo.issuesDir, "*.md"))
	if err != nil {
		t.Fatalf("glob issue files: %v", err)
	}
	if len(matches) != 1 || matches[0] != orig.FilePath {
		t.Errorf("issue files = %v, want only %s", matches, orig.FilePath)
	}
}

func TestEditAddsBlocksRelationBidirectionally(t *testing.T) {
	repo := newCLIRepo(t)
	source := repo.seed(t, &issue.Issue{ID: 1, Title: "Source issue"})
	repo.seed(t, &issue.Issue{ID: 2, Title: "Target issue"})

	editTestEditor(t, editTestContent(t, source.FilePath, func(iss *issue.Issue) {
		iss.Relations.Blocks = []int{2}
	}))

	if _, _, err := repo.run(t, "edit", "1"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	if got := repo.load(t, 1).Relations.Blocks; !slices.Equal(got, []int{2}) {
		t.Errorf("#1 blocks = %v, want [2]", got)
	}
	if got := repo.load(t, 2).Relations.DependsOn; !slices.Equal(got, []int{1}) {
		t.Errorf("#2 depends-on = %v, want [1]", got)
	}
}

func TestEditRemovesRelationBidirectionally(t *testing.T) {
	repo := newCLIRepo(t)
	source := repo.seed(t, &issue.Issue{
		ID:        1,
		Title:     "Source issue",
		Relations: issue.Relations{Blocks: []int{2}},
	})
	repo.seed(t, &issue.Issue{
		ID:        2,
		Title:     "Target issue",
		Relations: issue.Relations{DependsOn: []int{1}},
	})

	editTestEditor(t, editTestContent(t, source.FilePath, func(iss *issue.Issue) {
		iss.Relations.Blocks = nil
	}))

	if _, _, err := repo.run(t, "edit", "1"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	if got := repo.load(t, 1).Relations.Blocks; len(got) != 0 {
		t.Errorf("#1 blocks = %v, want empty", got)
	}
	if got := repo.load(t, 2).Relations.DependsOn; len(got) != 0 {
		t.Errorf("#2 depends-on = %v, want empty", got)
	}
}

func TestEditRejectsEmptyTitle(t *testing.T) {
	repo := newCLIRepo(t)
	orig := repo.seed(t, &issue.Issue{ID: 1, Title: "Keep me"})
	repo.seed(t, &issue.Issue{ID: 2, Title: "Untouched target"})

	// The editor blanks the title, adds a relation and stamps an old updated
	// date, so a save would be visible both in `updated` and in #2's relations.
	content := editTestContent(t, orig.FilePath, func(iss *issue.Issue) {
		iss.Title = ""
		iss.Updated = "2020-01-01"
		iss.Relations.Blocks = []int{2}
	})
	editTestEditor(t, content)

	stdout, _, err := repo.run(t, "edit", "1")
	if err == nil {
		t.Fatal("edit accepted an empty title")
	}
	wantContains(t, "error", err.Error(), "title must not be empty")
	wantNotContains(t, "stdout", stdout, "Updated:")

	// Validation runs before Save, so the issue is not re-serialized and no
	// relation is synced to #2.
	failed, err := issue.ParseFile(orig.FilePath)
	if err != nil {
		t.Fatalf("parse file after rejected edit: %v", err)
	}
	if failed.Updated != "2020-01-01" {
		t.Errorf("updated = %q, want 2020-01-01 (Save must not run)", failed.Updated)
	}
	if got := repo.load(t, 2).Relations.DependsOn; len(got) != 0 {
		t.Errorf("#2 depends-on = %v, want empty (relations must not sync on failed edit)", got)
	}
	if got := readFile(t, orig.FilePath); got != content {
		t.Errorf("on-disk content differs from editor output (partial write):\nwant:\n%s\ngot:\n%s", content, got)
	}
}

func TestEditRejectsInvalidFrontmatter(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*issue.Issue)
		want   string
	}{
		{"status", func(iss *issue.Issue) { iss.Status = "done" }, `invalid status "done"`},
		{"priority", func(iss *issue.Issue) { iss.Priority = "urgent" }, `invalid priority "urgent"`},
		{"id", func(iss *issue.Issue) { iss.ID = 0 }, "id must be > 0"},
		{"created", func(iss *issue.Issue) { iss.Created = "01.01.2026" }, `invalid created date "01.01.2026"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newCLIRepo(t)
			orig := repo.seed(t, &issue.Issue{ID: 1, Title: "Valid title"})
			repo.seed(t, &issue.Issue{ID: 2, Title: "Untouched target"})

			content := editTestContent(t, orig.FilePath, func(iss *issue.Issue) {
				iss.Updated = "2020-01-01"
				tt.mutate(iss)
				iss.Relations.Blocks = []int{2}
			})
			editTestEditor(t, content)

			stdout, _, err := repo.run(t, "edit", "1")
			if err == nil {
				t.Fatalf("edit accepted invalid %s", tt.name)
			}
			wantContains(t, "error", err.Error(), tt.want)
			wantNotContains(t, "stdout", stdout, "Updated:")

			failed, parseErr := issue.ParseFile(orig.FilePath)
			if parseErr != nil {
				t.Fatalf("parse file after rejected edit: %v", parseErr)
			}
			if failed.Updated != "2020-01-01" {
				t.Errorf("updated = %q, want 2020-01-01 (Save must not run)", failed.Updated)
			}
			if got := repo.load(t, 2).Relations.DependsOn; len(got) != 0 {
				t.Errorf("#2 depends-on = %v, want empty", got)
			}
		})
	}
}

func TestEditRejectsChangedID(t *testing.T) {
	repo := newCLIRepo(t)
	orig := repo.seed(t, &issue.Issue{ID: 1, Title: "Keep me"})
	repo.seed(t, &issue.Issue{ID: 2, Title: "Untouched target"})
	before := readFile(t, orig.FilePath)

	content := editTestContent(t, orig.FilePath, func(iss *issue.Issue) {
		iss.ID = 5
		iss.Updated = "2020-01-01"
		iss.Relations.Blocks = []int{2}
	})
	editTestEditor(t, content)

	stdout, _, err := repo.run(t, "edit", "1")
	if err == nil {
		t.Fatal("edit accepted a changed id")
	}
	wantContains(t, "error", err.Error(), "issue id changed from #1 to #5")
	wantContains(t, "error", err.Error(), "original file restored")
	wantNotContains(t, "stdout", stdout, "Updated:")

	// The pre-edit content is restored, so #1 stays addressable and neither Save
	// nor relation sync ran with the changed id.
	if got := readFile(t, orig.FilePath); got != before {
		t.Errorf("file was not restored:\nwant:\n%s\ngot:\n%s", before, got)
	}
	if got := repo.load(t, 1).Title; got != "Keep me" {
		t.Errorf("#1 title = %q, want %q", got, "Keep me")
	}
	if got := repo.load(t, 2).Relations.DependsOn; len(got) != 0 {
		t.Errorf("#2 depends-on = %v, want empty (relations must not sync on failed edit)", got)
	}
	matches, err := filepath.Glob(filepath.Join(repo.issuesDir, "*.md"))
	if err != nil {
		t.Fatalf("glob issue files: %v", err)
	}
	if len(matches) != 2 {
		t.Errorf("issue files = %v, want no file created for the new id", matches)
	}
}

func TestEditRejectsFilenameIDMismatch(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{"prefixed mismatch", "0002-legacy.md", "filename 0002-legacy.md does not match issue #1"},
		{"no id prefix", "legacy.md", "filename \"legacy.md\" does not start with an issue ID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newCLIRepo(t)
			iss := &issue.Issue{
				ID: 1, Title: "Legacy", Status: "open", Priority: "medium",
				Labels: []string{}, Created: "2026-01-01", Updated: "2026-01-01",
			}
			path := filepath.Join(repo.issuesDir, tt.filename)
			content := string(issue.Marshal(iss))
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatalf("write %s: %v", tt.filename, err)
			}
			editTestEditor(t, content)

			stdout, _, err := repo.run(t, "edit", "1")
			if err == nil {
				t.Fatalf("edit accepted filename %s for issue #1", tt.filename)
			}
			wantContains(t, "error", err.Error(), tt.want)
			wantNotContains(t, "stdout", stdout, "Updated:")
		})
	}
}

func TestEditEditorFailureLeavesFileUntouched(t *testing.T) {
	repo := newCLIRepo(t)
	orig := repo.seed(t, &issue.Issue{ID: 1, Title: "Untouched"})
	before := readFile(t, orig.FilePath)

	t.Setenv("EDITOR", "/bin/false")

	_, _, err := repo.run(t, "edit", "1")
	if err == nil {
		t.Fatal("edit succeeded although the editor failed")
	}
	wantContains(t, "error", err.Error(), "editor failed")
	wantContains(t, "error", err.Error(), "exit status 1")

	if got := readFile(t, orig.FilePath); got != before {
		t.Errorf("file changed after editor failure:\nwant:\n%s\ngot:\n%s", before, got)
	}
}

func TestEditMissingIssue(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Only issue"})

	marker := filepath.Join(t.TempDir(), "editor-ran")
	script := filepath.Join(t.TempDir(), "editor.sh")
	body := "#!/bin/sh\ntouch '" + marker + "'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}
	t.Setenv("EDITOR", script)

	_, _, err := repo.run(t, "edit", "7")
	if err == nil {
		t.Fatal("edit succeeded for a missing issue")
	}
	wantContains(t, "error", err.Error(), "issue #7 not found")
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Error("editor was launched for a missing issue")
	}
}

func TestEditWithoutIssuesDir(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("EDITOR", "/bin/false")

	_, _, err := runCLI(t, rootCmd, "edit", "1")
	if err == nil {
		t.Fatal("edit succeeded without a .issues/ directory")
	}
	wantContains(t, "error", err.Error(), "no .issues/ directory found")
}

func TestEditBrokenConfig(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Needs config"})
	if err := os.WriteFile(filepath.Join(repo.issuesDir, ".config.yml"), []byte("auto_stage: [oops\n"), 0o644); err != nil {
		t.Fatalf("write broken config: %v", err)
	}
	t.Setenv("EDITOR", "/bin/false")

	_, _, err := repo.run(t, "edit", "1")
	if err == nil {
		t.Fatal("edit succeeded with a broken config file")
	}
	wantContains(t, "error", err.Error(), "yaml:")
	wantNotContains(t, "error", err.Error(), "editor failed")
}

func TestEditRejectsUnparsableEditorOutput(t *testing.T) {
	repo := newCLIRepo(t)
	orig := repo.seed(t, &issue.Issue{ID: 1, Title: "Keep me"})

	editTestEditor(t, "no frontmatter here\n")

	_, _, err := repo.run(t, "edit", "1")
	if err == nil {
		t.Fatal("edit accepted unparsable editor output")
	}
	wantContains(t, "error", err.Error(), "error parsing edited file")
	wantContains(t, "error", err.Error(), "file must start with ---")

	if got := readFile(t, orig.FilePath); got != "no frontmatter here\n" {
		t.Errorf("file after failed parse = %q, want editor output untouched", got)
	}
}

func TestEditFallsBackToViWhenEditorUnset(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Needs editor"})
	t.Setenv("EDITOR", "")
	t.Setenv("PATH", "")

	_, _, err := repo.run(t, "edit", "1")
	if err == nil {
		t.Fatal("edit succeeded without an editor")
	}
	wantContains(t, "error", err.Error(), "editor failed")
	wantContains(t, "error", err.Error(), "vi")
}

func TestEditRelationToMissingIssueIsIgnored(t *testing.T) {
	repo := newCLIRepo(t)
	source := repo.seed(t, &issue.Issue{ID: 1, Title: "Source issue"})

	editTestEditor(t, editTestContent(t, source.FilePath, func(iss *issue.Issue) {
		iss.Relations.Blocks = []int{99}
	}))

	stdout, _, err := repo.run(t, "edit", "1")
	if err != nil {
		t.Fatalf("edit with dangling relation: %v", err)
	}
	wantContains(t, "stdout", stdout, "Updated: #1")
	if got := repo.load(t, 1).Relations.Blocks; !slices.Equal(got, []int{99}) {
		t.Errorf("#1 blocks = %v, want [99] kept as written", got)
	}
}

func TestEditStagesChangedFiles(t *testing.T) {
	tests := []struct {
		name    string
		initial issue.Relations
		mutate  func(*issue.Issue)
	}{
		{"added relation", issue.Relations{}, func(iss *issue.Issue) { iss.Relations.Blocks = []int{2} }},
		{"removed relation", issue.Relations{Blocks: []int{2}}, func(iss *issue.Issue) { iss.Relations.Blocks = nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newCLIRepo(t)
			repo.enableAutoStage(t)
			source := repo.seed(t, &issue.Issue{ID: 1, Title: "Source issue", Relations: tt.initial})
			target := repo.seed(t, &issue.Issue{
				ID:        2,
				Title:     "Target issue",
				Relations: issue.Relations{DependsOn: []int{1}},
			})

			if out, err := exec.Command("git", "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v: %s", err, out)
			}

			editTestEditor(t, editTestContent(t, source.FilePath, tt.mutate))

			stdout, stderr, err := repo.run(t, "edit", "1")
			if err != nil {
				t.Fatalf("edit: %v", err)
			}
			wantContains(t, "stdout", stdout, "Updated: #1")
			wantNotContains(t, "stderr", stderr, "could not stage")

			staged, err := exec.Command("git", "diff", "--cached", "--name-only").Output()
			if err != nil {
				t.Fatalf("git diff --cached: %v", err)
			}
			for _, path := range []string{filepath.Base(source.FilePath), filepath.Base(target.FilePath)} {
				wantContains(t, "staged files", string(staged), path)
			}
		})
	}
}

func TestEditSaveFailureIsReported(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	repo := newCLIRepo(t)
	orig := repo.seed(t, &issue.Issue{ID: 1, Title: "Read only"})

	// The editor writes valid content and then makes the file read-only, so Save
	// fails after validation succeeded.
	payload := filepath.Join(t.TempDir(), "payload.md")
	content := editTestContent(t, orig.FilePath, func(iss *issue.Issue) {
		iss.Title = "Read only edited"
	})
	if err := os.WriteFile(payload, []byte(content), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	script := filepath.Join(t.TempDir(), "editor.sh")
	body := "#!/bin/sh\ncat '" + payload + "' > \"$1\"\nchmod 444 \"$1\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}
	t.Setenv("EDITOR", script)
	t.Cleanup(func() { _ = os.Chmod(orig.FilePath, 0o644) })

	stdout, _, err := repo.run(t, "edit", "1")
	if err == nil {
		t.Fatal("edit succeeded although the issue file is not writable")
	}
	wantContains(t, "error", err.Error(), "permission denied")
	wantNotContains(t, "stdout", stdout, "Updated:")
}

func TestEditRelationSyncFailureIsReported(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}

	tests := []struct {
		name    string
		initial issue.Relations
		mutate  func(*issue.Issue)
	}{
		{"added", issue.Relations{}, func(iss *issue.Issue) { iss.Relations.Blocks = []int{2} }},
		{"removed", issue.Relations{Blocks: []int{2}}, func(iss *issue.Issue) { iss.Relations.Blocks = nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newCLIRepo(t)
			source := repo.seed(t, &issue.Issue{ID: 1, Title: "Source issue", Relations: tt.initial})
			target := repo.seed(t, &issue.Issue{ID: 2, Title: "Target issue"})
			if err := os.Chmod(target.FilePath, 0o444); err != nil {
				t.Fatalf("chmod target: %v", err)
			}
			t.Cleanup(func() { _ = os.Chmod(target.FilePath, 0o644) })

			editTestEditor(t, editTestContent(t, source.FilePath, tt.mutate))

			_, _, err := repo.run(t, "edit", "1")
			if err == nil {
				t.Fatal("edit succeeded although the relation target is not writable")
			}
			wantContains(t, "error", err.Error(), "failed to sync relation with #2")
			wantContains(t, "error", err.Error(), "permission denied")
		})
	}
}

func TestEditRemovesRelationToMissingIssueIsIgnored(t *testing.T) {
	repo := newCLIRepo(t)
	source := repo.seed(t, &issue.Issue{
		ID:        1,
		Title:     "Source issue",
		Relations: issue.Relations{Blocks: []int{99}},
	})

	editTestEditor(t, editTestContent(t, source.FilePath, func(iss *issue.Issue) {
		iss.Relations.Blocks = nil
	}))

	stdout, _, err := repo.run(t, "edit", "1")
	if err != nil {
		t.Fatalf("edit with dangling relation: %v", err)
	}
	wantContains(t, "stdout", stdout, "Updated: #1")
	if got := repo.load(t, 1).Relations.Blocks; len(got) != 0 {
		t.Errorf("#1 blocks = %v, want empty", got)
	}
}

func TestEditInvalidID(t *testing.T) {
	repo := newCLIRepo(t)

	_, _, err := repo.run(t, "edit", "abc")
	if err == nil {
		t.Fatal("edit accepted a non-numeric ID")
	}
	wantContains(t, "error", err.Error(), "invalid issue ID: abc")
}
