package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/steviee/git-issues/internal/issue"
)

// newCmdTestConfig rewrites .config.yml with the given default priority.
func newCmdTestConfig(t *testing.T, issuesDir, defaultPriority string) {
	t.Helper()
	content := "default_priority: " + defaultPriority + "\nauto_stage: false\nlabels:\n  - bug\n  - feature\n"
	if err := os.WriteFile(filepath.Join(issuesDir, ".config.yml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// newCmdTestEditorScript writes an executable shell script and returns its path.
func newCmdTestEditorScript(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "editor.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod editor script: %v", err)
	}
	return path
}

// newCmdTestMdFiles returns the issue filenames in issuesDir, sorted by name.
func newCmdTestMdFiles(t *testing.T, issuesDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(issuesDir)
	if err != nil {
		t.Fatalf("read issues dir: %v", err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			names = append(names, entry.Name())
		}
	}
	return names
}

// newCmdTestStubGit puts a failing git stub first in PATH and returns the file it
// logs its arguments to. The stub keeps the auto-stage warning deterministic
// even when TMPDIR happens to live inside a git repository.
func newCmdTestStubGit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "git.log")
	stubPath := filepath.Join(dir, "git")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$GIT_STUB_LOG\"\nexit 1\n"
	if err := os.WriteFile(stubPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write git stub: %v", err)
	}
	if err := os.Chmod(stubPath, 0o755); err != nil {
		t.Fatalf("chmod git stub: %v", err)
	}
	t.Setenv("GIT_STUB_LOG", logPath)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func TestNewCreatesIssueWithTitle(t *testing.T) {
	repo := newCLIRepo(t)

	stdout, stderr, err := repo.run(t, "new", "--title", "Fix login bug")
	if err != nil {
		t.Fatalf("new --title: %v", err)
	}
	wantContains(t, "stdout", stdout, "Created: .issues/0001-fix-login-bug.md (#1)")
	wantNotContains(t, "stderr", stderr, "Warning")

	if files := newCmdTestMdFiles(t, repo.issuesDir); !slices.Equal(files, []string{"0001-fix-login-bug.md"}) {
		t.Fatalf("issue files = %v, want [0001-fix-login-bug.md]", files)
	}

	iss := repo.load(t, 1)
	if iss.ID != 1 {
		t.Errorf("id = %d, want 1", iss.ID)
	}
	if iss.Title != "Fix login bug" {
		t.Errorf("title = %q, want %q", iss.Title, "Fix login bug")
	}
	if iss.Status != "open" {
		t.Errorf("status = %q, want open", iss.Status)
	}
	if iss.Priority != "medium" {
		t.Errorf("priority = %q, want medium (config default)", iss.Priority)
	}
	if len(iss.Labels) != 0 {
		t.Errorf("labels = %v, want empty", iss.Labels)
	}
	if iss.Body != "" {
		t.Errorf("body = %q, want empty", iss.Body)
	}
	if iss.Created != issue.Today() || iss.Updated != issue.Today() {
		t.Errorf("created/updated = %q/%q, want %q", iss.Created, iss.Updated, issue.Today())
	}

	raw := readFile(t, filepath.Join(repo.issuesDir, "0001-fix-login-bug.md"))
	if !strings.HasPrefix(raw, "---\n") {
		t.Errorf("file does not start with frontmatter delimiter:\n%s", raw)
	}
	for _, want := range []string{"id: 1\n", "title: Fix login bug", "status: open", "priority: medium", "created:", issue.Today()} {
		wantContains(t, "frontmatter", raw, want)
	}

	stdout, _, err = repo.run(t, "new", "--title", "Second issue")
	if err != nil {
		t.Fatalf("second new --title: %v", err)
	}
	wantContains(t, "stdout", stdout, "Created: .issues/0002-second-issue.md (#2)")

	wantFiles := []string{"0001-fix-login-bug.md", "0002-second-issue.md"}
	if files := newCmdTestMdFiles(t, repo.issuesDir); !slices.Equal(files, wantFiles) {
		t.Fatalf("issue files = %v, want %v", files, wantFiles)
	}
	if iss := repo.load(t, 2); iss.ID != 2 {
		t.Errorf("second issue id = %d, want 2", iss.ID)
	}
}

func TestNewPriority(t *testing.T) {
	tests := []struct {
		name           string
		configPriority string
		extraArgs      []string
		want           string
	}{
		{name: "config default priority", configPriority: "critical", want: "critical"},
		{name: "explicit flag overrides config", configPriority: "critical", extraArgs: []string{"--priority", "low"}, want: "low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newCLIRepo(t)
			newCmdTestConfig(t, repo.issuesDir, tt.configPriority)

			args := append([]string{"new", "--title", "Priority issue"}, tt.extraArgs...)
			if _, _, err := repo.run(t, args...); err != nil {
				t.Fatalf("new: %v", err)
			}

			if got := repo.load(t, 1).Priority; got != tt.want {
				t.Errorf("priority = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewLabelsAndBody(t *testing.T) {
	repo := newCLIRepo(t)

	_, _, err := repo.run(t, "new", "--title", "Documented bug",
		"--label", "bug", "--label", "auth",
		"--body", "Line one\nLine two")
	if err != nil {
		t.Fatalf("new with label and body: %v", err)
	}

	iss := repo.load(t, 1)
	if want := []string{"bug", "auth"}; !slices.Equal(iss.Labels, want) {
		t.Errorf("labels = %v, want %v", iss.Labels, want)
	}
	if want := "Line one\nLine two"; iss.Body != want {
		t.Errorf("body = %q, want %q", iss.Body, want)
	}

	raw := readFile(t, filepath.Join(repo.issuesDir, "0001-documented-bug.md"))
	wantContains(t, "body", raw, "Line one\nLine two")
}

func TestNewRelationsAreBidirectional(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Blocked target"})
	repo.seed(t, &issue.Issue{ID: 2, Title: "Dependency target"})
	repo.seed(t, &issue.Issue{ID: 3, Title: "Related target"})

	stdout, _, err := repo.run(t, "new", "--title", "With relations",
		"--blocks", "1", "--depends-on", "2", "--related-to", "3")
	if err != nil {
		t.Fatalf("new with relations: %v", err)
	}
	wantContains(t, "stdout", stdout, "Created: .issues/0004-with-relations.md (#4)")

	source := repo.load(t, 4)
	if want := []int{1}; !slices.Equal(source.Relations.Blocks, want) {
		t.Errorf("#4 blocks = %v, want %v", source.Relations.Blocks, want)
	}
	if want := []int{2}; !slices.Equal(source.Relations.DependsOn, want) {
		t.Errorf("#4 depends-on = %v, want %v", source.Relations.DependsOn, want)
	}
	if want := []int{3}; !slices.Equal(source.Relations.RelatedTo, want) {
		t.Errorf("#4 related-to = %v, want %v", source.Relations.RelatedTo, want)
	}

	blocked := repo.load(t, 1)
	if want := []int{4}; !slices.Equal(blocked.Relations.DependsOn, want) {
		t.Errorf("#1 depends-on = %v, want %v", blocked.Relations.DependsOn, want)
	}
	if got := blocked.Relations.Blocks; len(got) != 0 {
		t.Errorf("#1 blocks = %v, want none", got)
	}
	if want := []int{4}; !slices.Equal(repo.load(t, 2).Relations.Blocks, want) {
		t.Errorf("#2 blocks = %v, want %v", repo.load(t, 2).Relations.Blocks, want)
	}
	if want := []int{4}; !slices.Equal(repo.load(t, 3).Relations.RelatedTo, want) {
		t.Errorf("#3 related-to = %v, want %v", repo.load(t, 3).Relations.RelatedTo, want)
	}
}

func TestNewUnknownRelationTarget(t *testing.T) {
	tests := []struct {
		name string
		flag string
	}{
		{name: "blocks", flag: "--blocks"},
		{name: "depends-on", flag: "--depends-on"},
		{name: "related-to", flag: "--related-to"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newCLIRepo(t)
			repo.seed(t, &issue.Issue{ID: 1, Title: "Existing issue"})

			stdout, _, err := repo.run(t, "new", "--title", "Broken relation", tt.flag, "99")
			if err == nil {
				t.Fatal("expected new to fail for a missing relation target")
			}
			wantContains(t, "error", err.Error(), "issue #99 not found")
			wantNotContains(t, "stdout", stdout, "Created:")

			if files := newCmdTestMdFiles(t, repo.issuesDir); !slices.Equal(files, []string{"0001-existing-issue.md"}) {
				t.Errorf("issue files = %v, want only 0001-existing-issue.md", files)
			}
			if _, err := issue.LoadByID(repo.issuesDir, 2); err == nil {
				t.Error("issue #2 was created despite the missing relation target")
			}
		})
	}
}

func TestNewEditorCreatesIssue(t *testing.T) {
	repo := newCLIRepo(t)
	repo.seed(t, &issue.Issue{ID: 1, Title: "Blocked by editor issue"})

	editor := newCmdTestEditorScript(t, `#!/bin/sh
cat > "$1" <<'EOF'
---
title: "Editor issue"
priority: high
labels: [bug]
relations:
  blocks: [1]
  depends-on: []
---

Body from editor.
EOF
`)
	t.Setenv("EDITOR", editor)

	stdout, stderr, err := repo.run(t, "new")
	if err != nil {
		t.Fatalf("new via editor: %v", err)
	}
	wantContains(t, "stdout", stdout, "Created: .issues/0002-editor-issue.md (#2)")
	wantNotContains(t, "stderr", stderr, "Warning")

	iss := repo.load(t, 2)
	if iss.Title != "Editor issue" {
		t.Errorf("title = %q, want %q", iss.Title, "Editor issue")
	}
	if iss.Priority != "high" {
		t.Errorf("priority = %q, want high", iss.Priority)
	}
	if want := []string{"bug"}; !slices.Equal(iss.Labels, want) {
		t.Errorf("labels = %v, want %v", iss.Labels, want)
	}
	if want := "Body from editor."; iss.Body != want {
		t.Errorf("body = %q, want %q", iss.Body, want)
	}
	if want := []int{1}; !slices.Equal(iss.Relations.Blocks, want) {
		t.Errorf("#2 blocks = %v, want %v", iss.Relations.Blocks, want)
	}
	if want := []int{2}; !slices.Equal(repo.load(t, 1).Relations.DependsOn, want) {
		t.Errorf("#1 depends-on = %v, want %v", repo.load(t, 1).Relations.DependsOn, want)
	}
}

func TestNewEditorEmptyTitleAborts(t *testing.T) {
	repo := newCLIRepo(t)

	editor := newCmdTestEditorScript(t, `#!/bin/sh
cat > "$1" <<'EOF'
---
title: ""
priority: medium
labels: []
relations:
  blocks: []
  depends-on: []
---

<!-- Describe the issue here. Delete this line. -->
EOF
`)
	t.Setenv("EDITOR", editor)

	stdout, stderr, err := repo.run(t, "new")
	if err != nil {
		t.Fatalf("new via editor with empty title: %v", err)
	}
	wantContains(t, "stderr", stderr, "Aborted.")
	wantNotContains(t, "stdout", stdout, "Created:")

	if files := newCmdTestMdFiles(t, repo.issuesDir); len(files) != 0 {
		t.Errorf("issue files = %v, want none", files)
	}
	next, err := issue.NextID(repo.issuesDir)
	if err != nil {
		t.Fatalf("next id: %v", err)
	}
	if next != 1 {
		t.Errorf("next id = %d, want 1", next)
	}
}

func TestNewEditorFailure(t *testing.T) {
	repo := newCLIRepo(t)

	if _, err := os.Stat("/bin/false"); err != nil {
		t.Skipf("/bin/false not available: %v", err)
	}
	t.Setenv("EDITOR", "/bin/false")

	_, _, err := repo.run(t, "new")
	if err == nil {
		t.Fatal("expected new to fail when the editor exits non-zero")
	}
	wantContains(t, "error", err.Error(), "editor failed")

	if files := newCmdTestMdFiles(t, repo.issuesDir); len(files) != 0 {
		t.Errorf("issue files = %v, want none", files)
	}
}

func TestNewEditorInvalidFile(t *testing.T) {
	repo := newCLIRepo(t)

	editor := newCmdTestEditorScript(t, "#!/bin/sh\nprintf 'not an issue file\\n' > \"$1\"\n")
	t.Setenv("EDITOR", editor)

	_, _, err := repo.run(t, "new")
	if err == nil {
		t.Fatal("expected new to fail for an unparsable editor file")
	}
	wantContains(t, "error", err.Error(), "file must start with ---")

	if files := newCmdTestMdFiles(t, repo.issuesDir); len(files) != 0 {
		t.Errorf("issue files = %v, want none", files)
	}
}

func TestNewEditorWithoutPriorityUsesConfigDefault(t *testing.T) {
	repo := newCLIRepo(t)
	newCmdTestConfig(t, repo.issuesDir, "high")

	editor := newCmdTestEditorScript(t, `#!/bin/sh
cat > "$1" <<'EOF'
---
title: "No priority"
labels: []
---

No priority in frontmatter.
EOF
`)
	t.Setenv("EDITOR", editor)

	if _, _, err := repo.run(t, "new"); err != nil {
		t.Fatalf("new via editor without priority: %v", err)
	}

	iss := repo.load(t, 1)
	if iss.Priority != "high" {
		t.Errorf("priority = %q, want high (config default)", iss.Priority)
	}
}

func TestNewInvalidPriorityCreatesNothing(t *testing.T) {
	repo := newCLIRepo(t)

	_, _, err := repo.run(t, "new", "--title", "Invalid priority", "--priority", "urgent")
	if err == nil {
		t.Fatal("expected new to reject an invalid priority")
	}
	wantContains(t, "error", err.Error(), `invalid priority "urgent"`)

	if files := newCmdTestMdFiles(t, repo.issuesDir); len(files) != 0 {
		t.Errorf("issue files = %v, want none", files)
	}
}

func TestNewDefaultEditor(t *testing.T) {
	repo := newCLIRepo(t)

	editorDir := t.TempDir()
	viPath := filepath.Join(editorDir, "vi")
	if err := os.WriteFile(viPath, []byte(`#!/bin/sh
cat > "$1" <<'EOF'
---
title: "Default editor"
---

Written by vi.
EOF
`), 0o755); err != nil {
		t.Fatalf("write vi stub: %v", err)
	}
	if err := os.Chmod(viPath, 0o755); err != nil {
		t.Fatalf("chmod vi stub: %v", err)
	}
	t.Setenv("EDITOR", "")
	t.Setenv("PATH", editorDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	stdout, _, err := repo.run(t, "new")
	if err != nil {
		t.Fatalf("new via default editor: %v", err)
	}
	wantContains(t, "stdout", stdout, "Created: .issues/0001-default-editor.md (#1)")
	if got := repo.load(t, 1).Title; got != "Default editor" {
		t.Errorf("title = %q, want %q", got, "Default editor")
	}
}

func TestNewEditorTempFileFailure(t *testing.T) {
	repo := newCLIRepo(t)
	t.Setenv("EDITOR", newCmdTestEditorScript(t, "#!/bin/sh\nexit 0\n"))
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))

	_, _, err := repo.run(t, "new")
	if err == nil {
		t.Fatal("expected new to fail when the temp file cannot be created")
	}
	wantContains(t, "error", err.Error(), "no such file or directory")

	if files := newCmdTestMdFiles(t, repo.issuesDir); len(files) != 0 {
		t.Errorf("issue files = %v, want none", files)
	}
}

func TestNewRelationSyncFailure(t *testing.T) {
	const invalidTarget = `---
id: 1
title: Broken target
status: broken
priority: medium
labels: []
created: 2026-01-01
---
`

	for _, flag := range []string{"--blocks", "--depends-on", "--related-to"} {
		t.Run(flag, func(t *testing.T) {
			repo := newCLIRepo(t)
			path := filepath.Join(repo.issuesDir, "0001-broken-target.md")
			if err := os.WriteFile(path, []byte(invalidTarget), 0o644); err != nil {
				t.Fatalf("write invalid target: %v", err)
			}

			stdout, _, err := repo.run(t, "new", "--title", "Relation sync", flag, "1")
			if err == nil {
				t.Fatal("expected new to fail when the relation cannot be written")
			}
			wantContains(t, "error", err.Error(), "issue #2 created, but could not add")
			wantContains(t, "error", err.Error(), "to #1")
			wantContains(t, "error", err.Error(), "invalid status")
			wantNotContains(t, "stdout", stdout, "Created:")

			// The new issue is written before its relations are synced.
			created := repo.load(t, 2)
			if created.Title != "Relation sync" {
				t.Errorf("title = %q, want %q", created.Title, "Relation sync")
			}
			if got := created.Relations.Blocks; len(got) != 0 {
				t.Errorf("#2 blocks = %v, want none", got)
			}
			if got := created.Relations.DependsOn; len(got) != 0 {
				t.Errorf("#2 depends-on = %v, want none", got)
			}
			if got := repo.load(t, 2).Relations.RelatedTo; len(got) != 0 {
				t.Errorf("#2 related-to = %v, want none", got)
			}
			if got := readFile(t, path); got != invalidTarget {
				t.Errorf("target file changed after the failed relation write:\n%s", got)
			}
		})
	}
}

func TestNewWithoutIssuesDir(t *testing.T) {
	t.Chdir(t.TempDir())

	_, _, err := runCLI(t, rootCmd, "new", "--title", "No repo")
	if err == nil {
		t.Fatal("expected new to fail outside an initialized repository")
	}
	wantContains(t, "error", err.Error(), "no .issues/ directory found")
}

func TestNewWithBrokenConfig(t *testing.T) {
	repo := newCLIRepo(t)
	if err := os.WriteFile(filepath.Join(repo.issuesDir, ".config.yml"), []byte("default_priority: [oops\n"), 0o644); err != nil {
		t.Fatalf("write broken config: %v", err)
	}

	_, _, err := repo.run(t, "new", "--title", "Broken config")
	if err == nil {
		t.Fatal("expected new to fail when .config.yml is invalid")
	}
	wantContains(t, "error", err.Error(), "yaml")

	if files := newCmdTestMdFiles(t, repo.issuesDir); len(files) != 0 {
		t.Errorf("issue files = %v, want none", files)
	}
}

func TestNewAutoStageWarnsWhenGitFails(t *testing.T) {
	repo := newCLIRepo(t)
	repo.enableAutoStage(t)
	gitLog := newCmdTestStubGit(t)

	stdout, stderr, err := repo.run(t, "new", "--title", "Auto stage issue")
	if err != nil {
		t.Fatalf("new with auto_stage: %v", err)
	}
	wantContains(t, "stdout", stdout, "Created: .issues/0001-auto-stage-issue.md (#1)")
	wantContains(t, "stderr", stderr, "Warning: could not stage file (not a git repo?)")

	log := readFile(t, gitLog)
	wantContains(t, "git args", log, "add")
	wantContains(t, "git args", log, "0001-auto-stage-issue.md")

	if iss := repo.load(t, 1); iss.Title != "Auto stage issue" {
		t.Errorf("title = %q, want %q", iss.Title, "Auto stage issue")
	}
}

func TestNewParseID(t *testing.T) {
	id, err := parseID("42")
	if err != nil {
		t.Fatalf("parseID(42): %v", err)
	}
	if id != 42 {
		t.Errorf("parseID(42) = %d, want 42", id)
	}

	_, err = parseID("abc")
	if err == nil {
		t.Fatal("expected parseID to reject a non-numeric ID")
	}
	wantContains(t, "error", err.Error(), "invalid issue ID: abc")
}

func TestNewUnreadableIssuesDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}

	repo := newCLIRepo(t)
	if err := os.Chmod(repo.issuesDir, 0o111); err != nil {
		t.Fatalf("chmod .issues: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(repo.issuesDir, 0o755) })

	_, _, err := repo.run(t, "new", "--title", "Unreadable dir")
	if err == nil {
		t.Fatal("expected new to fail when .issues/ cannot be read")
	}
	wantContains(t, "error", err.Error(), "permission denied")
}
