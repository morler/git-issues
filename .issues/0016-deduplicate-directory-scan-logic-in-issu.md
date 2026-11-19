---
id: 16
title: Deduplicate directory scan logic in issue store
status: open
priority: low
labels:
    - refactor
created: "2026-11-19"
updated: "2026-11-19"
---

supercov quality: internal/issue/store.go duplicated_logic 0.80. Verified LoadAll (lines 31-54) and BrokenFiles (63-84) repeat the .md/dotfile directory scan. Extract one scan helper that yields entries and parse errors, preserving LoadAll's warning behavior.
