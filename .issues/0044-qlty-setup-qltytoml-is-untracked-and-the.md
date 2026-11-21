---
id: 44
title: 'qlty setup: qlty.toml is untracked and the config is untailored'
status: open
priority: low
labels:
    - tooling
    - ci
created: "2026-11-21"
updated: "2026-11-21"
---

.qlty/ currently shows up as untracked in git status, and the auto-generated .qlty/qlty.toml does not match this repo:

- plugins enabled: actionlint, gofmt, golangci-lint (comment), osv-scanner, radarlint-go (monitor), ripgrep, shellcheck, trufflehog, zizmor. actionlint, shellcheck, ripgrep and trufflehog produced 0 findings here; golangci-lint reports 0 (make lint passes with 0 issues).
- gofmt is redundant with make fmt-check, and golangci-lint is already the CI gate (issue #11).
- the 128 radarlint test-file findings and the 179 osv findings dominate the output: a single run yields 336 issues, of which only ~11 are actionable in production code.

Decide whether .qlty/qlty.toml should be committed (upstream recommends committing it), tailor exclude_patterns/[[triage]] so CI-visible findings stay meaningful, and add qlty to the Makefile quality gates or state explicitly that it is a local-only review tool.
