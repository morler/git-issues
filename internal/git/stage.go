package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Stage stages an issue file with `git add`. Staging is best effort: a missing
// git repository only produces a warning. A path that does not resolve to a
// direct child of issuesDir is rejected with an error and never passed to git;
// callers currently discard that error, but the file is not touched.
func Stage(issuesDir, filePath string) error {
	abs, err := resolveIssueFile(issuesDir, filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: not staging %q: %v\n", filePath, err)
		return err
	}

	cmd := exec.Command("git", "add", abs)
	cmd.Dir = filepath.Dir(abs)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not stage file (not a git repo?)\n")
		return nil
	}
	return nil
}

// resolveIssueFile resolves both paths and requires the file to be a direct
// child of the issues directory. Both sides are resolved so a symlinked
// .issues/ keeps working while look-alike directories are rejected. Resolution
// failures are fatal for this guard: it must not fall back to an unchecked path.
func resolveIssueFile(issuesDir, filePath string) (string, error) {
	if filePath == "" {
		return "", fmt.Errorf("empty path")
	}

	dir, err := resolveExisting(issuesDir)
	if err != nil {
		return "", fmt.Errorf("issues directory: %w", err)
	}
	abs, err := resolveExisting(filePath)
	if err != nil {
		return "", err
	}
	if filepath.Dir(abs) != dir {
		return "", fmt.Errorf("path is outside %s", issuesDir)
	}
	return abs, nil
}

// resolveExisting returns an absolute path with every symlink resolved. The
// file must exist; there is intentionally no unresolved fallback.
func resolveExisting(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return resolved, nil
}
