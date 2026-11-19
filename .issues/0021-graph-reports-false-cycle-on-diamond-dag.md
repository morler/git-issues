---
id: 21
title: graph reports false cycle on diamond DAG
status: open
priority: high
labels:
    - bug
created: "2026-11-19"
updated: "2026-11-19"
---

Repro: #1 blocks #2 and #3; #2 and #3 each block #4.

    issues graph

Output contains `Warning: cycle detected at #4, truncating`, although the graph is a diamond DAG and `issues check` reports the database as consistent.

Cause: printNode marks nodes in `visited` and never unmarks them when backtracking, so a second path to the same node is reported as a cycle. Found while adding the command-level tests for #12 (TestGraphCLICycleTruncated must use a real cycle to trigger the warning).
