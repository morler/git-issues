package issue

import (
	"fmt"
	"os"

	"github.com/steviee/git-issues/internal/config"
	"github.com/steviee/git-issues/internal/git"
)

type RelationEntry struct {
	Relation string
	ID       int
}

func Inverse(relation string) string {
	switch relation {
	case "blocks":
		return "depends-on"
	case "depends-on":
		return "blocks"
	default:
		return relation // related-to, duplicates are self-inverse
	}
}

func AddRelation(issuesDir string, sourceID int, relation string, targetID int, cfg *config.Config) error {
	if sourceID == targetID {
		return fmt.Errorf("cannot relate issue #%d to itself", sourceID)
	}

	source, err := LoadByID(issuesDir, sourceID)
	if err != nil {
		return err
	}
	target, err := LoadByID(issuesDir, targetID)
	if err != nil {
		return err
	}

	AddToSlice(source, relation, targetID)
	AddToSlice(target, Inverse(relation), sourceID)

	if err := savePair(issuesDir, source, target); err != nil {
		return err
	}

	if cfg.AutoStage {
		_ = git.Stage(source.FilePath)
		_ = git.Stage(target.FilePath)
	}

	return nil
}

func RemoveRelation(issuesDir string, sourceID int, relation string, targetID int, cfg *config.Config) error {
	if sourceID == targetID {
		return fmt.Errorf("cannot unrelate issue #%d from itself", sourceID)
	}

	source, err := LoadByID(issuesDir, sourceID)
	if err != nil {
		return err
	}
	target, err := LoadByID(issuesDir, targetID)
	if err != nil {
		return err
	}

	RemoveFromSlice(source, relation, targetID)
	RemoveFromSlice(target, Inverse(relation), sourceID)

	if err := savePair(issuesDir, source, target); err != nil {
		return err
	}

	if cfg.AutoStage {
		_ = git.Stage(source.FilePath)
		_ = git.Stage(target.FilePath)
	}

	return nil
}

// savePair writes two issues and restores the first one if the second write
// fails, so a relation is never left half-written on disk.
func savePair(issuesDir string, first, second *Issue) error {
	backup, backupErr := os.ReadFile(first.FilePath)

	if err := Save(issuesDir, first); err != nil {
		return err
	}
	if err := Save(issuesDir, second); err != nil {
		if backupErr == nil {
			_ = os.WriteFile(first.FilePath, backup, 0644)
		}
		return err
	}
	return nil
}

func DiffRelations(oldRel, newRel Relations) (added, removed []RelationEntry) {
	diffSlice := func(relation string, oldIDs, newIDs []int) {
		oldSet := make(map[int]bool)
		for _, id := range oldIDs {
			oldSet[id] = true
		}
		newSet := make(map[int]bool)
		for _, id := range newIDs {
			newSet[id] = true
		}
		for _, id := range newIDs {
			if !oldSet[id] {
				added = append(added, RelationEntry{Relation: relation, ID: id})
			}
		}
		for _, id := range oldIDs {
			if !newSet[id] {
				removed = append(removed, RelationEntry{Relation: relation, ID: id})
			}
		}
	}

	diffSlice("blocks", oldRel.Blocks, newRel.Blocks)
	diffSlice("depends-on", oldRel.DependsOn, newRel.DependsOn)
	diffSlice("related-to", oldRel.RelatedTo, newRel.RelatedTo)
	diffSlice("duplicates", oldRel.Duplicates, newRel.Duplicates)

	return
}

func getSlice(iss *Issue, relation string) *[]int {
	switch relation {
	case "blocks":
		return &iss.Relations.Blocks
	case "depends-on":
		return &iss.Relations.DependsOn
	case "related-to":
		return &iss.Relations.RelatedTo
	case "duplicates":
		return &iss.Relations.Duplicates
	default:
		return nil
	}
}

func AddToSlice(iss *Issue, relation string, id int) {
	slice := getSlice(iss, relation)
	if slice == nil {
		return
	}
	for _, v := range *slice {
		if v == id {
			return
		}
	}
	*slice = append(*slice, id)
}

func RemoveFromSlice(iss *Issue, relation string, id int) {
	slice := getSlice(iss, relation)
	if slice == nil {
		return
	}
	var result []int
	for _, v := range *slice {
		if v != id {
			result = append(result, v)
		}
	}
	*slice = result
}
