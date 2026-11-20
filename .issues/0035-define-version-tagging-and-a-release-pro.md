---
id: 35
title: Define version tagging and a release process
status: open
priority: low
labels:
    - docs
    - tooling
created: "2026-11-20"
updated: "2026-11-20"
---

The repository has no git tags, so `go install github.com/steviee/git-issues/cmd/issues@latest` (README, install section) resolves to a pseudo-version: no pinning, no rollback, no way to say "1.0".

Pick one and write it down:

- semver `vX.Y.Z` tags, plus a short "Release" section in README and a CHANGELOG, or
- an explicit statement that the tool is pre-release and `@latest` tracks main.

If tags are wanted, `ci.yml` can create them (or at least fail a release PR without a changelog entry). No code changes required for the tag itself.
