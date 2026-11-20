---
id: 24
title: close, done and batch close ignore open blockers
status: closed
priority: medium
labels:
    - bug
created: "2026-11-19"
updated: "2026-11-20"
closed: "2026-11-20"
---

`issues close <id>` refuses to close an issue with open blockers in non-interactive mode (and warns interactively), but:

- `issues done <id>` (documented as a shortcut for close) sets status closed unconditionally.
- `issues batch close` closes every matching issue without any blocker check.

So an agent that follows the documented workflow (`issues done <id>`) bypasses the blocker guard entirely. Decide whether done/batch close should respect blockers (or require --force) and align the three paths. Found while adding tests for #12.
