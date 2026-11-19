---
id: 12
title: Add command-level tests for unchecked cmd RunE paths
status: open
priority: high
labels:
    - test
created: "2026-11-19"
updated: "2026-11-19"
---

supercov run run_1e478e611d78d0d4: lines 32.01% (533/1665), branches 25.14%, MC/DC 24.76%. Verified unexecuted RunE paths: cmd/check.go 111 lines, cmd/workflow.go 96, cmd/new.go 85, cmd/batch.go 81, cmd/list.go 73, cmd/set.go 62, cmd/show.go 57, cmd/edit.go 53, cmd/next.go 55. Existing tests cover only pure helpers (filterIssues/openBlockers/printNode). Add command-level tests with a temp .issues directory and rootCmd.SetArgs; target 60%+ line coverage. Follow-up to closed #2.
