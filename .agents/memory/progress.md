# Progress

**Last Updated:** 2026-07-27

---

## Current Status

The Go app template AI-integration/tooling retrofit (Confluence runbook,
EP space page 1341063170) is complete on branch `feature/BE-651-dep-bump`,
verified clean end-to-end, and ready for PR.

---

## Completed

- Added `.ai-context` submodule (git@github.com:ellogroup/ai-context.git).
- Added root AI-integration files: `CLAUDE.md` (generic pointer, copied
  verbatim from `ello-golang-http-app-template`), `AGENTS.md` (adapted —
  see below), `.aiignore`, `.markdownlint.yml`.
- Added `.claude/settings.json` permission guardrails (verbatim); added
  `.claude/settings.local.json` to `.gitignore`.
- Copied `scripts/sync-skills.sh` and `scripts/init-memory.sh`; wired
  `ensure-ai-context`/`sync-ai-context`/`sync-skills`/`init-memory`
  Makefile targets; ran both once (4 skill wrappers generated under
  `.claude/skills/`, memory files seeded under `.agents/memory/`).
- Added root `.golangci.yml` adapted from the template's `app/.golangci.yml`
  (this repo has no `app`/`test` module split) — kept `sloglint` and
  `spancheck` enabled since this library genuinely wraps `slog.Handler`
  and OTel spans, unlike the runbook's own worked example.
- Updated `Makefile`: `build` depends on `ensure-ai-context`; `format`
  runs `goimports -local github.com/ellogroup`; `static-tests` runs
  `golangci-lint config verify` before `run`.
- Reformatted all imports with `goimports` (mechanical, no logic
  changes) — see ADR-001 in `decisions.md` for why this repo switched
  away from its old single-block import convention.
- Fixed all 5 pre-existing lint findings surfaced by the new config
  (3× missing GoDoc in `internal/default/conv.go`, 2× unchecked type
  assertion in `slog/handler/handler_test.go`'s mock). Confirmed 0 issues
  with `--max-same-issues=0 --max-issues-per-linter=0`.
- Reviewed README.md against current source — no API drift found despite
  recent dependency bumps. Found and fixed a real pre-existing gap: the
  old `CLAUDE.md` never listed the `aws/middleware/` package even though
  README already documented it; added it to `AGENTS.md`'s package table.
  Added a new README "Development" section documenting the AI tooling and
  the full Makefile command list (previously only partially documented in
  the old `CLAUDE.md`, which is now a generic pointer).
- Full verification pass green: `make sync-skills`, `make init-memory`
  (both idempotent on rerun), `make format` (no residual diff), `make
  static-tests` (golangci-lint/gosec/govulncheck all clean), `make
  unit-tests` (all pass), `make build` (clean).

---

## In Progress

- Nothing in progress — retrofit complete, awaiting PR review.

---

## Next

- Open a PR for `feature/BE-651-dep-bump` against `main`.
- After merge, no other ello-golang-* library repos were touched by this
  session — this retrofit covers `ello-golang-otel` only.

---

## Blockers

- None.

---

## Session Log

### 2026-07-27 — AI-context/tooling retrofit

Ran the full 8-step runbook (submodule, root AI files, `.claude/`
guardrails, skill wrappers + memory seed, linter config, Makefile,
lint-finding fixes, README review) as 9 small commits. Biggest judgement
call: this repo is a shared Go library (single module at repo root, no
`app/`/`infrastructure/`/`test/` module), unlike the runbook's worked
example which was an app — every template step needed adapting rather
than copying verbatim; see `AGENTS.md` "This repository" for the
adaptations. Also logged ADR-001: the developer explicitly chose to
adopt the template's `goimports -local` grouping over this repo's prior
documented single-block import convention, after being asked directly
about the conflict.
