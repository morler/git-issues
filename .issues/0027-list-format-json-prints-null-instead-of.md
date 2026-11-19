---
id: 27
title: list --format json prints null instead of [] when nothing matches
status: open
priority: low
labels:
    - bug
created: "2026-11-19"
updated: "2026-11-19"
---

Repro: `issues list --status all --priority critical --format json` on a database without critical issues prints `null`.

filterIssues returns a nil slice for no matches, so JSON consumers have to handle null where an empty array is expected. Initialize the result slice or substitute [] before marshalling. Found while adding tests for #12.
