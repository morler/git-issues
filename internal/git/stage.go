package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Stage stages an issue file with `git add`. Staging is best effort: a missing
// git repository only produces a warning and never fails the calling command.
// The path is resolved and must point at a file directly inside a .issues/
// directory, so no caller input can stage arbitrary files.
func Stage(filePath string) error {
	abs, err := issueFilePath(filePath)
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

// issueFilePath makes filePath absolute, resolves symlinks and rejects
// anything that is not a direct child of a .issues/ directory.
func issueFilePath(filePath string) (string, error) {
	if filePath == "" {
		return "", fmt.Errorf("empty path")
	}

	abs, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}

	if filepath.Base(filepath.Dir(abs)) != ".issues" {
		return "", fmt.Errorf("path is outside .issues/")
	}
	return abs, nil
}
