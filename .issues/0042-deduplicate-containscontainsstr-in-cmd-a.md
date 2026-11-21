---
id: 42
title: Deduplicate contains/containsStr in cmd and hoist repeated string literals
status: closed
priority: medium
labels:
    - refactor
    - quality
created: "2026-11-21"
updated: "2026-11-21"
closed: "2026-11-21"
---

qlty check --all (radarlint-go) reports 53 findings in non-test files; 5 of them are duplicated-literal (S1192) hits in production code and one is a duplicate implementation (S4144):

- cmd/set.go:130 containsStr is byte-identical to contains at cmd/set.go:121 (S4144). Both walk a []string; labels are a []string, so one function is enough. Replace the containsStr call sites in cmd/set.go:79 and cmd/batch.go:177 and delete the copy.
- cmd/edit.go:117 "failed to sync relation with #%d: %w" repeated 4x - hoist to a const.
- cmd/new.go:23-24 "depends-on" / "related-to" repeated 3x each - the relation names already exist as flag names and as issue.Relations yaml tags; introduce named constants for the four relation strings and use them in cmd/new.go, cmd/relate.go's validRelations and internal/issue.
- cmd/workflow.go:46 "in-progress" repeated 4x, internal/issue/relations.go:19 "depends-on" repeated 4x - same constants.

Not in scope here: the remaining 122 S1192 hits are in *_test.go files (mostly flag names and expected-output strings in table-driven tests) and the 6 S100 hits ask for underscore-free test names in internal/issue/id_test.go; both are style-only and are tracked separately.

## Closed
- Deleted `containsStr` from cmd/set.go; the two call sites (cmd/set.go, cmd/batch.go) use `contains`.
- Added `StatusOpen`/`StatusInProgress`/`StatusClosed`/`StatusWontfix` and `RelationBlocks`/`RelationDependsOn`/`RelationRelatedTo`/`RelationDuplicates` to internal/issue, used by ValidStatuses, cmd/new.go (flags and relation sync loop, now table-driven), cmd/relate.go's validRelations, cmd/workflow.go, internal/issue/relations.go (Inverse, DiffRelations, getSlice) and cmd/list.go's open-status checks.
- Hoisted `errSyncRelation` in cmd/edit.go for the four "failed to sync relation with #%d: %w" sites.

radarlint reports no S1192/S4144 finding in non-test Go files any more; `go build`, `go vet`, `make lint` and `go test -race ./...` pass.
