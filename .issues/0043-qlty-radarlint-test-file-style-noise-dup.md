---
id: 43
title: 'qlty radarlint test-file style noise: duplicated literals and test names'
status: open
priority: low
labels:
    - quality
    - test
created: "2026-11-21"
updated: "2026-11-21"
---

qlty check --all reports 128 style-only findings confined to tests:

- 122x radarlint-go:go:S1192 (S1192 duplicated literals) across 20 *_test.go files, e.g. cmd/list_cli_test.go:108 "--status" 16x, cmd/batch_cli_test.go:65 "in-progress" 14x, internal/issue/relations_test.go:52 "2026-03-04" 34x, cmd/edit_cli_test.go:143 "Keep me" 8x. The repository intentionally keeps table-driven CLI tests literal and readable, so hoisting every flag and fixture string would hurt more than help.
- 6x radarlint-go:go:S100 in internal/issue/id_test.go (lines 40, 52, 68, 82, 102, 142): radarlint wants ^_[a-zA-Z0-9]+$ test names, i.e. TestNextID_EmptyDirectory -> TestNextIDEmptyDirectory. This is the only test file in the repo using the underscore suffix convention; renaming it makes the file consistent with the other 566 tests but breaks the grep-by-prefix habit.

Recommendation: triage as accepted rather than fix - either rename the 6 id_test functions for consistency and add a [[triage]] block in .qlty/qlty.toml setting radarlint-go:go:S1192 to mode = "monitor" for **/*_test.go, or exclude the rule for test patterns. No functional risk either way.
