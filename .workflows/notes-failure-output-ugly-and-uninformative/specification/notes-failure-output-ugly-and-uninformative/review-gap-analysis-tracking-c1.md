---
status: complete
created: 2026-06-17
cycle: 1
phase: Gap Analysis
topic: notes-failure-output-ugly-and-uninformative
---

# Review Tracking: Notes Failure Output Ugly and Uninformative - Gap Analysis

All findings verified against the codebase before applying (failureMessage, causeText, abortError, resetAndAbort, hookFailureOutput, pretty.go StageFailed, transport.go attempt/Generate/classifyFatal, commit run.go isAIFallback). Applied under finding_gate_mode=auto.

## Findings

### 1. Fix 2's concise-`Message` mechanism is under-specified — the spec names no single seam to change

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Fix 2; Acceptance Criteria #2; Scope & Affected Surfaces

**Details**:
Fix 2 said the surfacing path "derives the concise display phrase" but never named WHICH function. As built, all three engine StageFailed sites funnel their display string through `failureMessage(cause)` in `internal/engine/release.go` (used by `surface`, `surfaceAndUnwind`, `resetAndAbort`); the verbose chain is built by `abortError` in `internal/notes/resolve.go`. Implementer faced an unresolved a/b/c fork.

**Resolution**: Approved
**Notes**: Verified `failureMessage` is the single funnel (release.go:1615) with an existing `*preflight.GateError` → `gate.Message()` branch. Added a "Seam (settled here)" paragraph to Fix 2 choosing **option (a)** — extend `failureMessage` to derive the concise phrase for notes/AI failures via an exported `causeText` equivalent / `Message()` method, mirroring the GateError branch — over (b) stripping abortError's prefix (changes errors.Is/log text) and (c) a new display-only error type. Notes this single seam covers forward chain, regenerate's shorter chain, and resetAndAbort.

---

### 2. A third surfacing site (`resetAndAbort` in regenerate_write.go) shares `failureMessage` but is never mentioned

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Scope & Affected Surfaces; Fix 1 step 3; Acceptance Criteria #3

**Details**:
`internal/engine/regenerate_write.go` `resetAndAbort` (line 352) also builds `StageFailure{Name, Message: failureMessage(cause)}` directly for the changelog-regenerate record/push path. Spec scoped to only two helpers, undercounting the StageFailure builders.

**Resolution**: Approved
**Notes**: Verified resetAndAbort's causes are git "record"/"push"/"batch rebuild" failures (regenerate_write.go:289,342; regenerate_batch_changelog.go:150), never the AI carrier. Added a "Third StageFailed site — resetAndAbort" paragraph to Scope: out of scope for Output population (cause is never the AI carrier), but inherits the concise Message for free via failureMessage.

---

### 3. `mint commit` is claimed to "benefit" but its AI failure never reaches a `StageFailed`/`Output` render

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Fix 1 ("Why transport-level"); Scope & Affected Surfaces (first paragraph)

**Details**:
A `mint commit` AI transport failure routes to the `$EDITOR` fallback (`isAIFallback`/`runEditorFallback`), never to `StageFailed`. So the carrier's captured output has no rendering site on the commit path; "all three benefit" overstates the user-facing effect.

**Resolution**: Approved
**Notes**: Verified `isAIFallback` (run.go:791) matches ErrGenerationFailed/ErrTimeout/ErrCommandMissing → runEditorFallback. Reworded Scope first paragraph and "Why transport-level": the shared seam means the carrier must preserve commit's errors.Is fallback routing; commit's rendered output is unchanged (no commit-side rendering/tests in scope). Release/regenerate gain the rendered Output.

---

### 4. The carrier error's shape, exported surface, and accessor are unspecified

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Fix 1 step 1; Invariants #1, #3; Testing Requirements (Transport)

**Details**:
What does the carrier hold (Result, combined string, or separate fields)? How does the engine extract it (errors.As through the abortError %w wrap)? `hookFailureOutput` reads `.Stderr`, but claude's payload is on STDOUT — a literal mirror renders nothing. stdout vs stderr composition undefined.

**Resolution**: Approved
**Notes**: Verified `attempt` returns `res.Stdout` and discards `res` on err (transport.go:199-203); `hookFailureOutput` reads `Result.Stderr` (release.go:1594). Updated Fix 1 step 1 to specify distinct Stdout/Stderr (+optional ExitCode) fields, stdout-first composition; rewrote step 3 to mirror the hookFailureOutput *pattern* but NOT its field choice — `errors.As` traverses the %w chain, reads stdout (stderr when stdout empty), with an explicit warning against copying the `.Stderr` read literally.

---

### 5. Output for the timeout / command-missing / diff-too-large causes is undefined

**Source**: Specification analysis
**Category**: Edge case within scope
**Affects**: Fix 1; Acceptance Criteria; Testing Requirements

**Details**:
`causeText` maps four causes; the spec centers only on generation-failed. Timeout/command-missing short-circuit via classifyFatal; ErrDiffTooLarge never reaches the transport. Their Output behaviour is unstated.

**Resolution**: Approved
**Notes**: Verified classifyFatal short-circuits Timeout/CommandMissing (transport.go:210-229) and ErrDiffTooLarge originates in CheckDiffSize. Added a "Per-cause Output behaviour" subsection to Fix 1: only generation-failed carries Output; the others render the concise phrase with empty Output (✗ line alone).

---

### 6. "Does not restate the stage name / does not repeat 'failed'" — `causeText`'s phrases are not verified against the rule

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Fix 2; Acceptance Criteria #2; Testing Requirements (Concise message)

**Details**:
The endorsed phrase "AI returned empty/invalid notes after retry" contains "notes" — the stage name — so a literal reading of "does not restate the stage name" is self-contradicting. Acceptance test cannot be written.

**Resolution**: Approved
**Notes**: Tightened the rule in Fix 2 and Acceptance #2: the Message must not begin with / duplicate the stage label as a leading prefix (no "notes notes…") and must not contain "failed"; an incidental "notes" inside the cause phrase is allowed. The test asserts absence of leading-label duplication and of "failed", not absence of the substring "notes".

---

### 7. The stdout copy of the failure line is not addressed for the captured body

**Source**: Specification analysis
**Category**: Edge case within scope
**Affects**: Acceptance Criteria #1; Testing Requirements

**Details**:
`StageFailed` writes the ✗ summary to both out and err, but the captured body to OUT ONLY. The spec should make the stream split explicit so the test author does not assert the body on stderr.

**Resolution**: Approved
**Notes**: Verified pretty.go StageFailed: `p.writef(...)` + `p.errf(...)` for the summary, `writeNotesBody(p.out, s.Output)` for the body (pretty.go:540-545); doc comment confirms body is "out only ... NEVER duplicated to err". Added a "Stream split (as-built)" note to Testing Requirements: body renders to stdout only; the new wiring test asserts Output population, not the stream.

---

### 8. The exact expected `pretty_test.go` assertion deltas are described only by reference

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Fix 3 (Test impact); Testing Requirements (Updated tests)

**Details**:
The worked example shows a two-space gap, whereas `padStage` pads "notes" to a column width — the example may not match real padded output and could mislead the test author.

**Resolution**: Approved
**Notes**: Verified `padStage(s.Name)` column padding (pretty.go:540). Added a "Note on the worked-example spacing" to Fix 3: examples use illustrative two-space spacing; the real line uses padStage column padding (gap preserved); the updated assertion replaces only the message text after the padded stage.

---
