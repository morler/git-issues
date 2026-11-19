---
id: 1
title: Fix broken install docs and binary name split
status: closed
priority: critical
labels:
    - bug
    - docs
created: "2026-11-19"
updated: "2026-11-19"
closed: "2026-11-19"
---

README recommends 'go install github.com/steviee/git-issues@latest' then calls 'issues', but go install produces 'git-issues'. Makefile installs 'issues', SKILL.md insists on 'git-issues'. Unify on 'issues': add cmd/issues entrypoint, update README, Makefile and skill.
