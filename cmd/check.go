package cmd

import (
	"fmt"
	"io"
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

// checkRelations lists the relation fields inspected for orphans and symmetry.
var checkRelations = []string{"blocks", "depends-on", "related-to", "duplicates"}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check issue database for inconsistencies",
	RunE: func(cmd *cobra.Command, args []string) error {
		issuesDir, err := issue.IssuesDir()
		if err != nil {
			return err
		}

		autoFix, _ := cmd.Flags().GetBool("fix")

		report, err := runCheck(issuesDir, autoFix)
		if err != nil {
			return err
		}

		printCheckReport(report)
		if len(report.errors) > 0 {
			os.Exit(1)
		}
		return nil
	},
}

// checkReport collects the findings of one consistency run.
type checkReport struct {
	errors   []string
	warnings []string
	fixed    []string
	dirty    map[int]bool // issue IDs modified by --fix
}

func newCheckReport() *checkReport {
	return &checkReport{dirty: make(map[int]bool)}
}

func (r *checkReport) errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func (r *checkReport) warnf(format string, args ...any) {
	r.warnings = append(r.warnings, fmt.Sprintf(format, args...))
}

func (r *checkReport) fixedf(format string, args ...any) {
	r.fixed = append(r.fixed, fmt.Sprintf(format, args...))
}

// checkContext is the shared state of one run.
type checkContext struct {
	issues    []*issue.Issue
	byID      map[int]*issue.Issue // first occurrence wins on duplicate IDs
	brokenIDs map[int]bool         // IDs whose files cannot be parsed
	autoFix   bool
	report    *checkReport
}

// runCheck loads the database, runs every consistency check and applies fixes
// when asked. It does not print anything; look at the report instead.
func runCheck(issuesDir string, autoFix bool) (*checkReport, error) {
	issues, err := issue.LoadAll(issuesDir)
	if err != nil {
		return nil, err
	}

	broken, err := issue.BrokenFiles(issuesDir)
	if err != nil {
		return nil, err
	}

	ctx := &checkContext{
		issues:    issues,
		byID:      make(map[int]*issue.Issue, len(issues)),
		brokenIDs: make(map[int]bool),
		autoFix:   autoFix,
		report:    newCheckReport(),
	}
	for _, iss := range issues {
		if _, exists := ctx.byID[iss.ID]; !exists {
			ctx.byID[iss.ID] = iss
		}
	}

	ctx.checkBrokenFiles(broken)
	ctx.checkDuplicateIDs()

	for _, iss := range issues {
		ctx.checkValidation(iss)
		ctx.checkDuplicateLabels(iss)
		ctx.checkOrphans(iss)
		ctx.checkSymmetry(iss)
		ctx.checkStatus(iss)
	}

	if err := ctx.saveFixes(issuesDir); err != nil {
		return nil, err
	}
	return ctx.report, nil
}

// checkBrokenFiles reports unparsable files. They cannot be fixed, but their
// IDs are remembered so relations pointing at them are not called orphans.
func (c *checkContext) checkBrokenFiles(broken []issue.BrokenFile) {
	for _, bf := range broken {
		c.report.errorf("cannot parse %s: %v", bf.Path, bf.Err)
		if id, ok := issue.IDFromFilename(filepath.Base(bf.Path)); ok {
			c.brokenIDs[id] = true
		}
	}
}

func (c *checkContext) checkDuplicateIDs() {
	idCount := make(map[int][]string, len(c.issues))
	for _, iss := range c.issues {
		idCount[iss.ID] = append(idCount[iss.ID], iss.FilePath)
	}
	for id, files := range idCount {
		if len(files) > 1 {
			c.report.errorf("Duplicate ID #%d in files: %s", id, strings.Join(files, ", "))
		}
	}
}

func (c *checkContext) checkValidation(iss *issue.Issue) {
	if err := issue.Validate(iss); err != nil {
		c.report.errorf("#%d: validation error: %s", iss.ID, err)
	}
}

func (c *checkContext) checkDuplicateLabels(iss *issue.Issue) {
	seen := make(map[string]bool, len(iss.Labels))
	for _, label := range iss.Labels {
		if seen[label] {
			c.report.warnf("#%d: duplicate label %q", iss.ID, label)
		}
		seen[label] = true
	}
}

// checkOrphans reports relation targets that do not exist. Missing targets are
// removed with --fix unless the target file exists but cannot be parsed.
func (c *checkContext) checkOrphans(iss *issue.Issue) {
	for _, relation := range checkRelations {
		for _, refID := range getRelSlice(iss, relation) {
			if _, ok := c.byID[refID]; ok {
				continue
			}
			if c.brokenIDs[refID] {
				c.report.errorf("#%d: %s references broken issue #%d", iss.ID, relation, refID)
				continue
			}
			c.report.errorf("#%d: %s references non-existent #%d", iss.ID, relation, refID)
			if c.autoFix {
				issue.RemoveFromSlice(iss, relation, refID)
				c.report.fixedf("#%d: removed orphan %s reference to #%d", iss.ID, relation, refID)
				c.report.dirty[iss.ID] = true
			}
		}
	}
}

// checkSymmetry reports relations whose inverse is missing on the target.
func (c *checkContext) checkSymmetry(iss *issue.Issue) {
	for _, relation := range checkRelations {
		inverse := issue.Inverse(relation)
		for _, refID := range getRelSlice(iss, relation) {
			target, ok := c.byID[refID]
			if !ok {
				continue // already reported as orphan
			}
			if containsInt(getRelSlice(target, inverse), iss.ID) {
				continue
			}
			c.report.errorf("#%d %s #%d but #%d does not have inverse %s #%d",
				iss.ID, relation, refID, refID, inverse, iss.ID)
			if c.autoFix {
				issue.AddToSlice(target, inverse, iss.ID)
				c.report.fixedf("#%d: added missing %s #%d", refID, inverse, iss.ID)
				c.report.dirty[target.ID] = true
			}
		}
	}
}

func (c *checkContext) checkStatus(iss *issue.Issue) {
	if (iss.Status == "closed" || iss.Status == "wontfix") && iss.Closed == "" {
		c.report.warnf("#%d: status is %s but closed date is empty", iss.ID, iss.Status)
	}
	if iss.Status == "open" && iss.Closed != "" {
		c.report.warnf("#%d: status is open but has closed date %s", iss.ID, iss.Closed)
	}
}

// saveFixes writes back the issues touched by --fix and stages them.
func (c *checkContext) saveFixes(issuesDir string) error {
	if !c.autoFix || len(c.report.dirty) == 0 {
		return nil
	}

	cfg, err := config.Load(issuesDir)
	if err != nil {
		return err
	}

	for _, iss := range c.issues {
		if !c.report.dirty[iss.ID] {
			continue
		}
		if err := issue.Save(issuesDir, iss); err != nil {
			c.report.errorf("#%d: failed to save fix: %v", iss.ID, err)
			continue
		}
		if cfg.AutoStage {
			_ = git.Stage(iss.FilePath)
		}
	}
	return nil
}

func printCheckReport(report *checkReport) {
	if len(report.errors) == 0 && len(report.warnings) == 0 {
		fmt.Println("No issues found. Database is consistent.")
		return
	}

	printFindings(os.Stderr, "Errors", "✗", report.errors)
	printFindings(os.Stderr, "Warnings", "⚠", report.warnings)
	printFindings(os.Stdout, "Fixed", "✓", report.fixed)
}

func printFindings(w io.Writer, title, marker string, findings []string) {
	if len(findings) == 0 {
		return
	}
	_, _ = fmt.Fprintf(w, "%s (%d):\n", title, len(findings))
	for _, finding := range findings {
		_, _ = fmt.Fprintf(w, "  %s %s\n", marker, finding)
	}
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
