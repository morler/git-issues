package cmd

import (
	"os"
	"testing"

	"github.com/steviee/git-issues/internal/issue"
)

// labelsTestSeed writes one issue per entry so label counts are easy to follow.
func labelsTestSeed(t *testing.T, r *cliRepo, status string, labels ...[]string) {
	t.Helper()
	for i, set := range labels {
		r.seed(t, &issue.Issue{
			ID:     i + 1,
			Title:  "Labeled issue",
			Status: status,
			Labels: set,
		})
	}
}

func TestLabelsCmdSortByCount(t *testing.T) {
	r := newCLIRepo(t)
	labelsTestSeed(t, r, "open",
		[]string{"bug", "auth"},
		[]string{"bug"},
		[]string{"feature", "auth"},
		[]string{"bug", "alpha"},
	)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "default sort",
			args: []string{"labels"},
			want: "bug      3\n" +
				"auth     2\n" +
				"alpha    1\n" +
				"feature  1\n",
		},
		{
			name: "explicit count sort",
			args: []string{"labels", "--sort", "count"},
			want: "bug      3\n" +
				"auth     2\n" +
				"alpha    1\n" +
				"feature  1\n",
		},
		{
			name: "unknown sort falls back to count",
			args: []string{"labels", "--sort", "bogus"},
			want: "bug      3\n" +
				"auth     2\n" +
				"alpha    1\n" +
				"feature  1\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := r.run(t, tt.args...)
			if err != nil {
				t.Fatalf("labels failed: %v (stderr: %q)", err, stderr)
			}
			if stdout != tt.want {
				t.Errorf("stdout = %q, want %q", stdout, tt.want)
			}
		})
	}
}

func TestLabelsCmdSortAlpha(t *testing.T) {
	r := newCLIRepo(t)
	labelsTestSeed(t, r, "open",
		[]string{"bug", "auth"},
		[]string{"bug"},
		[]string{"feature", "auth"},
		[]string{"bug", "alpha"},
	)

	stdout, stderr, err := r.run(t, "labels", "--sort", "alpha")
	if err != nil {
		t.Fatalf("labels --sort alpha failed: %v (stderr: %q)", err, stderr)
	}

	want := "alpha    1\n" +
		"auth     2\n" +
		"bug      3\n" +
		"feature  1\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestLabelsCmdNoLabels(t *testing.T) {
	r := newCLIRepo(t)
	labelsTestSeed(t, r, "open", nil, nil)

	stdout, stderr, err := r.run(t, "labels")
	if err != nil {
		t.Fatalf("labels failed: %v (stderr: %q)", err, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty output", stdout)
	}
}

func TestLabelsCmdAlignsByDisplayWidth(t *testing.T) {
	r := newCLIRepo(t)
	labelsTestSeed(t, r, "open",
		[]string{"bug"},
		[]string{"认证"},
	)

	stdout, stderr, err := r.run(t, "labels")
	if err != nil {
		t.Fatalf("labels failed: %v (stderr: %q)", err, stderr)
	}

	// "认证" is 4 display cells wide, so "bug" is padded to the same width
	// (3 spaces of padding plus the two separator spaces).
	want := "bug   1\n认证  1\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestLabelsCmdCountsClosedIssues(t *testing.T) {
	r := newCLIRepo(t)
	labelsTestSeed(t, r, "closed", []string{"bug"})
	r.seed(t, &issue.Issue{ID: 2, Title: "Open issue", Status: "open", Labels: []string{"bug", "feature"}})
	r.seed(t, &issue.Issue{ID: 3, Title: "Wontfix issue", Status: "wontfix", Labels: []string{"feature"}})

	stdout, stderr, err := r.run(t, "labels")
	if err != nil {
		t.Fatalf("labels failed: %v (stderr: %q)", err, stderr)
	}

	want := "bug      2\n" +
		"feature  2\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestLabelsCmdUnreadableDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}

	r := newCLIRepo(t)
	labelsTestSeed(t, r, "open", []string{"bug"})
	if err := os.Chmod(r.issuesDir, 0o000); err != nil {
		t.Fatalf("chmod .issues: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(r.issuesDir, 0o755); err != nil {
			t.Fatalf("restore .issues permissions: %v", err)
		}
	})

	stdout, _, err := r.run(t, "labels")
	if err == nil {
		t.Fatalf("labels succeeded on an unreadable .issues/, stdout: %q", stdout)
	}
	wantContains(t, "error", err.Error(), "permission denied")
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

func TestLabelsCmdNoIssuesDir(t *testing.T) {
	t.Chdir(t.TempDir())
	if dir, err := issue.IssuesDir(); err == nil {
		t.Skipf("temp dir is inside an initialized repository: %s", dir)
	}

	stdout, _, err := runCLI(t, rootCmd, "labels")
	if err == nil {
		t.Fatalf("labels succeeded without .issues/, stdout: %q", stdout)
	}
	wantContains(t, "error", err.Error(), "no .issues/ directory found (run 'issues init')")
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}
