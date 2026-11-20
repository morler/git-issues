package issue

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssuesDir(t *testing.T) {
	// Create a temp dir with .issues/ subdirectory
	root := t.TempDir()
	issuesDir := filepath.Join(root, ".issues")
	if err := os.MkdirAll(issuesDir, 0755); err != nil {
		t.Fatalf("mkdir .issues: %v", err)
	}

	// Create a nested subdirectory to test walking up
	nested := filepath.Join(root, "src", "pkg")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	// Save original dir
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("chdir back: %v", err)
		}
	}()

	// Change to nested dir and try to find .issues/
	if err := os.Chdir(nested); err != nil {
		t.Fatalf("chdir nested: %v", err)
	}
	found, err := IssuesDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Resolve symlinks for macOS (/var → /private/var)
	expectedResolved, _ := filepath.EvalSymlinks(issuesDir)
	foundResolved, _ := filepath.EvalSymlinks(found)
	if foundResolved != expectedResolved {
		t.Errorf("expected %q, got %q", expectedResolved, foundResolved)
	}
}

func TestIssuesDirNotFound(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("chdir back: %v", err)
		}
	}()

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	_, err = IssuesDir()
	if err == nil {
		t.Error("expected error when .issues/ not found")
	}
}

func TestLoadAll(t *testing.T) {
	dir := t.TempDir()
	writeIssueFile(t, dir, 1, "First issue")
	writeIssueFile(t, dir, 2, "Second issue")

	issues, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(issues))
	}
}

func TestLoadAllSkipsDotFiles(t *testing.T) {
	dir := t.TempDir()
	writeIssueFile(t, dir, 1, "Normal issue")
	// Write a dotfile that looks like an issue
	if err := os.WriteFile(filepath.Join(dir, ".agent.md"), []byte("---\nid: 99\ntitle: Agent\nstatus: open\npriority: medium\nlabels: []\ncreated: 2026-01-01\nupdated: 2026-01-01\n---\n"), 0644); err != nil {
		t.Fatalf("write dotfile: %v", err)
	}

	issues, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue (dotfile skipped), got %d", len(issues))
	}
}

func TestLoadByID(t *testing.T) {
	dir := t.TempDir()
	writeIssueFile(t, dir, 1, "First")
	writeIssueFile(t, dir, 2, "Second")

	iss, err := LoadByID(dir, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if iss.Title != "Second" {
		t.Errorf("expected title 'Second', got %q", iss.Title)
	}
}

func TestLoadByIDNotFound(t *testing.T) {
	dir := t.TempDir()
	writeIssueFile(t, dir, 1, "First")

	_, err := LoadByID(dir, 99)
	if err == nil {
		t.Error("expected error for non-existent ID")
	}
}

func TestSave(t *testing.T) {
	dir := t.TempDir()
	iss := &Issue{
		ID:       1,
		Title:    "Test issue",
		Status:   "open",
		Priority: "medium",
		Labels:   []string{},
		Created:  "2026-01-01",
		Body:     "Test body",
	}

	err := Save(dir, iss)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Reload and verify
	loaded, err := LoadByID(dir, 1)
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if loaded.Title != "Test issue" {
		t.Errorf("expected title 'Test issue', got %q", loaded.Title)
	}
	if loaded.Updated == "" {
		t.Error("expected updated field to be set")
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	writeIssueFile(t, dir, 1, "To delete")

	err := Delete(dir, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = LoadByID(dir, 1)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

// captureStderr captures everything fn writes to os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stderr
	os.Stderr = w
	defer func() {
		os.Stderr = old
		_ = w.Close()
	}()

	fn()

	os.Stderr = old
	if err := w.Close(); err != nil {
		t.Fatalf("close stderr pipe: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return string(data)
}

func TestScanIssuesSeparatesBrokenFiles(t *testing.T) {
	dir := t.TempDir()
	writeIssueFile(t, dir, 1, "Valid issue")
	if err := os.WriteFile(filepath.Join(dir, "0002-broken.md"), []byte("garbage\n"), 0o644); err != nil {
		t.Fatalf("write broken file: %v", err)
	}
	// None of these count as issue files.
	if err := os.WriteFile(filepath.Join(dir, ".agent.md"), []byte("garbage\n"), 0o644); err != nil {
		t.Fatalf("write dotfile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("notes\n"), 0o644); err != nil {
		t.Fatalf("write notes: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "nested.md"), 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	scan, err := scanIssues(dir)
	if err != nil {
		t.Fatalf("scanIssues: %v", err)
	}
	if len(scan.names) != 2 {
		t.Errorf("names = %v, want 2 issue files", scan.names)
	}
	if len(scan.issues) != 1 || scan.issues[0].ID != 1 {
		t.Errorf("issues = %v, want only #1", scan.issues)
	}
	if len(scan.broken) != 1 || filepath.Base(scan.broken[0].Path) != "0002-broken.md" {
		t.Errorf("broken = %v, want 0002-broken.md", scan.broken)
	}
}

func TestLoadAllWarnsOnBrokenFiles(t *testing.T) {
	dir := t.TempDir()
	writeIssueFile(t, dir, 1, "Valid issue")
	if err := os.WriteFile(filepath.Join(dir, "0002-broken.md"), []byte("garbage\n"), 0o644); err != nil {
		t.Fatalf("write broken file: %v", err)
	}

	var issues []*Issue
	var err error
	stderr := captureStderr(t, func() { issues, err = LoadAll(dir) })
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(issues) != 1 || issues[0].ID != 1 {
		t.Errorf("issues = %v, want only #1", issues)
	}
	for _, want := range []string{"Warning: skipping issue file 0002-broken.md", "file must start with ---"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want it to contain %q", stderr, want)
		}
	}
}

func TestLoadWithBrokenReturnsUnparsableFiles(t *testing.T) {
	dir := t.TempDir()
	writeIssueFile(t, dir, 1, "Valid issue")
	if err := os.WriteFile(filepath.Join(dir, "0002-broken.md"), []byte("garbage\n"), 0o644); err != nil {
		t.Fatalf("write broken file: %v", err)
	}

	issues, broken, err := LoadWithBroken(dir)
	if err != nil {
		t.Fatalf("LoadWithBroken: %v", err)
	}
	if len(issues) != 1 || issues[0].ID != 1 {
		t.Errorf("issues = %v, want only #1", issues)
	}
	if len(broken) != 1 {
		t.Fatalf("broken = %v, want one file", broken)
	}
	if filepath.Base(broken[0].Path) != "0002-broken.md" || broken[0].Err == nil {
		t.Errorf("broken entry = %+v, want 0002-broken.md with a parse error", broken[0])
	}
}

func writeIssueFile(t *testing.T, dir string, id int, title string) {
	t.Helper()
	content := "---\n" +
		"id: " + fmt.Sprintf("%d", id) + "\n" +
		"title: \"" + title + "\"\n" +
		"status: open\n" +
		"priority: medium\n" +
		"labels: []\n" +
		"created: 2026-01-01\n" +
		"updated: 2026-01-01\n" +
		"---\n"
	slug := GenerateSlug(title)
	filename := Filename(id, slug)
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
