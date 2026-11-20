---
id: 29
title: Fix OCR review findings on unpushed changes
status: closed
priority: medium
labels:
    - bug
created: "2026-11-20"
updated: "2026-11-20"
closed: "2026-11-20"
---

Review session afc346e6 (origin/main..HEAD) found: fmt-check false pass for untracked files/git failures; git.Stage over-resolves symlinked .issues and fails open on EvalSymlinks errors, and its error contract is inconsistent; TUI Init loses the watcher and reload drops LoadAll errors; batch close scans the directory per dependency and refuses when the blocker is closed by the same batch; edit does not restore the file when the id becomes invalid (id: 0); check scans the issues directory twice (LoadAll + BrokenFiles). Fix each with probes and tests.

## Closed
All verified findings fixed: fmt-check includes untracked files, handles non-git trees and never invokes gofmt without args; git.Stage now takes issuesDir, resolves both sides (symlinked .issues works, look-alikes rejected, failures fail closed) and documents its error contract; TUI persists the watcher via watcherMsg in Init and surfaces reload errors; batch close builds one ID map, skips blockers closed by the same batch and still refuses unselected open blockers; edit checks the id before Validate so an invalid id restores the file; check loads issues and broken files in one pass via LoadWithBroken. Tests and E2E probes cover each fix.
