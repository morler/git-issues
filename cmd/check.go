package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/steviee/git-issues/internal/config"
	"github.com/steviee/git-issues/internal/git"
	"github.com/steviee/git-issues/internal/issue"

	"github.com/spf13/cobra"
)

func init() {
	checkCmd.Flags().Bool("fix", false, "Auto-fix fixable issues (broken relations)")
	rootCmd.AddCommand(checkCmd)
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check issue database for inconsistencies",
	RunE: func(cmd *cobra.Command, args []string) error {
		issuesDir, err := issue.IssuesDir()
		if err != nil {
			return err
		}

		autoFix, _ := cmd.Flags().GetBool("fix")

		issues, err := issue.LoadAll(issuesDir)
		if err != nil {
			return err
		}

		issueMap := make(map[int]*issue.Issue)
		var errors []string
		var warnings []string
		var fixed []string
		dirty := make(map[int]bool)
		brokenIDs := make(map[int]bool)

		// 0. Unparsable files (cannot be auto-fixed, but must not be invisible)
		broken, err := issue.BrokenFiles(issuesDir)
		if err != nil {
			return err
		}
		for _, bf := range broken {
			errors = append(errors, fmt.Sprintf("cannot parse %s: %v", bf.Path, bf.Err))
			if id, ok := issue.IDFromFilename(filepath.Base(bf.Path)); ok {
				brokenIDs[id] = true
			}
		}

		// 1. Check for duplicate IDs
		idCount := make(map[int][]string)
		for _, iss := range issues {
			idCount[iss.ID] = append(idCount[iss.ID], iss.FilePath)
		}
		for id, files := range idCount {
			if len(files) > 1 {
				errors = append(errors, fmt.Sprintf("Duplicate ID #%d in files: %s", id, strings.Join(files, ", ")))
			}
		}

		// Build map (use first occurrence for each ID)
		for _, iss := range issues {
			if _, exists := issueMap[iss.ID]; !exists {
				issueMap[iss.ID] = iss
			}
		}

		for _, iss := range issues {
			// 2. Validation check
			if err := issue.Validate(iss); err != nil {
				errors = append(errors, fmt.Sprintf("#%d: validation error: %s", iss.ID, err))
			}

			// 3. Duplicate labels within an issue
			labelSeen := make(map[string]bool)
			for _, l := range iss.Labels {
				if labelSeen[l] {
					warnings = append(warnings, fmt.Sprintf("#%d: duplicate label %q", iss.ID, l))
				}
				labelSeen[l] = true
			}

			// 4. Orphan references (relations pointing to non-existent issues)
			checkOrphans := func(relation string, ids []int) {
				for _, refID := range ids {
					if _, ok := issueMap[refID]; !ok {
						if brokenIDs[refID] {
							errors = append(errors, fmt.Sprintf("#%d: %s references broken issue #%d", iss.ID, relation, refID))
							continue
						}
						errors = append(errors, fmt.Sprintf("#%d: %s references non-existent #%d", iss.ID, relation, refID))
						if autoFix {
							issue.RemoveFromSlice(iss, relation, refID)
							fixed = append(fixed, fmt.Sprintf("#%d: removed orphan %s reference to #%d", iss.ID, relation, refID))
							dirty[iss.ID] = true
						}
					}
				}
			}
			checkOrphans("blocks", iss.Relations.Blocks)
			checkOrphans("depends-on", iss.Relations.DependsOn)
			checkOrphans("related-to", iss.Relations.RelatedTo)
			checkOrphans("duplicates", iss.Relations.Duplicates)

			// 5. Bidirectional relation symmetry
			checkSymmetry := func(relation string, ids []int) {
				inv := issue.Inverse(relation)
				for _, refID := range ids {
					target, ok := issueMap[refID]
					if !ok {
						continue // already reported as orphan
					}
					invSlice := getRelSlice(target, inv)
					if !containsInt(invSlice, iss.ID) {
						errors = append(errors, fmt.Sprintf("#%d %s #%d but #%d does not have inverse %s #%d",
							iss.ID, relation, refID, refID, inv, iss.ID))
						if autoFix {
							issue.AddToSlice(target, inv, iss.ID)
							fixed = append(fixed, fmt.Sprintf("#%d: added missing %s #%d", refID, inv, iss.ID))
							dirty[target.ID] = true
						}
					}
				}
			}
			checkSymmetry("blocks", iss.Relations.Blocks)
			checkSymmetry("depends-on", iss.Relations.DependsOn)
			checkSymmetry("related-to", iss.Relations.RelatedTo)
			checkSymmetry("duplicates", iss.Relations.Duplicates)

			// 6. Status inconsistencies
			if (iss.Status == "closed" || iss.Status == "wontfix") && iss.Closed == "" {
				warnings = append(warnings, fmt.Sprintf("#%d: status is %s but closed date is empty", iss.ID, iss.Status))
			}
			if iss.Status == "open" && iss.Closed != "" {
				warnings = append(warnings, fmt.Sprintf("#%d: status is open but has closed date %s", iss.ID, iss.Closed))
			}
		}

		// Save fixed issues (only the ones actually modified)
		if autoFix && len(dirty) > 0 {
			cfg, err := config.Load(issuesDir)
			if err != nil {
				return err
			}
			for _, iss := range issues {
				if !dirty[iss.ID] {
					continue
				}
				if err := issue.Save(issuesDir, iss); err != nil {
					errors = append(errors, fmt.Sprintf("#%d: failed to save fix: %v", iss.ID, err))
					continue
				}
				if cfg.AutoStage {
					_ = git.Stage(iss.FilePath)
				}
			}
		}

		// Output
		if len(errors) == 0 && len(warnings) == 0 {
			fmt.Println("No issues found. Database is consistent.")
			return nil
		}

		if len(errors) > 0 {
			_, _ = fmt.Fprintf(os.Stderr, "Errors (%d):\n", len(errors))
			for _, e := range errors {
				_, _ = fmt.Fprintf(os.Stderr, "  ✗ %s\n", e)
			}
		}

		if len(warnings) > 0 {
			_, _ = fmt.Fprintf(os.Stderr, "Warnings (%d):\n", len(warnings))
			for _, w := range warnings {
				_, _ = fmt.Fprintf(os.Stderr, "  ⚠ %s\n", w)
			}
		}

		if len(fixed) > 0 {
			_, _ = fmt.Fprintf(os.Stdout, "Fixed (%d):\n", len(fixed))
			for _, f := range fixed {
				_, _ = fmt.Fprintf(os.Stdout, "  ✓ %s\n", f)
			}
		}

		if len(errors) > 0 {
			os.Exit(1)
		}
		return nil
	},
}

func getRelSlice(iss *issue.Issue, relation string) []int {
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
		return nil
	}
}
