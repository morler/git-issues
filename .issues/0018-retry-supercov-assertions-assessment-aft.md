---
id: 18
title: Retry supercov assertions assessment after HTTP 422
status: open
priority: low
labels:
    - tooling
created: "2026-11-19"
updated: "2026-11-20"
---

supercov runs latest assertions assess stopped after 21 failed requests: openrouter.ai HTTP 422. The coverage run itself is complete (run_1e478e611d78d0d4, 79 tests passed), but assertion quality is unknown. Retry later (possibly --refresh or another model); do not wire into CI until stable.

## Retry 2026-11-20

Re-measured the current checkout and retried the assessment.

- Fresh run `run_794ec2fe98d35e89`: 280 tests passed, lines 85.69% (1527/1782), branches 79.98%, MC/DC 71.13%.
- `assertions assess` with the configured model (`liquid/d1.2` via openrouter.ai) still stops after 21 failed requests with HTTP 422. `--workers 2` and repeated runs fail identically.
- The key and model work for quality requests (a live `quality --refresh` call succeeded). The official TypeSafe endpoint rejects the key (HTTP 401), so OpenRouter is the only route.
- Scoped assessments succeed while prompts stay small: `internal/git`, `internal/tui`, `internal/issue`, `internal/config`, `cmd/board_cli_test.go`, `cmd/check_test.go`, `cmd/graph_test.go`.
- Deterministic failures: `cmd/batch_cli_test.go`, `cmd/check_cli_test.go`, `cmd/edit_cli_test.go`. A single test fails on its own (`--title TestBatchCloseDefaultsToOpenOnly`, ~21.5k input tokens).
- `liquid/lfm-40b` returns HTTP 400 "larger than the model accepts" for the same prompt, so this is a model context limit, not flakiness. `--refresh` cannot help because failing questions have no cached answer.
- `openai/gpt-4o-mini` is rejected by the systemone route (HTTP 400 even for a 2.2k-token quality prompt).

Conclusion: the full assertion assessment is blocked by the configured provider; assertion prompts for the CLI tests exceed the model's context. Do not wire into CI. Retry when a larger-context model is available.
