---
id: 20
title: Fix broken make build after missing cmd/issues entrypoint
status: closed
priority: high
labels:
    - bug
relations:
    related-to:
        - 1
created: "2026-11-19"
updated: "2026-11-19"
closed: "2026-11-19"
---

Makefile builds ./cmd/issues, but no main package exists there; the only entrypoint is root main.go. README, Makefile and CLAUDE.md already reference cmd/issues, so add the entrypoint, drop the duplicate root main.go and fix the website install command to /cmd/issues. Regression from #1.

Root cause: .gitignore had an unanchored `issues` rule, which also ignored the `cmd/issues/` directory, so a fresh clone was missing the entrypoint (make failed with `stat .../cmd/issues: directory not found`). Fixed by anchoring the rule to `/issues`, moving the entrypoint to cmd/issues/main.go and updating the website install command.
