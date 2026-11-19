---
id: 15
title: Refactor TUI board model responsibilities
status: open
priority: medium
labels:
    - refactor
created: "2026-11-19"
updated: "2026-11-19"
---

supercov quality: internal/tui/board.go 5.8/10 (lowest in repo), long_method 0.76, magic_values 0.74, god_class 0.64, temporary_field 0.60; internal/tui/column.go magic_values 0.83. Model owns columns, watcher, detail viewport, error state and status bar. Extract rendering/submodels and name the width/height/timeout constants. Do not change behavior; watcher lifecycle and UTF-8 truncation were already fixed in 2eff42e.
