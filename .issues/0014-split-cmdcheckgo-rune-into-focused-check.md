---
id: 14
title: Split cmd/check.go RunE into focused checks
status: open
priority: medium
labels:
    - refactor
created: "2026-11-19"
updated: "2026-11-19"
---

supercov quality snapshot q_d9ce71d646aec7f0: cmd/check.go 6.5/10, long_method 0.91, deep_nesting 0.69. Verified RunE spans 111 uncovered lines and mixes duplicate-ID, validation, orphan, symmetry and status checks. Extract each check into a function that returns findings; combine with the command-level tests from #12.
