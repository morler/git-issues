---
id: 12
title: Add command-level tests for unchecked cmd RunE paths
status: closed
priority: high
labels:
    - test
created: "2026-11-19"
updated: "2026-11-19"
closed: "2026-11-19"
---

supercov run run_1e478e611d78d0d4: lines 32.01% (533/1665), branches 25.14%, MC/DC 24.76%. Verified unexecuted RunE paths: cmd/check.go 111 lines, cmd/workflow.go 96, cmd/new.go 85, cmd/batch.go 81, cmd/list.go 73, cmd/set.go 62, cmd/show.go 57, cmd/edit.go 53, cmd/next.go 55. Existing tests cover only pure helpers (filterIssues/openBlockers/printNode). Add command-level tests with a temp .issues directory and rootCmd.SetArgs; target 60%+ line coverage. Follow-up to closed #2.

## Closed
cmd package line coverage went from 12.8% to 97.6% (per-file: new 96.8%, list 99.1%, show 98.2%, set 100%, workflow 100%, batch 100%, check 99.1%, graph 100%, edit 100%, init 78.6%, next 98.2%, relate/unrelate/labels/reopen 100%). Test suite: 183 tests in the cmd package (was 8), 0 skips, all pass under -race and -shuffle=on.

Shared harness in cmd/cli_test.go: newCLIRepo (temp dir + .issues/ + t.Chdir), seed/load helpers, run (in-process rootCmd.SetArgs with stdout/stderr capture and full flag reset between runs), and runCLISubprocess (test binary re-entry via TestMain) for paths that call os.Exit.

Bugs found while writing the tests are filed as #21-#27; #28 is a tooling follow-up. Board TUI RunE (tui.NewModel/tea program) stays uncovered because it needs a TTY.
