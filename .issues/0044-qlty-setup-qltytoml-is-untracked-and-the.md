---
id: 44
title: qlty config is untailored and not wired into make/CI
status: in-progress
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

## Version control decided
`.qlty/qlty.toml`, `.qlty/.gitignore` and `.qlty/configs/.shellcheckrc` are now committed (78ff512). Qlty's generated `.gitignore` whitelists exactly those three paths and ignores the per-machine cache symlinks (`logs`, `out`, `results`, `plugin_cachedir`) plus `sources/`, so a clean clone runs `qlty check` without `qlty init` - verified in a fresh clone of the repo.

Still open here: the plugin set is untailored (see the list above) and qlty is not part of `make lint` or CI.
