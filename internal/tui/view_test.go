package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/steviee/git-issues/internal/issue"
)

func TestApplyStatus(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		closed     string
		wantClosed string
	}{
		{"start work clears closed", "in-progress", "2026-01-10", ""},
		{"reopen clears closed", "open", "2026-01-10", ""},
		{"close stamps today", "closed", "", issue.Today()},
		{"wontfix stamps today", "wontfix", "", issue.Today()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iss := &issue.Issue{Status: "open", Closed: tt.closed}
			applyStatus(iss, tt.status)
			if iss.Status != tt.status {
				t.Errorf("status = %q, want %q", iss.Status, tt.status)
			}
			if iss.Closed != tt.wantClosed {
				t.Errorf("closed = %q, want %q", iss.Closed, tt.wantClosed)
			}
		})
	}
}

func TestRenderBoardStatus(t *testing.T) {
	if got := renderBoardStatus(nil); strings.Contains(got, "Error:") {
		t.Errorf("error-free status contains an error line: %q", got)
	}

	got := renderBoardStatus(errors.New("save failed"))
	for _, want := range []string{"Error: save failed", "q: quit"} {
		if !strings.Contains(got, want) {
			t.Errorf("status missing %q: %q", want, got)
		}
	}
}

func TestRenderDetailContent(t *testing.T) {
	iss := &issue.Issue{
		ID:       7,
		Title:    "Detail issue",
		Status:   "open",
		Priority: "high",
		Labels:   []string{"bug", "auth"},
		Created:  "2026-01-01",
		Updated:  "2026-01-02",
		Relations: issue.Relations{
			Blocks:    []int{2},
			DependsOn: []int{3},
		},
	}

	out := renderDetailContent(iss, 40)
	for _, want := range []string{
		"Issue #7",
		"Detail issue",
		"bug, auth",
		"Created:  2026-01-01",
		"Updated:  2026-01-02",
		"Blocks:",
		"#2",
		"Depends on:",
		"#3",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("detail content missing %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"Related to:", "Duplicates:", "Closed:"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("detail content unexpectedly contains %q:\n%s", unwanted, out)
		}
	}

	body := renderDetailContent(&issue.Issue{ID: 1, Title: "Body", Body: "## Heading\n"}, 40)
	if !strings.Contains(body, "Heading") {
		t.Errorf("rendered body heading missing from detail content:\n%s", body)
	}
	if strings.Contains(body, "## Heading") {
		t.Errorf("body was not rendered as markdown:\n%s", body)
	}
}
