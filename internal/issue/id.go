package issue

import (
	"regexp"
	"strconv"
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
	scan, err := scanIssues(issuesDir)
	if err != nil {
		return 0, err
	}

	maxID := 0
	for _, name := range scan.names {
		// The filename is the fallback source of truth: a corrupted file must
		// still reserve its ID so IDs are never reused.
		if id, ok := IDFromFilename(name); ok && id > maxID {
			maxID = id
		}
	}
	for _, iss := range scan.issues {
		if iss.ID > maxID {
			maxID = iss.ID
		}
	}

	return maxID + 1, nil
}
