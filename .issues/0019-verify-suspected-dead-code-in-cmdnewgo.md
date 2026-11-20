---
id: 19
title: Verify suspected dead code in cmd/new.go
status: wontfix
priority: low
labels:
    - quality
created: "2026-11-19"
updated: "2026-11-20"
closed: "2026-11-20"
---

supercov quality file cmd/new.go reports dead_code 0.70 (file-level, no line evidence). Manual review did not find clearly unreachable code; candidates are openEditor's returned relation slices and the editor template. Confirm whether anything is unused; close as wontfix if this is a false positive.

## Closed
False positive: deadcode ./... reports no unreachable function in cmd/new.go. openEditor's returned values and editorTemplate are all used, and openEditor is 92.9% covered by the CLI tests. The supercov dead_code 0.70 judgment is file-level with no line evidence.
