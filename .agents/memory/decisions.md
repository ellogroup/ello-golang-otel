# Architecture Decision Records

A log of significant technical decisions made during development of this
application. Each entry explains what was decided, why, and what alternatives
were considered.

Entries are append-only — once recorded, an ADR is never deleted. If a
decision is reversed, a new ADR is added with status Superseded and a
reference to the original.

---

## Template

### ADR-[NNN] — [Short descriptive title]

**Date:** [DATE]
**Status:** [Proposed | Accepted | Superseded by ADR-NNN]
**Author:** [Developer name or "AI-assisted — reviewed by [name]"]

**Context:**
[What situation or requirement led to this decision? What constraints existed?
What problem were we solving?]

**Decision:**
[What was decided? Be specific — avoid vague statements like "we chose the
better approach".]

**Rationale:**
[Why was this the right decision given the context? What made it preferable to
the alternatives?]

**Alternatives Considered:**

| Alternative | Reason rejected |
|---|---|
| [Option] | [Why it was not chosen] |

**Consequences:**
[What are the trade-offs? What does this decision make easier or harder in
future? Are there follow-on decisions that will need to be made?]

**Related:**
- [Link to relevant ticket, PR, or other ADR if applicable]

---

### ADR-001 — Adopt goimports local-prefix import grouping, superseding the single-block convention

**Date:** 2026-07-27
**Status:** Accepted
**Author:** AI-assisted (Claude Code) — reviewed by Dave Richards

**Context:**
Retrofitting the Go app template's AI-integration and tooling standards
(runbook: EP space, "Retrofitting the Go App Template") into this
already-established repository. The template's `.golangci.yml` and
`Makefile format` target run `goimports -local github.com/ellogroup`,
which groups `github.com/ellogroup/...` imports into their own block,
separate from stdlib and other third-party imports. This repository's
`CLAUDE.md` previously documented the opposite convention: a single
gofmt-sorted import block with no goimports-style grouping.

**Decision:**
Adopt the template's goimports local-prefix grouping as the standard for
this repository, superseding the previous single-block convention. The
`format` target now runs `goimports -local github.com/ellogroup -w ./` in
addition to `gofmt`/`go fix`, and `.golangci.yml` enables the `goimports`
formatter with the same `local-prefixes` setting.

**Rationale:**
Consistency with every other retrofitted Ello repository was preferred
over preserving this repo's prior one-off convention. The developer
confirmed this trade-off explicitly when asked, rather than silently
reformatting every file.

**Alternatives Considered:**

| Alternative | Reason rejected |
|---|---|
| Keep the existing single-block convention; skip the `goimports` step for this repo only | Would leave this repo permanently out of step with the shared tooling standard and every other retrofitted repo, for a stylistic preference with no functional benefit |

**Consequences:**
The `goimports` reformat (this ADR's decision) will touch most existing
source files as a mechanical import-block reordering with no logic
changes — committed as its own "reformat imports" commit per the runbook.
Any future contributor following the old single-block convention from
memory or an outdated local note should be pointed at this ADR.

**Related:**
- Confluence runbook: "Runbook: Retrofitting the Go App Template (AI Integration + Go Tooling)" (EP space, page 1341063170)

---
