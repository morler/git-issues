---
id: 33
title: list silently accepts invalid --status, --priority, --sort and --format
status: open
priority: low
labels:
    - bug
created: "2026-11-20"
updated: "2026-11-20"
---

`issues list` does not validate its flag values:

- `--status foo` → empty result plus `No foo issues found.` (`cmd/list.go:102`, `:165`)
- `--priority foo` → silent empty result (`cmd/list.go:105`)
- `--sort foo` → falls through to the priority sort (`cmd/list.go:151`)
- `--format foo` → falls through to the table (`cmd/list.go:70`)

`issues set` already rejects invalid enum values with `invalid status "foo", must be one of: ...`; `list` should fail the same way before loading issues. A typo currently looks like "no issues match".

This absorbs the older later-note about `--status` being unchecked; the `null` half of that note is fixed by #27.
