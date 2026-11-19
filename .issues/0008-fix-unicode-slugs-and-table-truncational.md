---
id: 8
title: Fix Unicode slugs and table truncation/alignment
status: closed
priority: medium
labels:
    - bug
created: "2026-11-19"
updated: "2026-11-19"
closed: "2026-11-19"
---

Chinese titles generate an empty slug (0001-.md) because only [a-z0-9 -] survives. Also list/show/labels truncate and pad by bytes, corrupting multi-byte titles. Keep Unicode letters/digits in slugs and use display width for tables.
