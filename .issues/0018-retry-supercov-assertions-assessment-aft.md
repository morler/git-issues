---
id: 18
title: Retry supercov assertions assessment after HTTP 422
status: open
priority: low
labels:
    - tooling
created: "2026-11-19"
updated: "2026-11-19"
---

supercov runs latest assertions assess stopped after 21 failed requests: openrouter.ai HTTP 422. The coverage run itself is complete (run_1e478e611d78d0d4, 79 tests passed), but assertion quality is unknown. Retry later (possibly --refresh or another model); do not wire into CI until stable.
