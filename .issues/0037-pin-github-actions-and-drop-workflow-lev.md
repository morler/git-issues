---
id: 37
title: Pin GitHub Actions and drop workflow-level pages permissions
status: closed
priority: high
labels:
    - security
    - ci
created: "2026-11-21"
updated: "2026-11-21"
closed: "2026-11-21"
---

qlty check --all (zizmor plugin) reports 11 findings, all verified against the two workflow files.

- unpinned-uses (high) - every `uses:` is a mutable tag: ci.yml:15 checkout@v4, ci.yml:19 setup-go@v5, ci.yml:34 golangci-lint-action@v8, pages.yml:25 checkout@v4, pages.yml:28 setup-node@v4, pages.yml:41 upload-pages-artifact@v3, pages.yml:54 deploy-pages@v4.
- artipacked (medium) - ci.yml:15 and pages.yml:24 checkout steps keep the token in .git/config.
- excessive-permissions (high) - pages.yml:10-11 grants pages:write and id-token:write workflow-wide although only the deploy job signals/needs them.

Fix: pin each action to the commit SHA of the tag in use, add persist-credentials: false to checkout, move pages/id-token permissions into the deploy job.

## Closed
All 11 zizmor findings gone (`qlty check --all --filter zizmor` -> No issues).

- ci.yml: checkout@eef6144 (v4.2.1), setup-go@40f1582 (v5.6.0), golangci-lint-action@4afd733 (v8.0.0), each pinned to the commit the tag pointed at; checkout runs with `persist-credentials: false`.
- pages.yml: same checkout pin + persist-credentials, setup-node@49933ea (v4.4.0), upload-pages-artifact@56afc60 (v3.0.1), deploy-pages@d6db901 (v4.0.5). Workflow-level permissions reduced to `contents: read`; `pages: write` and `id-token: write` moved into the `deploy` job, which is the only job that needs them.
