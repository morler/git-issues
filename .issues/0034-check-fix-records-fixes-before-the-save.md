---
id: 34
title: check --fix records fixes before the save can fail
status: open
priority: medium
labels:
    - bug
    - quality
created: "2026-11-20"
updated: "2026-11-20"
---

`checkOrphans` and `checkSymmetry` call `fixedf` while they mutate the issue, but the write happens much later in `saveFixes` (`cmd/check.go:213`). If `issue.Save` fails, the report prints both `✓ #2: added missing depends-on #1` under "Fixed" and `✗ #2: failed to save fix: ...` under "Errors" — the run claims work it did not do.

Record the fix in `saveFixes` after a successful `issue.Save`, or collect pending fixes and flush them.

Related: `dirty` is keyed by issue ID (`cmd/check.go:54`), so with duplicate IDs both files sharing that ID are written and staged even though only one appears in `byID`. Key pending saves by `FilePath`.
