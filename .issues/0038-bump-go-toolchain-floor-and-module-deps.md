---
id: 38
title: Bump Go toolchain floor and module deps to clear reachable govulncheck findings
status: closed
priority: medium
labels:
    - security
    - tooling
created: "2026-11-21"
updated: "2026-11-21"
closed: "2026-11-21"
---

qlty check --all (osv-scanner plugin) reports 87 findings against go.mod; 3 are reachable from our code per govulncheck (go run golang.org/x/vuln/cmd/govulncheck@latest ./...):

- GO-2026-5970 | golang.org/x/text@v0.24.0 infinite loop on invalid input | fixed in v0.39.0 | reachable: issue.LoadAll -> fmt.Fprintf -> norm.Form.Properties (internal/issue/store.go:39).
- GO-2026-5320 | github.com/yuin/goldmark@v1.7.8 XSS | fixed in v1.7.17 | reachable: tui.renderMarkdownBody -> glamour renderer (internal/tui/view.go:138).
- GO-2025-3595 | golang.org/x/net@v0.33.0 incorrect neutralization | fixed in v0.38.0 | reachable: tui.renderMarkdownBody -> html.Tokenizer.Next.

The remaining 84 osv findings come from the `go 1.25.0` directive: osv-scanner scans stdlib@1.25.0, and the whole 1.25 line has no fixed release for the reported GO-2026-* sets (fixed versions are 1.26.9-1.26.11 / 1.27.2+). Bump the go directive to a fixed patch release, update the README minimum, and refresh x/net, x/text, x/sys and goldmark. Verified: stdlib@1.26.11, x/net@v0.63.0, x/text@v0.44.0, x/sys@v0.50.0 and goldmark@v1.7.17 all return 0 OSV entries for the Go ecosystem.

## Closed
Before: osv-scanner 87 findings against go.mod, govulncheck reported 3 reachable vulnerabilities.

After: osv-scanner 0 findings against go.mod, `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` -> "No vulnerabilities found."

- `go 1.25.0` -> `go 1.26.11` (the 1.25 line has no fixed release for the reported GO-2026 set). README and CLAUDE.md updated to match.
- golang.org/x/net v0.33.0 -> v0.63.0, golang.org/x/text v0.24.0 -> v0.44.0, golang.org/x/sys v0.38.0 -> v0.50.0, github.com/yuin/goldmark v1.7.8 -> v1.7.17; golang.org/x/term came along to v0.48.0 via `go mod tidy`.

`go build ./...`, `go vet ./...`, `make lint` (0 golangci-lint issues) and `go test -race ./...` all pass. The go directive now requires every contributor's toolchain to be >= 1.26.11, which is stated in both README and CLAUDE.md.
