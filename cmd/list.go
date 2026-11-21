package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/steviee/git-issues/internal/issue"

	"github.com/spf13/cobra"
)

func init() {
	listCmd.Flags().String("status", "open", "Filter by status (open|in-progress|closed|wontfix|all)")
	listCmd.Flags().String("priority", "", "Filter by priority (low|medium|high|critical)")
	listCmd.Flags().StringSlice("label", nil, "Filter by label (OR logic, repeatable)")
	listCmd.Flags().Int("blocks", 0, "Filter issues that block this ID")
	listCmd.Flags().Int("depends-on", 0, "Filter issues that depend on this ID")
	listCmd.Flags().String("sort", "priority", "Sort by (priority|id|updated|created)")
	listCmd.Flags().String("format", "table", "Output format (table|json|ids)")
	rootCmd.AddCommand(listCmd)
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		issuesDir, err := issue.IssuesDir()
		if err != nil {
			return err
		}

		opts := filterOptionsFromFlags(cmd)
		sortBy, _ := cmd.Flags().GetString("sort")
		format, _ := cmd.Flags().GetString("format")

		issues, err := issue.LoadAll(issuesDir)
		if err != nil {
			return err
		}

		// Build ID->Issue lookup for relation checks
		issueMap := make(map[int]*issue.Issue)
		for _, iss := range issues {
			issueMap[iss.ID] = iss
		}

		// Filter
		filtered := filterIssues(issues, opts)

		// Sort
		sortIssues(filtered, sortBy)

		// Output
		switch format {
		case "json":
			data, err := json.MarshalIndent(filtered, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
		case "ids":
			ids := make([]string, len(filtered))
			for i, iss := range filtered {
				ids[i] = fmt.Sprintf("%d", iss.ID)
			}
			fmt.Println(strings.Join(ids, " "))
		default:
			printTable(filtered, issueMap, opts)
		}

		return nil
	},
}

// filterOptions collects the `issues list` filter flags.
type filterOptions struct {
	status      string
	priority    string
	labels      []string
	blocksID    int
	dependsOnID int
}

// filterOptionsFromFlags reads the list filters from the command flags.
func filterOptionsFromFlags(cmd *cobra.Command) filterOptions {
	var opts filterOptions
	opts.status, _ = cmd.Flags().GetString("status")
	opts.priority, _ = cmd.Flags().GetString("priority")
	opts.labels, _ = cmd.Flags().GetStringSlice("label")
	opts.blocksID, _ = cmd.Flags().GetInt("blocks")
	opts.dependsOnID, _ = cmd.Flags().GetInt("depends-on")
	return opts
}

func filterIssues(issues []*issue.Issue, opts filterOptions) []*issue.Issue {
	// Start non-nil so an empty result marshals as [] instead of null.
	filtered := make([]*issue.Issue, 0, len(issues))
	for _, iss := range issues {
		if matchesFilters(iss, opts) {
			filtered = append(filtered, iss)
		}
	}
	return filtered
}

// matchesFilters reports whether an issue satisfies every active filter.
func matchesFilters(iss *issue.Issue, opts filterOptions) bool {
	if opts.status != "all" && iss.Status != opts.status {
		return false
	}
	if opts.priority != "" && iss.Priority != opts.priority {
		return false
	}
	if len(opts.labels) > 0 && !hasAnyLabel(iss.Labels, opts.labels) {
		return false
	}
	if opts.blocksID != 0 && !containsInt(iss.Relations.Blocks, opts.blocksID) {
		return false
	}
	if opts.dependsOnID != 0 && !containsInt(iss.Relations.DependsOn, opts.dependsOnID) {
		return false
	}
	return true
}

func hasAnyLabel(issueLabels, filterLabels []string) bool {
	for _, fl := range filterLabels {
		for _, il := range issueLabels {
			if il == fl {
				return true
			}
		}
	}
	return false
}

func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

func sortIssues(issues []*issue.Issue, sortBy string) {
	sort.SliceStable(issues, func(i, j int) bool {
		switch sortBy {
		case "id":
			return issues[i].ID < issues[j].ID
		case "updated":
			return issues[i].Updated > issues[j].Updated
		case "created":
			return issues[i].Created > issues[j].Created
		default: // priority
			ri := issue.PriorityRank(issues[i].Priority)
			rj := issue.PriorityRank(issues[j].Priority)
			if ri != rj {
				return ri > rj
			}
			return issues[i].ID < issues[j].ID
		}
	})
}

// emptyListMessage names the active filters so an empty table is not mistaken
// for an empty database.
func emptyListMessage(opts filterOptions) string {
	if opts.priority != "" || len(opts.labels) > 0 || opts.blocksID != 0 || opts.dependsOnID != 0 {
		return "No matching issues found."
	}
	if opts.status == "all" {
		return "No issues found."
	}
	return fmt.Sprintf("No %s issues found.", opts.status)
}

func printTable(issues []*issue.Issue, issueMap map[int]*issue.Issue, opts filterOptions) {
	if len(issues) == 0 {
		fmt.Println(emptyListMessage(opts))
		return
	}

	hasSymbols := false

	_, _ = fmt.Fprintf(os.Stdout, "%s%s%s%s%s%s\n",
		padRight("ID", 6), padRight("PRI", 10), padRight("STATUS", 13), padRight("TITLE", 46), padRight("LABELS", 14), "")

	for _, iss := range issues {
		relStr, hasMarker := relationMarker(iss, issueMap)
		hasSymbols = hasSymbols || hasMarker

		_, _ = fmt.Fprintf(os.Stdout, "%s%s%s%s%s%s\n",
			padRight(fmt.Sprintf("%04d", iss.ID), 6),
			padRight(iss.Priority, 10),
			padRight(iss.Status, 13),
			padRight(truncateDisplay(iss.Title, 44), 46),
			padRight(formatLabels(iss.Labels), 14),
			relStr)
	}

	if hasSymbols {
		fmt.Println()
		fmt.Println("⬛ = blocks open issues   ⬜ = blocked by open issue")
	}
}

// formatLabels renders the label column, or an empty string when the issue has
// no labels.
func formatLabels(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	return "[" + strings.Join(labels, ", ") + "]"
}

// relationMarker renders the ⬛/⬜ relation column and reports whether any
// marker was printed. Only relations to still-open issues count, so a closed
// blocker disappears from the table.
func relationMarker(iss *issue.Issue, issueMap map[int]*issue.Issue) (string, bool) {
	parts := make([]string, 0, 2)
	if ids := openIDs(iss.Relations.Blocks, issueMap); len(ids) > 0 {
		parts = append(parts, "⬛ "+formatIDList(ids))
	}
	if ids := openIDs(iss.Relations.DependsOn, issueMap); len(ids) > 0 {
		parts = append(parts, "⬜ "+formatIDList(ids))
	}
	return strings.Join(parts, "  "), len(parts) > 0
}

// openIDs filters ids down to the issues that are still open or in progress.
func openIDs(ids []int, issueMap map[int]*issue.Issue) []int {
	var open []int
	for _, id := range ids {
		related, ok := issueMap[id]
		if ok && (related.Status == issue.StatusOpen || related.Status == issue.StatusInProgress) {
			open = append(open, id)
		}
	}
	return open
}

func formatIDList(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d", id)
	}
	return strings.Join(parts, ",")
}
