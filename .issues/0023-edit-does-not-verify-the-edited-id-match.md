---
id: 23
title: edit does not verify the edited id matches the target
status: open
priority: high
labels:
    - bug
created: "2026-11-19"
updated: "2026-11-19"
---

Repro: `issues edit 1`, change the frontmatter to `id: 5`, save.

Result: prints `Updated: #1`, but 0001-*.md now contains `id: 5`. NextID and LoadByID then disagree, and `issues show` returns the wrong issue.

Cause: cmd/edit.go validates the reparsed file but never compares `edited.ID` with the id from the argument. Also validate that the filename ID still matches. Found while adding tests for #12.
