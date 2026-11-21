---
id: 39
title: Update website npm dependencies to clear osv-scanner and npm audit findings
status: closed
priority: high
labels:
    - security
    - docs
created: "2026-11-21"
updated: "2026-11-21"
closed: "2026-11-21"
---

qlty check --all (osv-scanner) reports 92 findings against website/package-lock.json; `npm audit --package-lock-only` in website/ agrees with 16 vulnerable packages (2 critical, 11 high, 1 moderate, 2 low).

Critical:
- astro@6.2.1 (direct dependency, latest is 7.4.13): XSS via unescaped spread attribute names, reflected XSS via view-transition names, RCE through AVIF image optimization, image-endpoint arbitrary file read. Vulnerable ranges top out at <=7.4.6, so the fix is astro >=7.4.13.
- next@16.2.4: only present in the lockfile as an unsatisfied peer of geist@1.7.0 (`npm ls next` -> empty). geist is imported nowhere in src/ - the fonts are loaded from the jsdelivr CDN in src/styles/global.css - so dropping the geist dependency removes this entry entirely.

High: vite@7.3.2, js-yaml@4.1.1, postcss@8.4.31+8.5.13, devalue@5.8.0, sharp@0.34.5, svgo@4.0.1, zod@4.4.1, nanoid@3.3.12, smol-toml@1.6.1, source-map-js@1.2.1, browserslist@4.28.2.

Constraints: the astro fix is a major upgrade (6 -> 7) of the marketing site, so `npm ci && npm run build` has to be re-verified in website/; the repo currently ships no node_modules, so the upgrade was not attempted. Also consider that osv-scanner keeps re-reporting the lockfile until the lockfile itself moves.

## Closed
Before: osv-scanner 92 findings in website/package-lock.json, `npm audit` 16 packages (2 critical, 11 high, 1 moderate, 2 low).

After: osv-scanner 0 findings, `npm audit` 0 vulnerabilities.

- astro 6.2.1 -> 7.4.13, @astrojs/react 5.0.4 -> 7.1.9, @tailwindcss/vite 4.2.4 -> 4.3.5. This pulls vite 8.3.8 (postcss 8.5.33), sharp 0.35.5 and drops the vulnerable transitive set (devalue, svgo, js-yaml, nanoid, smol-toml, source-map-js, zod, browserslist).
- Removed the `geist` dependency: nothing in src/ imported it (the fonts come from the jsdelivr CDN in src/styles/global.css) and it dragged in next@16.2.4 as an unsatisfied peer, which accounted for the critical next advisories and baseline-browser-mapping.
- package.json engine raised to `node >=22.23.4`, the floor astro 7 declares.
- Astro 7's stricter template parser rejected the bare `<id>` placeholders in the AgentWorkflow terminal mock ("Expected corresponding JSX closing tag for 'id'"); they are now `&lt;id&gt;`.

Verified by a clean `npm ci && npm run build` (1 page, 0 errors) and a rendered-HTML diff against the astro 6 build: identical class set (1223 class tokens, no diff), identical ids except Astro's generated island ids, identical CSS selector set apart from the astro-scope hash suffix, +591 bytes from the escaping change. The preview server was stopped afterwards.
