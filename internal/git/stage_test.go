package git

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func initRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	root := t.TempDir()
	gitCmd(t, root, "init", "-q")
	return root
}

func writeFile(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("---\nid: 1\n---\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

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

func TestStageStagesFileInsideIssuesDir(t *testing.T) {
	root := initRepo(t)
	issuesDir := filepath.Join(root, ".issues")
	file := writeFile(t, issuesDir, "0001-first.md")

	if err := Stage(issuesDir, file); err != nil {
		t.Fatalf("Stage(%q) = %v, want nil", file, err)
	}
	if got, want := gitCmd(t, root, "status", "--porcelain"), "A  .issues/0001-first.md\n"; got != want {
		t.Errorf("staged files = %q, want %q", got, want)
	}
}

func TestStageAcceptsRelativePath(t *testing.T) {
	root := initRepo(t)
	writeFile(t, filepath.Join(root, ".issues"), "0001-first.md")
	t.Chdir(root)

	if err := Stage(".issues", filepath.Join(".issues", "0001-first.md")); err != nil {
		t.Fatalf("Stage with relative path = %v, want nil", err)
	}
	if got, want := gitCmd(t, root, "status", "--porcelain"), "A  .issues/0001-first.md\n"; got != want {
		t.Errorf("staged files = %q, want %q", got, want)
	}
}

func TestStageAcceptsSymlinkedIssuesDir(t *testing.T) {
	root := initRepo(t)
	realDir := filepath.Join(root, "store")
	writeFile(t, realDir, "0001-first.md")
	issuesDir := filepath.Join(root, ".issues")
	if err := os.Symlink(realDir, issuesDir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := Stage(issuesDir, filepath.Join(issuesDir, "0001-first.md")); err != nil {
		t.Fatalf("Stage through symlinked issues dir = %v, want nil", err)
	}
	if got := gitCmd(t, root, "status", "--porcelain"); got == "" {
		t.Errorf("resolved file was not staged")
	}
}

func TestStageRejectsPathsOutsideIssuesDir(t *testing.T) {
	root := initRepo(t)
	issuesDir := filepath.Join(root, ".issues")
	writeFile(t, root, "stray.md")
	writeFile(t, filepath.Join(root, "issues"), "0002-other.md")
	writeFile(t, issuesDir, "0001-first.md")

	otherRoot := t.TempDir()
	lookAlike := writeFile(t, filepath.Join(otherRoot, ".issues"), "0003-look-alike.md")

	outside := filepath.Join(root, "stray.md")
	link := filepath.Join(issuesDir, "0004-link.md")
	if err := os.Symlink(outside, link); err != nil {
		t.Logf("skipping symlink case: %v", err)
	}

	cases := map[string]string{
		"absolute path outside .issues": outside,
		"path traversal":                filepath.Join(issuesDir, "..", "stray.md"),
		"directory with similar name":   filepath.Join(root, "issues", "0002-other.md"),
		"look-alike .issues elsewhere":  lookAlike,
		"missing file":                  filepath.Join(issuesDir, "9999-missing.md"),
		"empty path":                    "",
	}
	if _, err := os.Lstat(link); err == nil {
		cases["symlink pointing outside"] = link
	}

	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			var err error
			captureStderr(t, func() { err = Stage(issuesDir, path) })
			if err == nil {
				t.Fatalf("Stage(%q) = nil, want error", path)
			}
			if got := gitCmd(t, root, "diff", "--cached", "--name-only"); got != "" {
				t.Errorf("staged files = %q, want none", got)
			}
		})
	}
}

func TestStageRejectsMissingIssuesDir(t *testing.T) {
	root := initRepo(t)
	file := writeFile(t, root, "0001-first.md")

	err := Stage(filepath.Join(root, "missing"), file)
	if err == nil {
		t.Fatalf("Stage with missing issues dir = nil, want error")
	}
}

func TestStageRejectionWarns(t *testing.T) {
	root := t.TempDir()
	issuesDir := filepath.Join(root, ".issues")
	writeFile(t, issuesDir, "0001-first.md")
	outside := writeFile(t, root, "stray.md")

	var err error
	stderr := captureStderr(t, func() { err = Stage(issuesDir, outside) })

	if err == nil {
		t.Fatalf("Stage(%q) = nil, want error", outside)
	}
	for _, want := range []string{"Warning:", "path is outside"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want it to contain %q", stderr, want)
		}
	}
}

func TestStageWarnsButSucceedsOutsideGitRepo(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	issuesDir := filepath.Join(root, ".issues")
	file := writeFile(t, issuesDir, "0001-first.md")

	var err error
	stderr := captureStderr(t, func() { err = Stage(issuesDir, file) })

	if err != nil {
		t.Fatalf("Stage(%q) = %v, want nil", file, err)
	}
	if want := "could not stage file"; !strings.Contains(stderr, want) {
		t.Errorf("stderr = %q, want it to contain %q", stderr, want)
	}
}
