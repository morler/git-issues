package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/steviee/git-issues/internal/config"
	"github.com/steviee/git-issues/internal/git"
	"github.com/steviee/git-issues/internal/issue"

	"github.com/spf13/cobra"
)

func init() {
	newCmd.Flags().StringP("title", "t", "", "Issue title")
	newCmd.Flags().StringP("priority", "p", "", "Priority (low|medium|high|critical)")
	newCmd.Flags().StringSliceP("label", "l", nil, "Labels (repeatable)")
	newCmd.Flags().StringP("body", "b", "", "Issue body")
	newCmd.Flags().IntSlice(issue.RelationBlocks, nil, "IDs this issue blocks (repeatable)")
	newCmd.Flags().IntSlice(issue.RelationDependsOn, nil, "IDs this issue depends on (repeatable)")
	newCmd.Flags().IntSlice(issue.RelationRelatedTo, nil, "IDs related to this issue (repeatable)")
	rootCmd.AddCommand(newCmd)
}

const editorTemplate = `---
title: ""
priority: medium
labels: []
relations:
  blocks: []
  depends-on: []
---

<!-- Describe the issue here. Delete this line. -->
`

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new issue",
	RunE: func(cmd *cobra.Command, args []string) error {
		issuesDir, err := issue.IssuesDir()
		if err != nil {
			return err
		}

		cfg, err := config.Load(issuesDir)
		if err != nil {
			return err
		}

		title, _ := cmd.Flags().GetString("title")
		priority, _ := cmd.Flags().GetString("priority")
		labels, _ := cmd.Flags().GetStringSlice("label")
		body, _ := cmd.Flags().GetString("body")
		blocks, _ := cmd.Flags().GetIntSlice(issue.RelationBlocks)
		dependsOn, _ := cmd.Flags().GetIntSlice(issue.RelationDependsOn)
		relatedTo, _ := cmd.Flags().GetIntSlice(issue.RelationRelatedTo)

		if priority == "" {
			priority = cfg.DefaultPriority
		}

		if title == "" {
			// Open editor
			var err error
			title, priority, labels, body, blocks, dependsOn, relatedTo, err = openEditor(cfg)
			if err != nil {
				return err
			}
		}

		if title == "" {
			fmt.Fprintln(os.Stderr, "Aborted.")
			return nil
		}

		allTargets := append(append(append([]int{}, blocks...), dependsOn...), relatedTo...)
		if err := ensureIssuesExist(issuesDir, allTargets); err != nil {
			return err
		}

		nextID, err := issue.NextID(issuesDir)
		if err != nil {
			return err
		}

		if labels == nil {
			labels = []string{}
		}

		iss := &issue.Issue{
			ID:       nextID,
			Title:    title,
			Status:   "open",
			Priority: priority,
			Labels:   labels,
			Created:  issue.Today(),
			Updated:  issue.Today(),
			Body:     body,
		}

		if err := issue.Save(issuesDir, iss); err != nil {
			return fmt.Errorf("%w", err)
		}

		if cfg.AutoStage {
			_ = git.Stage(issuesDir, iss.FilePath)
		}

		// Sync relations bidirectionally
		for _, rel := range []struct {
			name string
			ids  []int
		}{
			{issue.RelationBlocks, blocks},
			{issue.RelationDependsOn, dependsOn},
			{issue.RelationRelatedTo, relatedTo},
		} {
			if err := applyRelations(issuesDir, nextID, rel.name, rel.ids, cfg); err != nil {
				return err
			}
		}

		filename := filepath.Base(iss.FilePath)
		fmt.Printf("Created: .issues/%s (#%d)\n", filename, iss.ID)
		return nil
	},
}

func openEditor(cfg *config.Config) (title, priority string, labels []string, body string, blocks, dependsOn, relatedTo []int, err error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	tmpFile, err := os.CreateTemp("", "issue-*.md")
	if err != nil {
		return "", "", nil, "", nil, nil, nil, err
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmpFile.WriteString(editorTemplate); err != nil {
		_ = tmpFile.Close()
		return "", "", nil, "", nil, nil, nil, err
	}
	_ = tmpFile.Close()

	editorCmd := exec.Command(editor, tmpPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	if err := editorCmd.Run(); err != nil {
		return "", "", nil, "", nil, nil, nil, fmt.Errorf("editor failed: %w", err)
	}

	iss, err := issue.ParseFile(tmpPath)
	if err != nil {
		return "", "", nil, "", nil, nil, nil, err
	}

	if iss.Title == "" {
		return "", "", nil, "", nil, nil, nil, nil
	}

	priority = iss.Priority
	if priority == "" {
		priority = cfg.DefaultPriority
	}

	return iss.Title, priority, iss.Labels, iss.Body, iss.Relations.Blocks, iss.Relations.DependsOn, iss.Relations.RelatedTo, nil
}

func ensureIssuesExist(issuesDir string, ids []int) error {
	for _, id := range ids {
		if _, err := issue.LoadByID(issuesDir, id); err != nil {
			return err
		}
	}
	return nil
}

func applyRelations(issuesDir string, sourceID int, relation string, ids []int, cfg *config.Config) error {
	for _, targetID := range ids {
		if err := issue.AddRelation(issuesDir, sourceID, relation, targetID, cfg); err != nil {
			return fmt.Errorf("issue #%d created, but could not add %s relation to #%d: %w", sourceID, relation, targetID, err)
		}
	}
	return nil
}

func parseID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid issue ID: %s", s)
	}
	return id, nil
}
