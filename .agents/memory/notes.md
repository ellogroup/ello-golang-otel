# Notes

Working notes, gotchas, lessons learned, and useful references accumulated
during development of this application.

Entries are append-only within each section. The Scratch section may be
cleared between sessions if it no longer contains relevant content.

---

## Gotchas

Things that caused unexpected problems and how they were resolved. Future
sessions and developers should read this before starting work.

- **[DATE] [Short title]:** [What the problem was, what caused it, how it was
  resolved. Be specific enough that someone encountering the same issue can
  recognise it and know the fix.]

---

## Lessons Learned

Patterns and approaches that proved effective or ineffective. Worth knowing
for next time.

- **[DATE] [Short title]:** [What was learned and why it matters.]

---

## Useful References

Links and resources that are specifically relevant to this application — not
general documentation, but things that were actually useful during development.

- **[Title]:** [URL or reference] — [Why it is useful and in what context.]

---

## Scratch

Temporary working notes from the current or recent session. May be cleared
once no longer relevant.

[Working notes here.]

### 2026-11-17 — /check-security on commit 9c42504 (otel contrib removal)

No Critical/High findings. Notes: (1) `aws/middleware` injects the global propagator's headers
(incl. W3C `baggage` if set) into every AWS request — same as upstream `otelaws`; callers must not put
PII/secrets in baggage. (2) `go-vulncheck` could not be run (not installed; `go run …@latest` was blocked by
the auto-mode classifier) — vulnerability scan of deps is outstanding. (3) Dependencies `aws-sdk-go-v2/service/sqs`
(test-only) and `smithy-go` became direct requires (already approved modules / already transitive).
(4) Ported/vendored deprecated code tracked as TD-002/TD-003.
