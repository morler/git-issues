---
id: 43
title: 'qlty radarlint test-file style noise: duplicated literals and test names'
status: closed
priority: low
labels:
    - quality
    - test
created: "2026-11-21"
updated: "2026-11-21"
closed: "2026-11-21"
---

qlty check --all reports 128 style-only findings confined to tests:

- 122x radarlint-go:go:S1192 (S1192 duplicated literals) across 20 *_test.go files, e.g. cmd/list_cli_test.go:108 "--status" 16x, cmd/batch_cli_test.go:65 "in-progress" 14x, internal/issue/relations_test.go:52 "2026-03-04" 34x, cmd/edit_cli_test.go:143 "Keep me" 8x. The repository intentionally keeps table-driven CLI tests literal and readable, so hoisting every flag and fixture string would hurt more than help.
- 6x radarlint-go:go:S100 in internal/issue/id_test.go (lines 40, 52, 68, 82, 102, 142): radarlint wants ^_[a-zA-Z0-9]+$ test names, i.e. TestNextID_EmptyDirectory -> TestNextIDEmptyDirectory. This is the only test file in the repo using the underscore suffix convention; renaming it makes the file consistent with the other 566 tests but breaks the grep-by-prefix habit.

Recommendation: triage as accepted rather than fix - either rename the 6 id_test functions for consistency and add a [[triage]] block in .qlty/qlty.toml setting radarlint-go:go:S1192 to mode = "monitor" for **/*_test.go, or exclude the rule for test patterns. No functional risk either way.

## Closed
Both halves are resolved by triage rather than by rewriting the tests:

- The 6 S100 hits are gone: the `TestNextID_*` functions in internal/issue/id_test.go were renamed to `TestNextID*`, matching the other 566 tests in the repo.
- The 124 S1192 and 9 S3776 hits are all in `*_test.go` and are demoted to `low` by two `[[triage]]` blocks in `.qlty/qlty.toml`, with a comment explaining why (table-driven tests are literally meant to repeat flag names and expected messages; complexity is a production-code signal). Production files still report both rules at full level.

`qlty check --all --fail-level high` exits 0; the 133 remaining findings are all `low`.
