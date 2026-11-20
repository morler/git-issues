---
id: 25
title: batch set accepts invalid fields and values when nothing matches
status: closed
priority: medium
labels:
    - bug
created: "2026-11-19"
updated: "2026-11-20"
closed: "2026-11-20"
---

Repro: `issues batch set priority urgent --status closed` with no matching issues prints `0 issues updated.` and exits 0.

Cause: cmd/batch.go validates field and value inside the per-issue loop, so with zero matches the validation never runs and a typo in a scripted call passes silently. Validate field and value once before iterating. Found while adding tests for #12.
