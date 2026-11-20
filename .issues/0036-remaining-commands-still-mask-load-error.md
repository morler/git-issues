---
id: 36
title: Remaining commands still mask load errors as issue not found
status: open
priority: medium
labels:
    - bug
created: "2026-11-20"
updated: "2026-11-20"
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
