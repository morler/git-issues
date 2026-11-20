---
id: 34
title: check --fix records fixes before the save can fail
status: closed
priority: medium
labels:
    - bug
    - quality
created: "2026-11-20"
updated: "2026-11-20"
closed: "2026-11-20"
---

`checkOrphans` and `checkSymmetry` call `fixedf` while they mutate the issue, but the write happens much later in `saveFixes` (`cmd/check.go:213`). If `issue.Save` fails, the report prints both `✓ #2: added missing depends-on #1` under "Fixed" and `✗ #2: failed to save fix: ...` under "Errors" — the run claims work it did not do.

Record the fix in `saveFixes` after a successful `issue.Save`, or collect pending fixes and flush them.

Related: `dirty` is keyed by issue ID (`cmd/check.go:54`), so with duplicate IDs both files sharing that ID are written and staged even though only one appears in `byID`. Key pending saves by `FilePath`.

## Closed
Fixes are collected as pendingFix while checking and reported under "Fixed" only after issue.Save succeeds; pending writes are keyed by FilePath, so the second file of a duplicate ID is no longer rewritten. The unused checkReport.dirty field is gone. Tests cover a read-only target (only the save error is reported) and an untouched duplicate-ID file.
