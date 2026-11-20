package issue

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func IssuesDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(dir, ".issues")
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no .issues/ directory found (run 'issues init')")
		}
		dir = parent
	}
}

func LoadAll(issuesDir string) ([]*Issue, error) {
	issues, broken, err := LoadWithBroken(issuesDir)
	if err != nil {
		return nil, err
	}

	for _, bf := range broken {
		fmt.Fprintf(os.Stderr, "Warning: skipping issue file %s: %v\n", filepath.Base(bf.Path), bf.Err)
	}
	return issues, nil
}

// LoadWithBroken returns the parsed issues and the unparsable files of
// issuesDir from a single directory pass, without printing warnings.
func LoadWithBroken(issuesDir string) ([]*Issue, []BrokenFile, error) {
	scan, err := scanIssues(issuesDir)
	if err != nil {
		return nil, nil, err
	}
	return scan.issues, scan.broken, nil
}

// issueScan is the result of reading the issues directory once.
type issueScan struct {
	names  []string     // issue file names that match the naming convention
	issues []*Issue     // successfully parsed issues, in directory order
	broken []BrokenFile // files that failed to parse, in directory order
}

// scanIssues reads issuesDir and parses every issue file, separating readable
// issues from parse failures. Callers decide how to report the latter.
func scanIssues(issuesDir string) (issueScan, error) {
	entries, err := os.ReadDir(issuesDir)
	if err != nil {
		return issueScan{}, err
	}

	var scan issueScan
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, ".") {
			continue
		}

		path := filepath.Join(issuesDir, name)
		iss, err := ParseFile(path)
		scan.names = append(scan.names, name)
		if err != nil {
			scan.broken = append(scan.broken, BrokenFile{Path: path, Err: err})
			continue
		}
		scan.issues = append(scan.issues, iss)
	}
	return scan, nil
}

// BrokenFile describes an issue file that could not be parsed.
type BrokenFile struct {
	Path string
	Err  error
}

func LoadByID(issuesDir string, id int) (*Issue, error) {
	issues, err := LoadAll(issuesDir)
	if err != nil {
		return nil, err
	}
	for _, issue := range issues {
		if issue.ID == id {
			return issue, nil
		}
	}
	return nil, fmt.Errorf("issue #%d not found", id)
}

func Save(issuesDir string, issue *Issue) error {
	issue.Updated = Today()

	if err := Validate(issue); err != nil {
		return err
	}

	data := Marshal(issue)

	if issue.FilePath == "" {
		slug := GenerateSlug(issue.Title)
		issue.FilePath = filepath.Join(issuesDir, Filename(issue.ID, slug))
	}

	return os.WriteFile(issue.FilePath, data, 0644)
}

func Delete(issuesDir string, id int) error {
	issue, err := LoadByID(issuesDir, id)
	if err != nil {
		return err
	}
	return os.Remove(issue.FilePath)
}
