package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/steviee/git-issues/internal/config"
	"github.com/steviee/git-issues/internal/git"
	"github.com/steviee/git-issues/internal/issue"

	"github.com/spf13/cobra"
)

func init() {
	batchCloseCmd.Flags().Bool("wontfix", false, "Close as wontfix")
	batchCloseCmd.Flags().StringSlice("label", nil, "Filter by label (OR logic)")
	batchCloseCmd.Flags().String("status", "open", "Filter by current status")
	batchCloseCmd.Flags().Bool("force", false, "Close even if there are open blockers")

	batchSetCmd.Flags().StringSlice("label", nil, "Filter by label (OR logic)")
	batchSetCmd.Flags().String("status", "open", "Filter by current status")

	batchCmd.AddCommand(batchCloseCmd)
	batchCmd.AddCommand(batchSetCmd)
	rootCmd.AddCommand(batchCmd)
}

var batchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Bulk operations on multiple issues",
}

var batchCloseCmd = &cobra.Command{
	Use:   "close",
	Short: "Close multiple issues matching filters",
	RunE: func(cmd *cobra.Command, args []string) error {
		issuesDir, err := issue.IssuesDir()
		if err != nil {
			return err
		}

		cfg, err := config.Load(issuesDir)
		if err != nil {
			return err
		}

		labels, _ := cmd.Flags().GetStringSlice("label")
		status, _ := cmd.Flags().GetString("status")
		wontfix, _ := cmd.Flags().GetBool("wontfix")
		force, _ := cmd.Flags().GetBool("force")

		issues, err := issue.LoadAll(issuesDir)
		if err != nil {
			return err
		}

		var selected []*issue.Issue
		for _, iss := range issues {
			if status != "all" && iss.Status != status {
				continue
			}
			if len(labels) > 0 && !hasAnyLabel(iss.Labels, labels) {
				continue
			}
			selected = append(selected, iss)
		}

		// Refuse the whole batch while any selected issue has open blockers that
		// this batch does not close itself, so a bulk close cannot bypass the
		// guard `issues close` and `issues done` use.
		if !force {
			byID := make(map[int]*issue.Issue, len(issues))
			for _, iss := range issues {
				byID[iss.ID] = iss
			}
			selectedIDs := make(map[int]bool, len(selected))
			for _, iss := range selected {
				selectedIDs[iss.ID] = true
			}

			var blocked []string
			for _, iss := range selected {
				blockers := openBlockersFrom(byID, iss, selectedIDs)
				if len(blockers) == 0 {
					continue
				}
				for _, b := range blockers {
					fmt.Fprintf(os.Stderr, "Warning: #%d has open blocker: %s\n", iss.ID, b)
				}
				blocked = append(blocked, fmt.Sprintf("#%d", iss.ID))
			}
			if len(blocked) > 0 {
				return fmt.Errorf("refusing to close %s with open blockers (use --force)", strings.Join(blocked, ", "))
			}
		}

		count := 0
		for _, iss := range selected {
			if wontfix {
				iss.Status = "wontfix"
			} else {
				iss.Status = "closed"
			}
			iss.Closed = issue.Today()

			if err := issue.Save(issuesDir, iss); err != nil {
				fmt.Printf("Error closing #%d: %s\n", iss.ID, err)
				continue
			}
			if cfg.AutoStage {
				_ = git.Stage(issuesDir, iss.FilePath)
			}
			count++
			fmt.Printf("Closed: #%d %s\n", iss.ID, iss.Title)
		}

		fmt.Printf("\n%d issues closed.\n", count)
		return nil
	},
}

var batchSetCmd = &cobra.Command{
	Use:   "set <field> <value>",
	Short: "Set a field on multiple issues matching filters",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		field := args[0]
		value := args[1]

		issuesDir, err := issue.IssuesDir()
		if err != nil {
			return err
		}

		cfg, err := config.Load(issuesDir)
		if err != nil {
			return err
		}

		// Validate once up front: matching no issues must not turn a typo in a
		// scripted call into a silent success.
		if err := validateBatchSet(field, value); err != nil {
			return err
		}

		labels, _ := cmd.Flags().GetStringSlice("label")
		status, _ := cmd.Flags().GetString("status")

		issues, err := issue.LoadAll(issuesDir)
		if err != nil {
			return err
		}

		count := 0
		for _, iss := range issues {
			if status != "all" && iss.Status != status {
				continue
			}
			if len(labels) > 0 && !hasAnyLabel(iss.Labels, labels) {
				continue
			}

			switch field {
			case "priority":
				iss.Priority = value
			case "status":
				iss.Status = value
				if value == "closed" || value == "wontfix" {
					iss.Closed = issue.Today()
				} else {
					iss.Closed = ""
				}
			case "label":
				if strings.HasPrefix(value, "+") {
					label := value[1:]
					if !containsStr(iss.Labels, label) {
						iss.Labels = append(iss.Labels, label)
					}
				} else if strings.HasPrefix(value, "-") {
					label := value[1:]
					iss.Labels = removeStr(iss.Labels, label)
				} else {
					iss.Labels = strings.Split(value, ",")
					for i := range iss.Labels {
						iss.Labels[i] = strings.TrimSpace(iss.Labels[i])
					}
				}
			}

			if err := issue.Save(issuesDir, iss); err != nil {
				fmt.Printf("Error updating #%d: %s\n", iss.ID, err)
				continue
			}
			if cfg.AutoStage {
				_ = git.Stage(issuesDir, iss.FilePath)
			}
			count++
			fmt.Printf("Updated #%d: %s = %s\n", iss.ID, field, value)
		}

		fmt.Printf("\n%d issues updated.\n", count)
		return nil
	},
}

// validateBatchSet rejects unknown fields and invalid enum values before any
// issue is matched, so the same call fails whether or not the filter hits.
func validateBatchSet(field, value string) error {
	switch field {
	case "priority":
		if !contains(issue.ValidPriorities, value) {
			return fmt.Errorf("invalid priority %q, must be one of: %s", value, strings.Join(issue.ValidPriorities, ", "))
		}
	case "status":
		if !contains(issue.ValidStatuses, value) {
			return fmt.Errorf("invalid status %q, must be one of: %s", value, strings.Join(issue.ValidStatuses, ", "))
		}
	case "label":
	default:
		return fmt.Errorf("unknown field %q for batch set, must be one of: priority, status, label", field)
	}
	return nil
}
