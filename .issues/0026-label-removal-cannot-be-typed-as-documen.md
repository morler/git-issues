---
id: 26
title: label removal cannot be typed as documented
status: open
priority: medium
labels:
    - bug
created: "2026-11-19"
updated: "2026-11-19"
---

CLAUDE.md, .agent.md, SKILL.md and README document `issues set 7 label -bug`, but cobra parses `-bug` as a flag:

    $ issues set 7 label -bug
    Error: unknown shorthand flag: 'b' in -bug

`issues batch set label -bug` fails the same way. The removal only works as `issues set 7 label -- -bug`.

Fix by disabling interspersed flag parsing for these commands (Flags().SetInterspersed(false)) or by updating the docs to document the `--` form. Found while adding tests for #12.
