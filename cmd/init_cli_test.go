package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/steviee/git-issues/internal/config"
)

func TestInitCreatesSkeleton(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	stdout, _, err := runCLI(t, rootCmd, "init")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	wantContains(t, "stdout", stdout, "Initialized git-issues in .issues/")

	issuesDir := filepath.Join(root, ".issues")

	configPath := filepath.Join(issuesDir, ".config.yml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf(".config.yml missing: %v", err)
	}
	cfg, err := config.Load(issuesDir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DefaultPriority != "medium" {
		t.Errorf("default_priority = %q, want medium", cfg.DefaultPriority)
	}
	if !cfg.AutoStage {
		t.Error("auto_stage = false, want true")
	}
	wantLabels := config.Default().Labels
	if len(cfg.Labels) != len(wantLabels) {
		t.Fatalf("labels = %v, want %v", cfg.Labels, wantLabels)
	}
	for i, label := range wantLabels {
		if cfg.Labels[i] != label {
			t.Errorf("labels = %v, want %v", cfg.Labels, wantLabels)
			break
		}
	}

	agentPath := filepath.Join(issuesDir, ".agent.md")
	agent, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf(".agent.md missing: %v", err)
	}
	wantContains(t, ".agent.md", string(agent), "# git-issues agent context")
	wantContains(t, ".agent.md", string(agent), "issues next")

	gitignorePath := filepath.Join(issuesDir, ".gitignore")
	info, err := os.Stat(gitignorePath)
	if err != nil {
		t.Fatalf(".gitignore missing: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf(".gitignore size = %d, want 0", info.Size())
	}
}

func TestInitRejectsExistingDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	if _, _, err := runCLI(t, rootCmd, "init"); err != nil {
		t.Fatalf("first init: %v", err)
	}
	configPath := filepath.Join(root, ".issues", ".config.yml")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	stdout, _, err := runCLI(t, rootCmd, "init")
	if err == nil {
		t.Fatal("second init succeeded, want .issues/ already exists error")
	}
	wantContains(t, "error", err.Error(), ".issues/ already exists")
	wantNotContains(t, "stdout", stdout, "Initialized")

	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after failed init: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("config rewritten by failed init:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestInitReportsMkdirFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	root := t.TempDir()
	t.Chdir(root)
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatalf("chmod root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })

	stdout, _, err := runCLI(t, rootCmd, "init")
	if err == nil {
		t.Fatal("init succeeded in a read-only directory")
	}
	wantContains(t, "error", err.Error(), "permission denied")
	wantNotContains(t, "stdout", stdout, "Initialized")
	if _, statErr := os.Stat(filepath.Join(root, ".issues", ".config.yml")); !os.IsNotExist(statErr) {
		t.Errorf(".config.yml was written despite mkdir failure: %v", statErr)
	}
}
