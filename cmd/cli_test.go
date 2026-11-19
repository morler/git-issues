package cmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/steviee/git-issues/internal/issue"
)

// TestMain lets tests re-execute the test binary as the issues CLI. That is the
// only way to exercise code paths that call os.Exit (for example check with
// errors), which would otherwise kill the test process.
func TestMain(m *testing.M) {
	if args, ok := os.LookupEnv(cliSubprocessEnv); ok {
		var argv []string
		if args != "" {
			argv = strings.Split(args, "\n")
		}
		rootCmd.SetArgs(argv)
		if err := rootCmd.Execute(); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// cliSubprocessEnv carries newline-separated arguments. os/exec rejects NUL in
// environment values, so arguments cannot be NUL-separated.
const cliSubprocessEnv = "GIT_ISSUES_TEST_CLI_ARGS"

// runCLISubprocess runs the CLI in a child process with dir as working
// directory and the current environment, and reports its exit code.
func runCLISubprocess(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(os.Args[0])
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), cliSubprocessEnv+"="+strings.Join(args, "\n"))

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode = 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run CLI subprocess: %v", err)
		}
		exitCode = exitErr.ExitCode()
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// cliRepo is a temporary directory containing a .issues/ directory. It is the
// working directory for command runs so issues.IssuesDir resolves to it.
type cliRepo struct {
	root      string
	issuesDir string
}

// newCLIRepo creates an initialized repository with auto_stage disabled, so
// command runs never invoke git unless a test opts in via enableAutoStage.
func newCLIRepo(t *testing.T) *cliRepo {
	t.Helper()
	root := t.TempDir()
	issuesDir := filepath.Join(root, ".issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatalf("mkdir .issues: %v", err)
	}
	writeConfig(t, issuesDir, false)
	t.Chdir(root)
	return &cliRepo{root: root, issuesDir: issuesDir}
}

func writeConfig(t *testing.T, issuesDir string, autoStage bool) {
	t.Helper()
	value := "false"
	if autoStage {
		value = "true"
	}
	content := "default_priority: medium\nauto_stage: " + value + "\nlabels:\n  - bug\n  - feature\n"
	if err := os.WriteFile(filepath.Join(issuesDir, ".config.yml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// enableAutoStage turns git staging back on for tests that verify staging.
func (r *cliRepo) enableAutoStage(t *testing.T) {
	t.Helper()
	writeConfig(t, r.issuesDir, true)
}

// seed writes an issue file with the given fields and derives its filename
// from the title, mirroring how the CLI names files.
func (r *cliRepo) seed(t *testing.T, iss *issue.Issue) *issue.Issue {
	t.Helper()
	if iss.Created == "" {
		iss.Created = "2026-01-01"
	}
	if iss.Status == "" {
		iss.Status = "open"
	}
	if iss.Priority == "" {
		iss.Priority = "medium"
	}
	if iss.Labels == nil {
		iss.Labels = []string{}
	}
	if err := issue.Save(r.issuesDir, iss); err != nil {
		t.Fatalf("seed issue #%d: %v", iss.ID, err)
	}
	return iss
}

// load reads an issue back from disk.
func (r *cliRepo) load(t *testing.T, id int) *issue.Issue {
	t.Helper()
	iss, err := issue.LoadByID(r.issuesDir, id)
	if err != nil {
		t.Fatalf("load issue #%d: %v", id, err)
	}
	return iss
}

// run executes the root command with args, capturing stdout and stderr.
// Command flags are reset to their defaults afterwards so tests stay isolated.
func (r *cliRepo) run(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	return runCLI(t, rootCmd, args...)
}

// runCLI executes cmd with args and captures everything written to os.Stdout
// and os.Stderr. All flags in the tree are restored to their defaults after
// the run, because cobra flags are process-wide.
func runCLI(t *testing.T, cmd *cobra.Command, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	resetFlags(cmd)
	t.Cleanup(func() { resetFlags(cmd) })

	outFile, err := os.CreateTemp(t.TempDir(), "stdout-*")
	if err != nil {
		t.Fatalf("temp stdout: %v", err)
	}
	errFile, err := os.CreateTemp(t.TempDir(), "stderr-*")
	if err != nil {
		t.Fatalf("temp stderr: %v", err)
	}

	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outFile, errFile
	defer func() { os.Stdout, os.Stderr = oldStdout, oldStderr }()

	cmd.SetArgs(args)
	runErr := cmd.Execute()

	os.Stdout, os.Stderr = oldStdout, oldStderr
	for _, f := range []*os.File{outFile, errFile} {
		if err := f.Close(); err != nil {
			t.Fatalf("close capture file: %v", err)
		}
	}

	stdout = readFile(t, outFile.Name())
	stderr = readFile(t, errFile.Name())
	return stdout, stderr, runErr
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Slice flags cannot parse their own DefValue ("[]"), so replace them.
		if slice, ok := f.Value.(pflag.SliceValue); ok {
			if err := slice.Replace(nil); err != nil {
				panic("reset slice flag " + f.Name + ": " + err.Error())
			}
		} else if err := f.Value.Set(f.DefValue); err != nil {
			panic("reset flag " + f.Name + ": " + err.Error())
		}
		f.Changed = false
	})
	for _, child := range cmd.Commands() {
		resetFlags(child)
	}
}

// wantContains fails when want is not present in got.
func wantContains(t *testing.T, what, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("%s missing %q, got:\n%s", what, want, got)
	}
}

// wantNotContains fails when want is present in got.
func wantNotContains(t *testing.T, what, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Errorf("%s unexpectedly contains %q, got:\n%s", what, want, got)
	}
}
