package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/steviee/git-issues/internal/config"
	"github.com/steviee/git-issues/internal/git"
	"github.com/steviee/git-issues/internal/issue"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(editCmd)
}

var editCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit an issue in $EDITOR",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}

		issuesDir, err := issue.IssuesDir()
		if err != nil {
			return err
		}

		cfg, err := config.Load(issuesDir)
		if err != nil {
			return err
		}

		iss, err := issue.LoadByID(issuesDir, id)
		if err != nil {
			return fmt.Errorf("issue #%d not found", id)
		}

		// Snapshot old relations for diff (will be used in Phase 5)
		oldRelations := iss.Relations
		original, err := os.ReadFile(iss.FilePath)
		if err != nil {
			return err
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}

		editorCmd := exec.Command(editor, iss.FilePath)
		editorCmd.Stdin = os.Stdin
		editorCmd.Stdout = os.Stdout
		editorCmd.Stderr = os.Stderr
		if err := editorCmd.Run(); err != nil {
			return fmt.Errorf("editor failed: %w", err)
		}

		// Re-parse the file
		edited, err := issue.ParseFile(iss.FilePath)
		if err != nil {
			return fmt.Errorf("error parsing edited file: %w", err)
		}

		// IDs are immutable and the filename encodes the ID, so a changed ID
		// would desynchronize LoadByID, NextID and the file name. Checked before
		// Validate so a deleted or typoed id restores the file instead of leaving
		// an unaddressable issue behind.
		if edited.ID != id {
			message := fmt.Sprintf("issue id changed from #%d to #%d; IDs are immutable", id, edited.ID)
			if err := os.WriteFile(iss.FilePath, original, 0o644); err != nil {
				return fmt.Errorf("%s; restoring the original file failed: %w", message, err)
			}
			// Restore the pre-edit content so the issue stays addressable; without
			// this the file would carry a new id and `issues show` could not find it.
			return fmt.Errorf("%s; original file restored", message)
		}

		if err := issue.Validate(edited); err != nil {
			return fmt.Errorf("%w", err)
		}

		name := filepath.Base(edited.FilePath)
		fileID, ok := issue.IDFromFilename(name)
		if !ok {
			return fmt.Errorf("filename %q does not start with an issue ID", name)
		}
		if fileID != id {
			return fmt.Errorf("filename %s does not match issue #%d", name, id)
		}

		// Save (updates `updated` field)
		if err := issue.Save(issuesDir, edited); err != nil {
			return err
		}

		if cfg.AutoStage {
			_ = git.Stage(issuesDir, edited.FilePath)
		}

		// Sync relation changes bidirectionally
		added, removed := issue.DiffRelations(oldRelations, edited.Relations)
		for _, entry := range added {
			// Source already has the relation from user edit, just update target's inverse
			target, err := issue.LoadByID(issuesDir, entry.ID)
			if err != nil {
				continue
			}
			issue.AddToSlice(target, issue.Inverse(entry.Relation), id)
			if err := issue.Save(issuesDir, target); err != nil {
				return fmt.Errorf("failed to sync relation with #%d: %w", entry.ID, err)
			}
			if cfg.AutoStage {
				_ = git.Stage(issuesDir, target.FilePath)
			}
		}
		for _, entry := range removed {
			target, err := issue.LoadByID(issuesDir, entry.ID)
			if err != nil {
				continue
			}
			issue.RemoveFromSlice(target, issue.Inverse(entry.Relation), id)
			if err := issue.Save(issuesDir, target); err != nil {
				return fmt.Errorf("failed to sync relation with #%d: %w", entry.ID, err)
			}
			if cfg.AutoStage {
				_ = git.Stage(issuesDir, target.FilePath)
			}
		}

		fmt.Printf("Updated: #%d\n", id)
		return nil
	},
}
