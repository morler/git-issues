---
id: 41
title: Reduce cognitive complexity of the TUI file watcher debounce loop
status: closed
priority: high
labels:
    - refactor
    - quality
created: "2026-11-21"
updated: "2026-11-21"
closed: "2026-11-21"
---

qlty check --all (radarlint-go:go:S3776) flags internal/tui/watcher.go:44 waitForChange - cognitive complexity 20 (limit 15).

The nested select-in-for-in-for handles events, closed channels and the drain of queued events. Extract the drain loop into its own function and return early on closed channels so the debounce path stays flat. Covered by internal/tui tests; the board RunE is not covered, so keep the change mechanical.

## Closed
`waitForChange` in internal/tui/watcher.go now returns early for irrelevant file ops and hands the debounce drain to a new `drainEvents` helper. radarlint no longer reports S3776 for the file; `go test ./internal/tui/` passes.
