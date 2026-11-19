---
id: 6
title: Make relation writes atomic and surface new relation errors
status: closed
priority: high
labels:
    - bug
created: "2026-11-19"
updated: "2026-11-19"
closed: "2026-11-19"
---

AddRelation/RemoveRelation write source then target without rollback; issues new ignores AddRelation errors so relations silently vanish when a target ID is missing. Roll back on partial failure, pre-validate targets, and return errors.
