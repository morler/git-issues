package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/steviee/git-issues/internal/config"
	"github.com/steviee/git-issues/internal/git"
	"github.com/steviee/git-issues/internal/issue"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func init() {
	closeCmd.Flags().Bool("wontfix", false, "Close as wontfix")
	closeCmd.Flags().String("reason", "", "Reason for closing")
	closeCmd.Flags().Bool("force", false, "Close even if there are open blockers (skips the prompt)")
	rootCmd.AddCommand(closeCmd)
}

var closeCmd = &cobra.Command{
	Use:   "close <id>",
	Short: "Close an issue",
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

		// Check for open blockers
		force, _ := cmd.Flags().GetBool("force")
		proceed, err := closeWithBlockersGate(issuesDir, iss, force)
		if err != nil {
			return err
		}
		if !proceed {
			return nil
		}

		wontfix, _ := cmd.Flags().GetBool("wontfix")
		reason, _ := cmd.Flags().GetString("reason")

		if wontfix {
			iss.Status = "wontfix"
		} else {
			iss.Status = "closed"
		}
		iss.Closed = issue.Today()

		if reason != "" {
			if iss.Body != "" && !strings.HasSuffix(iss.Body, "\n") {
				iss.Body += "\n"
			}
			iss.Body += "\n## Closed\n" + reason + "\n"
		}

		if err := issue.Save(issuesDir, iss); err != nil {
			return err
		}

		if cfg.AutoStage {
			_ = git.Stage(iss.FilePath)
		}

		fmt.Printf("Closed: #%d\n", id)
		return nil
	},
}

func stdinIsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// closeWithBlockersGate applies the shared close policy for one issue: open
// blockers are reported on stderr, --force skips the check, an interactive
// terminal prompts for confirmation, and non-interactive callers get a hard
// error. proceed is false when the close must not happen; err is nil when the
// user declined the prompt.
func closeWithBlockersGate(issuesDir string, iss *issue.Issue, force bool) (proceed bool, err error) {
	if force || len(iss.Relations.DependsOn) == 0 {
		return true, nil
	}

	blockers := openBlockers(issuesDir, iss)
	if len(blockers) == 0 {
		return true, nil
	}

	for _, b := range blockers {
		fmt.Fprintf(os.Stderr, "Warning: #%d has open blocker: %s\n", iss.ID, b)
	}

	if !stdinIsTerminal() {
		return false, fmt.Errorf("refusing to close #%d with open blockers in non-interactive mode (use --force)", iss.ID)
	}

	fmt.Fprint(os.Stderr, "Close anyway? [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		fmt.Println("Aborted.")
		return false, nil
	}
	return true, nil
}

func openBlockers(issuesDir string, iss *issue.Issue) []string {
	var blockers []string
	for _, depID := range iss.Relations.DependsOn {
		dep, err := issue.LoadByID(issuesDir, depID)
		if err != nil {
			continue
		}
		if dep.Status == "open" || dep.Status == "in-progress" {
			blockers = append(blockers, fmt.Sprintf("#%d (%s)", dep.ID, dep.Title))
		}
	}
	return blockers
}
