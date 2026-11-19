---
id: 7
title: Handle close on non-interactive stdin with open blockers
status: closed
priority: high
labels:
    - bug
created: "2026-11-19"
updated: "2026-11-19"
closed: "2026-11-19"
---

issues close reads stdin for confirmation; in scripts/CI EOF is treated as 'no' but exits 0, so callers think the close succeeded. Detect non-TTY, exit 1 unless a new --force flag is passed.
