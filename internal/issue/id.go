package issue

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var filenameIDPattern = regexp.MustCompile(`^(\d+)-`)

// IDFromFilename extracts the issue ID from the {id:04d}-{slug}.md convention.
func IDFromFilename(name string) (int, bool) {
	match := filenameIDPattern.FindStringSubmatch(name)
	if match == nil {
		return 0, false
	}
	id, err := strconv.Atoi(match[1])
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func NextID(issuesDir string) (int, error) {
	entries, err := os.ReadDir(issuesDir)
	if err != nil {
		return 0, err
	}

	maxID := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, ".") {
			continue
		}
		// The filename is the fallback source of truth: a corrupted file must
		// still reserve its ID so IDs are never reused.
		if id, ok := IDFromFilename(name); ok && id > maxID {
			maxID = id
		}
		issue, err := ParseFile(filepath.Join(issuesDir, name))
		if err != nil {
			continue
		}
		if issue.ID > maxID {
			maxID = issue.ID
		}
	}

	return maxID + 1, nil
}
