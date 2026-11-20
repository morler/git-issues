---
id: 32
title: show reports any load error as issue not found
status: closed
priority: medium
labels:
    - bug
created: "2026-11-20"
updated: "2026-11-20"
closed: "2026-11-20"
---

`cmd/show.go:33` drops the error from `issue.LoadByID` and always returns `issue #%d not found`. A permission error, unparsable frontmatter, or any I/O failure on `.issues/` is therefore reported as a missing issue, hiding the real cause.

`internal/issue/store.go:106` has no sentinel, so `show` cannot tell the cases apart. Add `issue.ErrNotFound` (wrapped by `LoadByID`), then branch with `errors.Is` in `show` and surface every other error as-is.

The relation section (`cmd/show.go:100`) already prints `(not found)` for failed lookups; keep that behaviour, it is about dangling references.

## Closed
Added the issue.ErrNotFound sentinel: LoadByID now wraps it in "issue #N not found", so show returns that error unchanged for a missing ID and reports anything else as "load issue #N: <cause>". Covered by a store unit test (sentinel + read-failure case) and a CLI probe with an unreadable .issues/ directory.
