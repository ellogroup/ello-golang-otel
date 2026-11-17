# Tech Debt Register

A structured log of known technical debt in this application. Entries are
created at the point debt is identified — not retrospectively.

Honest entries are more useful than polished ones. The reason a piece of debt
exists is as important as the debt itself.

---

## Summary

| Severity | Open | In Progress | Resolved | Accepted |
|---|---|---|---|---|
| Critical | 0 | 0 | 0 | 0 |
| High | 0 | 0 | 0 | 0 |
| Medium | 0 | 0 | 0 | 0 |
| Low | 0 | 0 | 0 | 0 |

---

## Template

### TD-[NNN] — [Short descriptive title]

**Status:** [Open | In Progress | Resolved | Accepted]
**Severity:** [Critical | High | Medium | Low]
**Category:** [Security | Performance | Architecture | Code Quality | Dependencies | Operational]
**Created:** [DATE]
**Created by:** [Developer name or "AI-assisted — reviewed by [name]"]
**Owner:** [Team or person responsible for resolution]
**Linked ticket:** [Ticket reference or None]

**What is the debt?**
[Clear, specific description of the problem. Not "this could be better" but
what exactly is wrong or missing.]

**Why does it exist?**
[Honest reason — time pressure, unclear requirements, legacy decision, known
shortcut taken deliberately, etc. This is not a blame record — it is context.]

**What is the risk if unaddressed?**
[Concrete impact. Not "it might cause problems" but what specifically breaks,
degrades, or becomes harder as a result of leaving this unresolved.]

**Proposed resolution:**
[What needs to be done to fix it. Does not need to be a full solution — a
direction is sufficient.]

**Effort estimate:** [S — hours | M — days | L — week+ | XL — significant]
**Resolution target:** [Current sprint | This quarter | Next quarter | Backlog | Accepted risk]

---

---

### TD-001 — X-Ray ID generator paired with TraceIDRatioBased sampler

**Raised:** 2026-11-17
**Problem:** Upstream advises against using the X-Ray ID generator with `TraceIDRatioBased`
(timestamp-prefixed IDs may skew sampling). `provider/tracer.go` does this when `WithXRay()` and
`OTEL_SAMPLE_RATE < 1`.
**Proposed resolution:** Evaluate a different sampler when X-Ray is enabled; vendored `internal/xray` is now ours to maintain.
**Effort estimate:** S
**Resolution target:** Backlog

---

### TD-002 — Vendored `internal/xray` copy of a deprecated upstream module

**Raised:** 2026-11-17
**Problem:** `internal/xray` is vendored from `go.opentelemetry.io/contrib/propagators/aws/xray` v1.47.0,
deprecated upstream as abandoned (no code owners; see ADR-002). We own its maintenance. No AWS-maintained
Go replacement was found (ADOT Go docs still reference the upstream package).
**Proposed resolution:** Periodically check whether upstream is un-deprecated/adopted and switch back
(steps in `AGENTS.md` "Vendored code — revisit periodically"). Separately, consider dropping the ID
generator if consumers export via the CloudWatch OTLP endpoint (W3C IDs) rather than the ADOT X-Ray exporter.
**Effort estimate:** S
**Resolution target:** Backlog — recheck at each dependency bump
**Last checked:** 2026-11-17 — still deprecated, no replacement found.

---

### TD-003 — Ported `aws/middleware` core of deprecated upstream `otelaws`

**Raised:** 2026-11-17
**Problem:** `aws/middleware` is a port of `go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws`
v0.72.0, deprecated upstream as abandoned (no code owners; see ADR-002/003). We own its maintenance;
service-specific attribute builders were not ported.
**Proposed resolution:** Periodically check whether upstream is un-deprecated/adopted and switch back
(steps in `AGENTS.md` "Vendored code — revisit periodically"). Consider porting service attributes if
consumers need them.
**Effort estimate:** S
**Resolution target:** Backlog — recheck at each dependency bump
**Last checked:** 2026-11-17 — still deprecated, no replacement found.
