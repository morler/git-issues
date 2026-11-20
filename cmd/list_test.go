package cmd

import (
	"slices"
	"testing"

	"github.com/steviee/git-issues/internal/issue"
)

func testIssue(id int, status, priority string, labels []string, blocks, dependsOn []int) *issue.Issue {
	return &issue.Issue{
		ID:       id,
		Title:    "Issue",
		Status:   status,
		Priority: priority,
		Labels:   labels,
		Relations: issue.Relations{
			Blocks:    blocks,
			DependsOn: dependsOn,
		},
		Created: "2026-01-01",
		Updated: "2026-01-01",
	}
}

func ids(issues []*issue.Issue) []int {
	result := make([]int, len(issues))
	for i, iss := range issues {
		result[i] = iss.ID
	}
	return result
}

func TestFilterIssues(t *testing.T) {
	all := []*issue.Issue{
		testIssue(1, "open", "high", []string{"bug"}, nil, nil),
		testIssue(2, "open", "low", []string{"feature"}, nil, nil),
		testIssue(3, "closed", "critical", []string{"bug", "auth"}, nil, nil),
		testIssue(4, "in-progress", "medium", nil, []int{9}, []int{1}),
	}

	tests := []struct {
		name        string
		status      string
		priority    string
		labels      []string
		blocksID    int
		dependsOnID int
		want        []int
	}{
		{"default open only", "open", "", nil, 0, 0, []int{1, 2}},
		{"all statuses", "all", "", nil, 0, 0, []int{1, 2, 3, 4}},
		{"priority filter", "all", "critical", nil, 0, 0, []int{3}},
		{"single label", "all", "", []string{"auth"}, 0, 0, []int{3}},
		{"label OR logic", "all", "", []string{"feature", "auth"}, 0, 0, []int{2, 3}},
		{"status and label combined", "open", "", []string{"bug"}, 0, 0, []int{1}},
		{"blocks filter", "all", "", nil, 9, 0, []int{4}},
		{"depends-on filter", "all", "", nil, 0, 1, []int{4}},
		{"no match", "open", "critical", nil, 0, 0, []int{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ids(filterIssues(all, tt.status, tt.priority, tt.labels, tt.blocksID, tt.dependsOnID))
			if !slices.Equal(got, tt.want) {
				t.Errorf("filterIssues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterIssuesNoMatchReturnsEmptySlice(t *testing.T) {
	all := []*issue.Issue{testIssue(1, "open", "high", nil, nil, nil)}

	got := filterIssues(all, "open", "critical", nil, 0, 0)
	if got == nil {
		t.Fatal("filterIssues returned nil for no matches, want an empty slice")
	}
	if len(got) != 0 {
		t.Errorf("filterIssues returned %v, want empty", got)
	}
}

func TestSortIssuesPriority(t *testing.T) {
	all := []*issue.Issue{
		testIssue(1, "open", "low", nil, nil, nil),
		testIssue(2, "open", "critical", nil, nil, nil),
		testIssue(3, "open", "high", nil, nil, nil),
		testIssue(4, "open", "medium", nil, nil, nil),
	}

	sortIssues(all, "priority")

	want := []int{2, 3, 4, 1}
	if got := ids(all); !slices.Equal(got, want) {
		t.Errorf("priority sort = %v, want %v", got, want)
	}
}

func TestSortIssuesByID(t *testing.T) {
	all := []*issue.Issue{
		testIssue(3, "open", "low", nil, nil, nil),
		testIssue(1, "open", "critical", nil, nil, nil),
		testIssue(2, "open", "high", nil, nil, nil),
	}

	sortIssues(all, "id")

	want := []int{1, 2, 3}
	if got := ids(all); !slices.Equal(got, want) {
		t.Errorf("id sort = %v, want %v", got, want)
	}
}
