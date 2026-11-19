---
id: 28
title: add pre-commit hook or CI step that fails on gofmt drift
status: open
priority: low
labels:
    - ci
created: "2026-11-19"
updated: "2026-11-19"
---

CI already runs gofmt and golangci-lint, but the local Makefile lint target only runs vet + golangci-lint. Adding gofmt to `make lint` (or a pre-commit hook) would catch formatting drift before the push. Not blocking; purely developer convenience.
