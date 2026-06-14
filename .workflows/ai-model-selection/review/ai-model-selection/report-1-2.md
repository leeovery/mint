TASK: Define the typed closed verb enum (release, commit; no regenerate) — ai-model-selection-1-2

ACCEPTANCE CRITERIA:
- An exported `Verb` type exists in `internal/config` with exactly two exported constants (`VerbRelease`, `VerbCommit`).
- There is no `regenerate` (or other third) enum value.
- A test enumerates the two values and asserts no additional reachable member maps to a distinct table (no unknown/zero-value verb silently distinct from the two).
- The type's WHY-comment records the no-regenerate rationale and the exhaustive-by-construction guarantee.
- `go build ./...`, `gofmt -l .`, `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues) all pass.

STATUS: Complete

SPEC CONTEXT:
specification.md ("Single source of truth for config defaults") mandates the `verb` parameter to the layered accessors be a typed, closed enum defined in `internal/config` — not a raw string — with exactly two values, one per verb table ([release], [commit]). The typed enum gives compile-time safety against string-typo fall-through to the shared baseline and makes the regenerate routing un-missable: with NO `regenerate` value, `internal/engine/regenerate_fresh.go` can only pass the release verb. The accessor domain is therefore exhaustive by construction — no "unrecognized verb" branch. regenerate is not a separate verb (it re-runs the release-notes task and resolves through [release]); the verb config space is exactly two tables. This task defines only the type + constants + a closed-set test; the accessors arrive in 1-4 / 1-7 (and are already present here).

IMPLEMENTATION:
- Status: Implemented
- Location: internal/config/verb.go:21 (`type Verb int`), :27 (`VerbRelease Verb = iota`), :29 (`VerbCommit`). Consumed by accessors at internal/config/config.go:538 (AICommandFor), :596 (TimeoutFor).
- Notes:
  - Exactly two exported constants, `VerbRelease` and `VerbCommit`; underlying type is unexported `int` (`type Verb int`), so callers must use a named constant — a raw int is not assignable without explicit conversion. Matches the task's "named int over unexported underlying representation."
  - No `regenerate` (or any third) value anywhere — confirmed by grep across internal/config, internal/engine, internal/commit. No `[regenerate]` enum constant exists.
  - Zero value is VerbRelease (iota-0), a REAL verb table, never a silent unknown/empty member. The accessors treat "any value != VerbCommit" as release, so the zero value resolves exhaustively to [release] with no unrecognized-verb branch — exactly the exhaustive-by-construction guarantee the spec requires.
  - WHY-comment (verb.go:3-20) records both required rationales verbatim: the no-regenerate routing ("regenerate_fresh.go can only pass VerbRelease") and the exhaustive-by-construction / no-unrecognized-verb-branch guarantee, plus the compile-time typo-safety motivation. True to as-built.
  - Scope discipline respected: this task adds ONLY the type, constants, and test. The accessors (AICommandFor/TimeoutFor) that consume the type land in 1-4/1-7 and were not introduced by this task.

TESTS:
- Status: Adequate
- Coverage: internal/config/verb_test.go covers all three task-listed test intents:
  - TestVerb_ClosedSet (:14) — asserts VerbRelease and VerbCommit are distinct (no collision into one table). ("it defines exactly two verb values, release and commit")
  - TestVerb_NoRegenerateValue (:28) — enumerates the closed set as a 2-element slice, asserts len == 2, and asserts no member aliases another (distinct underlying values). ("it has no regenerate verb value" — by enumeration; the comment notes the slice must grow if a third is added, forcing a reviewer to confront the rule.)
  - TestVerb_ZeroValueIsRealVerb (:54) — pins `var zero Verb` == VerbRelease, locking the chosen representation's zero value to a real verb table. (Conditional test was applicable since `type Verb int` has a meaningful zero value; correctly included.)
- Notes:
  - Tests are behaviour/contract-focused (distinctness, closed cardinality, zero-value identity), not implementation minutiae. No redundant assertions, no unnecessary setup or mocking. All t.Parallel(), external test package (config_test) — consistent with project idioms.
  - The "closed set" is necessarily expressed as a hand-maintained slice (Go has no enum reflection), so the test cannot mechanically detect a smuggled third exported constant; it relies on the documented convention that adding a verb forces editing the slice. This is the standard Go idiom for closed-enum pinning and is acceptable — no stronger mechanism exists without code generation. No action needed.

CODE QUALITY:
- Project conventions: Followed. Heavy WHY-comments stating the contract/invariant (no-regenerate, exhaustive-by-construction, zero-value-is-real) per CLAUDE.md. Comments are true to as-built. File placement (small sibling verb.go in package config) matches the task's allowance and the project's one-concern-per-file layout.
- SOLID principles: Good. Single, focused responsibility (the verb domain); the type is the seam the accessors depend on.
- Complexity: Low. A named int type and two iota constants — no branches.
- Modern idioms: Yes. Idiomatic Go closed enum (named type over int + iota). Deliberate, documented divergence from golang-naming's "place an Unknown/Invalid sentinel at iota 0": the spec REQUIRES the zero value be a real verb (no third/unknown member), and the WHY-comment justifies it (the skill explicitly permits overriding a rule with a comment). Not a violation.
- Readability: Good. Intent is self-evident from names and the type-level WHY-comment.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
