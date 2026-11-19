---
id: 17
title: Simplify list filter parameter list
status: open
priority: low
labels:
    - refactor
created: "2026-11-19"
updated: "2026-11-19"
---

supercov quality: cmd/list.go 6.2/10, long_parameter_list 0.82. filterIssues takes issue slice plus status, priority, labels, blocksID and dependsOnID. Introduce a filterOptions struct so call sites and tests stay readable.
