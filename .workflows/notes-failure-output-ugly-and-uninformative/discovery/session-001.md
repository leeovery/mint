# Discovery Session 001

Date: 2026-06-17
Work unit: notes-failure-output-ugly-and-uninformative

## Description (as of session)

Fix `mint release` notes-failure rendering: surface the AI's real error
verbatim, collapse the redundant top-line message, and settle the
failure-line (padStage) layout.

## Seed

- seeds/2026-06-14-notes-failure-output-ugly-and-uninformative.md (inbox:bug)

## Imports

(none)

## Map State at Start

(n/a — single-topic work)

## Exploration

Originated from an inbox bug captured while dogfooding `mint release` on
mint itself. When AI notes generation fails, the failure line reads
`✗ notes      notes generation failed (AI returned empty/invalid notes
after retry): generating notes: ai generation failed` — three problems
bundled into one coherent failure-rendering defect:

1. **The real cause is discarded.** The transport (`internal/ai/transport.go`)
   maps a non-zero exit or empty body to `ErrGenerationFailed` and drops
   claude's captured stdout/stderr; the notes layer
   (`internal/notes/resolve.go`, `internal/notes/generate.go`) re-wraps it
   into a redundant chain, and `StageFailure.Output` ends up empty — so the
   operator never sees claude's actual message (e.g. `Prompt is too long`).
2. **The top-line message is redundant** — "failed" appears three times and
   restates the stage name. Desired: collapse to one concise sentence that
   does not restate the stage or repeat "failed".
3. **The failure line is visually ugly** — `padStage` alignment in
   `internal/presenter/pretty.go` (`StageFailed`) puts "notes" twice across a
   wide gap. The gap is intentional column alignment shared with `✓` success
   lines and is pinned in presenter tests, so changing it is a contract
   change to be settled deliberately (keep the gap for failures, or drop it),
   with the pinned tests updated to match.

Shape settled as a **bugfix**: present-broken behaviour, known reproduction,
specific symptoms, root cause already half-traced in the report. One coherent
topic — the three threads are facets of the same failure-rendering path, not
separate features. Investigation is the next phase (confirm the cause trace
before speccing the fix).

A possibly-related sub-issue raised in the bug text — the line-based
`max_diff_lines` guard (default 50000) did not catch an 8.6k-line but ~867 KB
byte-dense diff, suggesting a byte/token-aware ceiling — was **deliberately
left out of scope** for this work unit (it's an enhancement to a guard, not
part of how a failure is rendered). It stays noted in the seed text and can be
promoted to its own work later if desired.

## Edits

(none)

## Topics Identified

(none)

## Conclusion

(none)
