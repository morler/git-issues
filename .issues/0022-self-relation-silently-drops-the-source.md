---
id: 22
title: Self-relation silently drops the source-side relation
status: closed
priority: high
labels:
    - bug
created: "2026-11-19"
updated: "2026-11-20"
closed: "2026-11-20"
---

Repro: `issues relate 1 blocks 1`.

Output claims `Related: #1 blocks #1` plus the auto inverse, but the file ends up with only `depends-on: [1]`; the `blocks` entry is lost.

Cause: AddRelation loads source and target into two distinct objects that share one FilePath; savePair writes source and then target to the same path, so the second write overwrites the first. Reject self-relations (sourceID == targetID) or save only once. Found while adding tests for #12.
