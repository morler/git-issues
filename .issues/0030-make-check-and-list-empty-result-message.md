---
id: 30
title: Make check and list empty-result messages accurate
status: closed
priority: medium
labels:
    - bug
created: "2026-11-20"
updated: "2026-11-20"
closed: "2026-11-20"
---

Reproduced with only closed issues in the store:
- `issues list` (default `--status open`) prints "No issues found." although closed issues exist.
- `issues check` prints "No issues found. Database is consistent." whenever there are no consistency findings, even when open issues exist (this repo: 29 issues, 1 open). The message conflates "no findings" with "no issues".

Expected:
- check → "No problems found. Database is consistent."
- list → filter-aware: default open → "No open issues found.", other filters → "No matching issues found.", `--status all` → "No issues found."
