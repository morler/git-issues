---
id: 40
title: Reduce cognitive complexity of printTable and filterIssues
status: closed
priority: high
labels:
    - refactor
    - quality
created: "2026-11-21"
updated: "2026-11-21"
closed: "2026-11-21"
---

qlty check --all (radarlint-go:go:S3776) flags two production functions in cmd/list.go:

- cmd/list.go:174 printTable - cognitive complexity 38 (limit 15). The function mixes header printing, label formatting, the blocks-open scan, the blocked-by-open scan and the legend decision in one loop body.
- cmd/list.go:98 filterIssues - cognitive complexity 16 (limit 15), marginal: five sequential filter conditions.

Extract the relation-marker computation (blocksOpen / blockedBy -> string) into a helper that returns the marker string plus a bool, and split the legend/row printing from the filter loop. Behaviour must not change: the table layout is asserted by cmd/list_cli_test.go, and the existing empty-result message logic (issue #30) stays as is.

## Closed
`f` (cmd/list.go) now splits into `printTable` (header, row loop, legend), `formatLabels`, `relationMarker` (returns the ⬜/⬛ column plus whether any marker printed), `openIDs` (filters relation IDs down to open/in-progress targets) and `formatIDList`; `filterIssues` delegates the five predicates to `matchesFilters`. radarlint reports no S3776 for cmd/list.go any more.

Output is unchanged: `go test ./cmd/` passes, including the table assertions in list_cli_test.go.
