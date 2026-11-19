package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoardRequiresIssuesDir(t *testing.T) {
	t.Chdir(t.TempDir())

	stdout, stderr, err := runCLI(t, rootCmd, "board")
	if err == nil {
		t.Fatal("board outside an issues repository returned no error")
	}
	wantContains(t, "board error", err.Error(), "no .issues/ directory found")
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("board wrote unexpected stdout: %q", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Errorf("board wrote unexpected stderr: %q", stderr)
	}
}

func TestBoardUnreadableIssuesDir(t *testing.T) {
	r := newCLIRepo(t)
	r.seed(t, testIssue(1, "open", "high", []string{"bug"}, nil, nil))
	// Execute-only directory: the config file can still be opened by name, but
	// listing the issues directory fails, so board must error before the TUI.
	if err := os.Chmod(r.issuesDir, 0o111); err != nil {
		t.Fatalf("chmod .issues: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(r.issuesDir, 0o755) })
	if _, err := os.ReadDir(r.issuesDir); err == nil {
		t.Skip("directory permissions are not enforced for this user")
	}

	argSets := map[string][]string{
		"without filters": {"board"},
		"with priority and label filters": {"board",
			"--priority", "high", "--label", "bug"},
	}
	for name, args := range argSets {
		t.Run(name, func(t *testing.T) {
			stdout, _, err := r.run(t, args...)
			if err == nil {
				t.Fatal("board with an unlistable .issues/ returned no error")
			}
			wantContains(t, "board error", err.Error(), "permission denied")
			wantNotContains(t, "board error", err.Error(), "error running board")
			if strings.TrimSpace(stdout) != "" {
				t.Errorf("board wrote unexpected stdout: %q", stdout)
			}
		})
	}
}

func TestBoardInvalidConfig(t *testing.T) {
	r := newCLIRepo(t)
	if err := os.WriteFile(filepath.Join(r.issuesDir, ".config.yml"), []byte("auto_stage: [broken\n"), 0o644); err != nil {
		t.Fatalf("write broken config: %v", err)
	}

	stdout, _, err := r.run(t, "board")
	if err == nil {
		t.Fatal("board with a broken config returned no error")
	}
	wantContains(t, "board error", err.Error(), "yaml")
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("board wrote unexpected stdout: %q", stdout)
	}
}
