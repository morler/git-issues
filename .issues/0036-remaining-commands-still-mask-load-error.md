---
id: 36
title: Remaining commands still mask load errors as issue not found
status: closed
priority: medium
labels:
    - bug
created: "2026-11-20"
updated: "2026-11-20"
closed: "2026-11-20"
---

#32 fixed the pattern in `show`: `issue.LoadByID` now wraps `issue.ErrNotFound`, so callers can distinguish a missing issue from an I/O failure. The same blind re-wrap still exists in:

- `cmd/close.go:46`
- `cmd/edit.go:42`
- `cmd/new.go:177`
- `cmd/reopen.go:39`
- `cmd/set.go:52`
- `cmd/workflow.go:43`, `:85`, `:142`

Each turns every `LoadByID` error into `issue #N not found`, so a permission error or unreadable `.issues/` is reported as a missing issue. `LoadByID` already words a missing issue exactly that way, so `if err != nil { return err }` preserves the message and surfaces real failures; the relation lookups in `edit.go:111,124` and `close.go:130` should be checked for the same masking.

`cmd/show.go` shows the intended shape.

## Closed
Every direct LoadByID call site now returns the store error unchanged (close, edit, new, reopen, set, claim, done): a missing issue keeps its "issue #N not found" wording, any other failure surfaces. Relation lookups follow the same rule: openBlockers and edit's sync loops skip only ErrNotFound (dangling reference, reported by check) and propagate read failures; openBlockers gained a test for the unreadable directory. cmd/workflow.go:142 is a map lookup after a successful LoadAll, so "not found" there is accurate and stays.

Reachability note: close/edit/reopen/set/claim/done read .issues/.config.yml first, so an unreadable .issues/ already fails there (verified: permission denied) and the old masks were only reachable if the store became unreadable mid-command. The change makes that path correct and the error contract uniform.
