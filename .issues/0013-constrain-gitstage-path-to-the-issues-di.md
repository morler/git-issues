---
id: 13
title: Constrain git.Stage path to the issues directory
status: open
priority: high
labels:
    - security
created: "2026-11-19"
updated: "2026-11-19"
---

supercov security snapshot q_f1ac57574e46da09 flags internal/git/stage.go: injection_sink 0.81, path_from_input 0.68 (file-level, 0 confirmed lines). Verified: exec.Command uses separate args so there is no shell injection, but filePath is only implicitly confined to .issues/. Harden Stage by resolving the absolute path and rejecting anything outside the issues directory before git add. Non-actionable flags from the same snapshot: missing_authorization in workflow/reopen/close/edit/set (local CLI has no auth model) and path_from_input in issue.go (internal callers only).
