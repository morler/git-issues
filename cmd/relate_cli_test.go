package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/steviee/git-issues/internal/config"
	"github.com/steviee/git-issues/internal/issue"
)

// relateTestPair seeds a source issue #1 and a target issue #2 without relations.
func relateTestPair(t *testing.T, r *cliRepo) {
	t.Helper()
	r.seed(t, &issue.Issue{ID: 1, Title: "Source issue"})
	r.seed(t, &issue.Issue{ID: 2, Title: "Target issue"})
}

// relateTestField returns the relation slice of iss that belongs to relation.
func relateTestField(t *testing.T, iss *issue.Issue, relation string) []int {
	t.Helper()
	switch relation {
	case "blocks":
		return iss.Relations.Blocks
	case "depends-on":
		return iss.Relations.DependsOn
	case "related-to":
		return iss.Relations.RelatedTo
	case "duplicates":
		return iss.Relations.Duplicates
	default:
		t.Fatalf("unknown relation %q", relation)
		return nil
	}
}

// relateTestEntryCount counts YAML list entries like "- 2" in an issue file.
func relateTestEntryCount(t *testing.T, path, entry string) int {
	t.Helper()
	count := 0
	for _, line := range strings.Split(readFile(t, path), "\n") {
		if strings.TrimSpace(line) == entry {
			count++
		}
	}
	return count
}

// relateTestTotalRelations reports how many relation entries iss carries.
func relateTestTotalRelations(iss *issue.Issue) int {
	return len(iss.Relations.Blocks) + len(iss.Relations.DependsOn) +
		len(iss.Relations.RelatedTo) + len(iss.Relations.Duplicates)
}

// relateTestBreakConfig writes invalid YAML to .config.yml so config.Load fails.
func relateTestBreakConfig(t *testing.T, r *cliRepo) {
	t.Helper()
	path := filepath.Join(r.issuesDir, ".config.yml")
	if err := os.WriteFile(path, []byte("invalid: [\n"), 0o644); err != nil {
		t.Fatalf("write broken config: %v", err)
	}
}

func TestRelateCmdWritesBothSides(t *testing.T) {
	tests := []struct {
		relation    string
		sourceField string
		targetField string
	}{
		{"blocks", "blocks", "depends-on"},
		{"depends-on", "depends-on", "blocks"},
		{"related-to", "related-to", "related-to"},
		{"duplicates", "duplicates", "duplicates"},
	}

	for _, tt := range tests {
		t.Run(tt.relation, func(t *testing.T) {
			r := newCLIRepo(t)
			relateTestPair(t, r)

			stdout, stderr, err := r.run(t, "relate", "1", tt.relation, "2")
			if err != nil {
				t.Fatalf("relate failed: %v (stderr: %q)", err, stderr)
			}

			want := "Related: #1 " + tt.relation + " #2\n" +
				"         #2 " + issue.Inverse(tt.relation) + " #1 (auto)\n"
			if stdout != want {
				t.Errorf("stdout = %q, want %q", stdout, want)
			}

			source := r.load(t, 1)
			target := r.load(t, 2)

			if got := relateTestField(t, source, tt.sourceField); !slices.Equal(got, []int{2}) {
				t.Errorf("source %s = %v, want [2]", tt.sourceField, got)
			}
			if got := relateTestField(t, target, tt.targetField); !slices.Equal(got, []int{1}) {
				t.Errorf("target %s = %v, want [1]", tt.targetField, got)
			}
			if got := relateTestTotalRelations(source); got != 1 {
				t.Errorf("source carries %d relation entries, want 1", got)
			}
			if got := relateTestTotalRelations(target); got != 1 {
				t.Errorf("target carries %d relation entries, want 1", got)
			}
			if n := relateTestEntryCount(t, source.FilePath, "- 2"); n != 1 {
				t.Errorf("source file has %d entries for #2, want 1", n)
			}
			if n := relateTestEntryCount(t, target.FilePath, "- 1"); n != 1 {
				t.Errorf("target file has %d entries for #1, want 1", n)
			}
		})
	}
}

func TestRelateCmdDuplicateIsIdempotent(t *testing.T) {
	r := newCLIRepo(t)
	relateTestPair(t, r)

	for i := 0; i < 2; i++ {
		stdout, stderr, err := r.run(t, "relate", "1", "related-to", "2")
		if err != nil {
			t.Fatalf("relate run %d failed: %v (stderr: %q)", i+1, err, stderr)
		}
		wantContains(t, "stdout", stdout, "Related: #1 related-to #2")
	}

	source := r.load(t, 1)
	target := r.load(t, 2)
	if got := source.Relations.RelatedTo; !slices.Equal(got, []int{2}) {
		t.Errorf("source related-to = %v, want [2]", got)
	}
	if got := target.Relations.RelatedTo; !slices.Equal(got, []int{1}) {
		t.Errorf("target related-to = %v, want [1]", got)
	}
	if n := relateTestEntryCount(t, source.FilePath, "- 2"); n != 1 {
		t.Errorf("source file has %d entries for #2, want 1", n)
	}
	if n := relateTestEntryCount(t, target.FilePath, "- 1"); n != 1 {
		t.Errorf("target file has %d entries for #1, want 1", n)
	}
}

func TestRelateCmdErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"invalid source id", []string{"relate", "x", "blocks", "2"}, "invalid issue ID: x"},
		{"invalid target id", []string{"relate", "1", "blocks", "y"}, "invalid issue ID: y"},
		{"unknown relation", []string{"relate", "1", "nope", "2"}, `invalid relation "nope", must be one of: blocks, depends-on, related-to, duplicates`},
		{"missing source", []string{"relate", "99", "blocks", "2"}, "issue #99 not found"},
		{"missing target", []string{"relate", "1", "blocks", "99"}, "issue #99 not found"},
		{"self relation", []string{"relate", "1", "blocks", "1"}, "cannot relate issue #1 to itself"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newCLIRepo(t)
			relateTestPair(t, r)

			stdout, _, err := r.run(t, tt.args...)
			if err == nil {
				t.Fatalf("relate succeeded, want error containing %q (stdout: %q)", tt.want, stdout)
			}
			wantContains(t, "error", err.Error(), tt.want)
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}

			for _, id := range []int{1, 2} {
				if got := relateTestTotalRelations(r.load(t, id)); got != 0 {
					t.Errorf("issue #%d carries %d relation entries after failed relate, want 0", id, got)
				}
			}
		})
	}
}

func TestRelateCmdBrokenConfig(t *testing.T) {
	r := newCLIRepo(t)
	relateTestPair(t, r)
	relateTestBreakConfig(t, r)

	stdout, _, err := r.run(t, "relate", "1", "blocks", "2")
	if err == nil {
		t.Fatalf("relate succeeded with a broken config, stdout: %q", stdout)
	}
	wantContains(t, "error", err.Error(), "yaml:")
	if got := relateTestTotalRelations(r.load(t, 1)); got != 0 {
		t.Errorf("source carries %d relation entries, want 0", got)
	}
}

// unrelateTestSeedRelations writes relation both onto source #1 and, via the
// inverse, onto target #2, mirroring what `issues relate` does.
func unrelateTestSeedRelations(t *testing.T, r *cliRepo, relation string) {
	t.Helper()
	relateTestPair(t, r)
	if err := issue.AddRelation(r.issuesDir, 1, relation, 2, &config.Config{AutoStage: false}); err != nil {
		t.Fatalf("seed %s relation: %v", relation, err)
	}
}

func TestUnrelateCmdRemovesBothSides(t *testing.T) {
	tests := []struct {
		relation    string
		sourceField string
		targetField string
	}{
		{"blocks", "blocks", "depends-on"},
		{"depends-on", "depends-on", "blocks"},
		{"related-to", "related-to", "related-to"},
		{"duplicates", "duplicates", "duplicates"},
	}

	for _, tt := range tests {
		t.Run(tt.relation, func(t *testing.T) {
			r := newCLIRepo(t)
			unrelateTestSeedRelations(t, r, tt.relation)

			stdout, stderr, err := r.run(t, "unrelate", "1", tt.relation, "2")
			if err != nil {
				t.Fatalf("unrelate failed: %v (stderr: %q)", err, stderr)
			}

			want := "Removed: #1 " + tt.relation + " #2\n" +
				"         #2 " + issue.Inverse(tt.relation) + " #1 (auto)\n"
			if stdout != want {
				t.Errorf("stdout = %q, want %q", stdout, want)
			}

			source := r.load(t, 1)
			target := r.load(t, 2)
			if got := relateTestField(t, source, tt.sourceField); len(got) != 0 {
				t.Errorf("source %s = %v, want empty", tt.sourceField, got)
			}
			if got := relateTestField(t, target, tt.targetField); len(got) != 0 {
				t.Errorf("target %s = %v, want empty", tt.targetField, got)
			}
			if got := relateTestTotalRelations(source); got != 0 {
				t.Errorf("source carries %d relation entries, want 0", got)
			}
			if got := relateTestTotalRelations(target); got != 0 {
				t.Errorf("target carries %d relation entries, want 0", got)
			}
		})
	}
}

func TestUnrelateCmdMissingRelationIsNoop(t *testing.T) {
	r := newCLIRepo(t)
	unrelateTestSeedRelations(t, r, "related-to")

	stdout, stderr, err := r.run(t, "unrelate", "1", "blocks", "2")
	if err != nil {
		t.Fatalf("unrelate of a missing relation failed: %v (stderr: %q)", err, stderr)
	}
	wantContains(t, "stdout", stdout, "Removed: #1 blocks #2")

	// The unrelated related-to relation must survive.
	source := r.load(t, 1)
	target := r.load(t, 2)
	if got := source.Relations.RelatedTo; !slices.Equal(got, []int{2}) {
		t.Errorf("source related-to = %v, want [2]", got)
	}
	if got := target.Relations.RelatedTo; !slices.Equal(got, []int{1}) {
		t.Errorf("target related-to = %v, want [1]", got)
	}
	if got := relateTestTotalRelations(source); got != 1 {
		t.Errorf("source carries %d relation entries, want 1", got)
	}
	if got := relateTestTotalRelations(target); got != 1 {
		t.Errorf("target carries %d relation entries, want 1", got)
	}
}

func TestUnrelateCmdErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"invalid source id", []string{"unrelate", "x", "blocks", "2"}, "invalid issue ID: x"},
		{"invalid target id", []string{"unrelate", "1", "blocks", "y"}, "invalid issue ID: y"},
		{"unknown relation", []string{"unrelate", "1", "nope", "2"}, `invalid relation "nope", must be one of: blocks, depends-on, related-to, duplicates`},
		{"missing source", []string{"unrelate", "99", "blocks", "2"}, "issue #99 not found"},
		{"missing target", []string{"unrelate", "1", "blocks", "99"}, "issue #99 not found"},
		{"self relation", []string{"unrelate", "1", "blocks", "1"}, "cannot unrelate issue #1 from itself"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newCLIRepo(t)
			unrelateTestSeedRelations(t, r, "related-to")

			stdout, _, err := r.run(t, tt.args...)
			if err == nil {
				t.Fatalf("unrelate succeeded, want error containing %q (stdout: %q)", tt.want, stdout)
			}
			wantContains(t, "error", err.Error(), tt.want)
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}

			// A failed unrelate must leave the seeded relation untouched.
			source := r.load(t, 1)
			target := r.load(t, 2)
			if got := source.Relations.RelatedTo; !slices.Equal(got, []int{2}) {
				t.Errorf("source related-to = %v, want [2]", got)
			}
			if got := target.Relations.RelatedTo; !slices.Equal(got, []int{1}) {
				t.Errorf("target related-to = %v, want [1]", got)
			}
		})
	}
}

// relateTestNoIssuesDir asserts that args fails cleanly outside a repository.
func relateTestNoIssuesDir(t *testing.T, args ...string) {
	t.Helper()
	t.Chdir(t.TempDir())
	if dir, err := issue.IssuesDir(); err == nil {
		t.Skipf("temp dir is inside an initialized repository: %s", dir)
	}

	stdout, _, err := runCLI(t, rootCmd, args...)
	if err == nil {
		t.Fatalf("%v succeeded without .issues/, stdout: %q", args, stdout)
	}
	wantContains(t, "error", err.Error(), "no .issues/ directory found (run 'issues init')")
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

func TestRelateCmdNoIssuesDir(t *testing.T) {
	relateTestNoIssuesDir(t, "relate", "1", "blocks", "2")
}

func TestUnrelateCmdNoIssuesDir(t *testing.T) {
	relateTestNoIssuesDir(t, "unrelate", "1", "blocks", "2")
}

func TestUnrelateCmdBrokenConfig(t *testing.T) {
	r := newCLIRepo(t)
	unrelateTestSeedRelations(t, r, "related-to")
	relateTestBreakConfig(t, r)

	stdout, _, err := r.run(t, "unrelate", "1", "related-to", "2")
	if err == nil {
		t.Fatalf("unrelate succeeded with a broken config, stdout: %q", stdout)
	}
	wantContains(t, "error", err.Error(), "yaml:")
	if got := r.load(t, 1).Relations.RelatedTo; len(got) != 1 {
		t.Errorf("source related-to = %v, want [2] (unchanged)", got)
	}
}
