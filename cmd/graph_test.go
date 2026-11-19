package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/steviee/git-issues/internal/issue"
)

func TestPrintNodeCycle(t *testing.T) {
	a := testIssue(1, "open", "high", nil, []int{2}, nil)
	b := testIssue(2, "open", "medium", nil, []int{1}, nil)
	issues := map[int]*issue.Issue{1: a, 2: b}

	var buf bytes.Buffer
	printNode(&buf, issues, 1, "", map[int]bool{}, map[int]bool{})

	out := buf.String()
	for _, want := range []string{"#1", "#2", "blocks:", "cycle detected at #1"} {
		if !strings.Contains(out, want) {
			t.Errorf("graph output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintNodeChain(t *testing.T) {
	a := testIssue(1, "open", "high", nil, []int{2}, nil)
	b := testIssue(2, "open", "medium", nil, []int{3}, nil)
	c := testIssue(3, "open", "low", nil, nil, nil)
	issues := map[int]*issue.Issue{1: a, 2: b, 3: c}

	var buf bytes.Buffer
	printNode(&buf, issues, 1, "", map[int]bool{}, map[int]bool{})

	out := buf.String()
	for _, want := range []string{"#1", "#2", "#3", "blocks:"} {
		if !strings.Contains(out, want) {
			t.Errorf("graph output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "cycle detected") {
		t.Errorf("unexpected cycle warning:\n%s", out)
	}
}
