---
id: 9
title: Fix TUI card truncation on multi-byte titles
status: closed
priority: medium
labels:
    - bug
created: "2026-11-19"
updated: "2026-11-19"
closed: "2026-11-19"
---

internal/tui/card.go truncates titles by byte, cutting UTF-8 in half. Truncate by display width.
