---
id: 31
title: 'list table: long label column overflows into the relation column'
status: open
priority: low
labels:
    - bug
    - quality
created: "2026-11-20"
updated: "2026-11-20"
---

`cmd/list.go:240` pads the label column with `padRight(labelsStr, 14)`, but `padRight` (`cmd/text.go:13`) only pads and never truncates. Labels wider than 14 cells run straight into the relation markers, e.g. `[bug, auth, feature]⬛ 2`.

Reproduced while reviewing issue #12; the title column already uses `truncateDisplay`, so apply the same helper to the label column, or widen LABELS and keep a guaranteed gap before `relStr`.
