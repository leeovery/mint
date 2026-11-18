# Consolidation Findings: headline-first-release-notes (Phase 1)

## Findings

### F1: The default prompt restates the maintenance stub as a second literal instead of composing it from stubBody
- **Class**: near-miss
- **Failure**: `stubBody` is the empty-diff maintenance body, and its doc invites retuning ("kept a package-level constant so the wording is tunable in one place"). Task 1-8 put the same wording into `DefaultPrompt` as its own literal: the AI's maintenance line is asked for "in the shape of" it. If someone retunes `stubBody` the way its doc invites, the AI path keeps asking for the old line. A release with nothing users meet then ships one maintenance line and an empty-diff release ships another, on the tag, the CHANGELOG and the provider release. Nothing catches it: `degenerate_test.go:95` pins the stub with its own literal, and `prompt_test.go:644` pins the prompt with its own. The only way anyone notices is two differently worded maintenance entries in a CHANGELOG.
- **Evidence**: `internal/notes/degenerate.go:5-13` (the `stubBody` const and its "tunable in one place" doc), `internal/notes/degenerate.go:45-46` (`StubBody()` returns it), `internal/notes/prompt.go:181-185` (the `DefaultPrompt` bullet with the copied literal at line 183). The test pins `internal/notes/degenerate_test.go:95` and `internal/notes/prompt_test.go:644` each keep their own literal and need no change.
- **Proposed shape**: Build the bullet's quote from the constant: `` in the shape of "` + stubBody + `". That line `` breaks the raw string around it, the same way `prompt.go:60` already splices in the `ai_command` example. Both values are untyped string constants, so `DefaultPrompt` stays a const and its bytes stay identical. `TestDefaultPrompt_AsksForMaintenanceLineWhenNothingUsersMeet` stays green, and the "tunable in one place" claim on `stubBody` becomes true again.

## Comment Corrections

These are the confirmed bank entry from the 1-11 reviewer. The phase changed the Change Map from a ranking signal to a coverage check. `changemap.go`'s own docs now say "first section", "readable form" and "no false novelty line", but these older test comments still call novelty the headline signal and the rollup salience-preserving. Each OLD block is tab-indented inside its test function body. The test names and the `t.Errorf`/`t.Fatalf` message strings in the same tests are left alone: names are never a finding, and message strings are code, not comment text.

- internal/notes/changemap_test.go:96-99 — calls the new package "the strongest headline signal" and novelty PRIMARY "in emphasis", but the map no longer ranks or picks the headline. What is left (novelty lists before a larger area's churn) is the test name and the index assertion below it.
  OLD: // Structural novelty is PRIMARY: a brand-new auth/ package (every path added)
       // is the strongest headline signal and must lead the map — ABOVE a larger-
       // magnitude change in an existing area (api/, only modified, 400 churned lines
       // vs auth/'s 30). Novelty over magnitude in both ordering and emphasis.
  NEW:
- internal/notes/changemap_test.go:122 — calls the novelty line a "headline", and otherwise restates the two `strings.Index` checks beside it.
  OLD: // The novelty headline for auth/ must appear before the api/ magnitude line.
  NEW:
- internal/notes/changemap_test.go:169-172 — "SECONDARY supporting context" and "the salience-preserving form" are the ranking language that `renderMagnitude`'s doc dropped in this phase ("the second section", "the readable form"). The rest restates the fixture with hand-traced totals.
  OLD: // Magnitude is SECONDARY supporting context: per-file churn rolls up to
       // directory/area granularity, ranked by total churn descending. A flat per-file
       // list is mush — rollup is the salience-preserving form. Here api/ (410) must
       // rank above util/ (12), which ranks above cmd/ (3).
  NEW:
- internal/notes/changemap_test.go:268-271 — "NO false novelty headline" calls a novelty line a headline; `newDirectories`' doc now says "no false novelty line". The rest restates the assertion and the heuristic documented on `newDirectories`.
  OLD: // When EVERY change is a modification within ONE existing area (no A/D/R, the
       // area is merely touched), there must be NO false novelty headline — only the
       // magnitude rollup for that area. The novelty heuristic (a dir is novel only when
       // all its paths are added) must NOT fire here.
  NEW:
- internal/notes/halvebound_test.go:111-112 — a hand-traced size that the phase's prompt growth made false. The fixed part is the 116,840-byte context plus `DefaultPrompt`, which grew from about 2.8 KB to about 12.3 KB, so it is now about 130,000 bytes. The function name already says the fixed part is outsized, and each caller guards its own geometry with a `fixture:` check.
  OLD: // outsizedFixedPart is a scenario whose operator context gives every call a fixed
       // part of about 120,000 bytes.
  NEW:
