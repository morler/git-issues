---
id: 4
title: Prevent issue ID reuse when files are corrupted
status: closed
priority: critical
labels:
    - bug
created: "2026-11-19"
updated: "2026-11-19"
closed: "2026-11-19"
---

LoadAll silently skips unparsable files and NextID reads IDs only from frontmatter, so a corrupted highest-ID file makes NextID reuse that ID. Derive IDs from filenames as fallback, warn on broken files, and report them in issues check.
