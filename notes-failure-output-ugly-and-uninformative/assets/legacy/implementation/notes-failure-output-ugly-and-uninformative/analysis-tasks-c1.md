---
topic: notes-failure-output-ugly-and-uninformative
cycle: 1
total_proposed: 2
---
# Analysis Tasks: notes-failure-output-ugly-and-uninformative (Cycle 1)

## Task 1: Consolidate duplicated abort-chain test helpers and concise-phrase assertions in the engine internal-test package
status: approved
severity: medium
sources: duplication

**Problem**: The engine internal-test package carries the same test construct authored three times across three task boundaries. `resolveAbortAround` (notesfailureoutput_internal_test.go:32-39), `wrapAsAbort` (notesfailurewiring_internal_test.go:75-82), and `resolveAbortError` (failuremessage_internal_test.go:27-34) have byte-identical bodies — each is a `t.Helper()` that calls `notes.ResolveFailure(t.Context(), runner.NewFakeRunner(), <arg>, "v1.0.0", config.Release{OnNotesFailure: "abort"})`, fatals on a nil error, and returns the err; even the doc comment is identical. Only the function name and the parameter label differ. The magic literals `"v1.0.0"` and `OnNotesFailure: "abort"` must now be kept in sync across three sites, and a future change to `ResolveFailure`'s signature touches all three identically — copy-paste drift waiting to happen. Separately, the three-part concise-message guard (no `:`, no `"failed"`, no leading `"notes"` prefix) — the literal encoding of Acceptance Criterion #2 — appears verbatim twice in failuremessage_internal_test.go (lines 63-71 inside the table loop of `TestFailureMessage_CollapsesForwardAbortChain`, and lines 90-98 as standalone statements in `TestFailureMessage_CollapsesRegenerateShortChain`), so a change to the rule must be edited in both places.

**Solution**: Collapse the three byte-identical abort-chain helpers into a single shared helper in the engine internal-test package, and extract the duplicated concise-phrase assertion triplet into a single shared assertion helper. Pure de-duplication of existing test code — no new behaviour, no production change.

**Outcome**: The abort-chain wrapping shape and its magic literals live in exactly one place; the AC#2 concise-phrase rule is pinned by exactly one helper. A future `ResolveFailure` signature change or a tightening of the concise-phrase rule edits one site, not three/two. All existing engine tests still pass and assert the same behaviour.

**Do**:
1. Add a single shared abort-chain helper to the engine internal-test package — either a new `engine_failure_testhelpers_test.go`, or hoist into whichever of the three existing files reads most naturally. Signature like `func wrapNotesAbort(t *testing.T, cause error) error`, with the one consolidated doc comment describing the abort-chain shape ("...wraps ... through abortError (\"notes generation failed (%s): %w\") ... The git runner is never invoked in abort mode."). Body is the existing shared body: `t.Helper()`, call `notes.ResolveFailure(t.Context(), runner.NewFakeRunner(), cause, "v1.0.0", config.Release{OnNotesFailure: "abort"})`, fatal on nil error, return err.
2. Replace the three call-site helpers (`resolveAbortAround`, `wrapAsAbort`, `resolveAbortError`) with calls to the new shared helper, and delete the three duplicated definitions and their duplicated doc comments.
3. Add a single shared assertion helper for the concise phrase, e.g. `func assertConcisePhrase(t *testing.T, got string)`, holding the three checks (no `:`, no `"failed"`, no `"notes"` leading prefix) with their `t.Errorf` messages.
4. Call `assertConcisePhrase` from both `TestFailureMessage_CollapsesForwardAbortChain` (replacing the inline triplet in the table loop) and `TestFailureMessage_CollapsesRegenerateShortChain` (replacing the standalone triplet).
5. Run the gates.

**Acceptance Criteria**:
- Exactly one abort-chain wrapping helper exists in the engine internal-test package; the literals `"v1.0.0"` and `OnNotesFailure: "abort"` and the abort-chain doc comment appear in exactly one place.
- The three former helpers (`resolveAbortAround`, `wrapAsAbort`, `resolveAbortError`) are gone, with all former callers routed through the shared helper.
- The concise-phrase three-part assertion exists in exactly one helper, called from both `TestFailureMessage_CollapsesForwardAbortChain` and `TestFailureMessage_CollapsesRegenerateShortChain`.
- No production (non-test) source files change.
- All project gates pass: `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues).

**Tests**:
- The consolidation is itself test code; correctness is proven by the existing engine internal tests continuing to pass unchanged in behaviour (same fatal-on-nil and same concise-phrase guards), under `go test -race ./internal/engine/...`.

## Task 2: Add an end-to-end release test that proves captured AI output reaches StageFailure.Output through the real transport→generator→resolve→surface chain
status: approved
severity: medium
sources: architecture

**Problem**: The load-bearing payoff of this fix is that claude's captured output, composed by the transport, actually reaches `StageFailure.Output` (and the concise `Message`) through the full release spine. Every new test exercises the seam in ISOLATION: the wiring/output tests hand-construct an `*ai.GenerationError` and either call `notesFailureOutput`/`failureMessage` directly or wrap it through `ResolveFailure` (wrapAsAbort) before invoking `surfaceAndUnwind`/`surface`. None drive the production chain `transport.Generate` (packs carrier) → `generator.generateFromDiffWithContext` ("generating notes: %w", generate.go:185) → `SelectBody`/`ResolveFailure` (abortError, resolve.go:101) → `surfaceAndUnwind`. The one test that does run the whole chain end-to-end over the real production transport (`TestRelease_PriorTag_NotesFailureAbort_AbortsBeforeMutation`, release_priortag_test.go:682-715, with `Transport` nil) seeds an EMPTY stdout (`runner.Result{Stdout: ""}`) so there is no output to carry, and asserts only that a `StageFailed` event was recorded — it never inspects `StageFailure.Output` or `Message`. In production the carrier sits TWO wraps deep (abortError → "generating notes: %w" → carrier), whereas the white-box `wrapAsAbort` builds a ONE-wrap chain. `errors.As` is depth-agnostic so the helper is correct, but a regression in the generator's wrap layer, the `SelectBody` plumbing, or the seeding contract (the exact gap the spec calls out: "the prior fakes had no stdout to lose") would be caught by no test. This is the integration seam the spec's Testing Requirements flagged: "the gap the existing tag/push-only presenter test left uncovered; assert at the wiring level, not just the presenter."

**Solution**: Extend the existing release-level test (or add a sibling alongside it) that drives `engine.Release` with the real production transport (`Transport` nil) and seeds claude with a non-zero exit carrying stdout, then asserts the recorded `StageFailed` payload carries both the composed `Output` and the concise `Message`. This closes the loop from transport capture to rendered Output across the real seam composition the white-box tests deliberately bypass. No production change — test addition only.

**Outcome**: A single end-to-end test proves that AI stdout captured by the production transport survives the two-wrap production chain and lands in `StageFailure.Output`, with the concise top-line `Message` set, so a regression anywhere in the generator wrap layer, `SelectBody` plumbing, or the seeding contract fails a test.

**Do**:
1. In release_priortag_test.go (reusing the existing fake-seeding harness), add a test (or extend a sibling of `TestRelease_PriorTag_NotesFailureAbort_AbortsBeforeMutation`) that drives `engine.Release` with the real production transport — leave `Transport` nil so `newDeps` builds the production transport.
2. Seed the claude subprocess to fail with captured stdout on BOTH attempts (the transport packs the carrier only after the single retry is exhausted), e.g. `runner.Result{Stdout: "Prompt is too long", ExitCode: 1}` for both attempts via the fake's sequence seeding.
3. Keep the abort-mode config so the failure routes through `abortError` → `surfaceAndUnwind` (the production two-wrap chain), and assert the recorded `StageFailed` payload has `Output == "Prompt is too long"` and `Message == "AI returned empty/invalid notes after retry"`.
4. Keep (or assert) the existing abort-before-mutation guarantee so the new test does not weaken the prior coverage.
5. Run the gates.

**Acceptance Criteria**:
- A release-level test drives `engine.Release` with the production transport (`Transport` nil), seeds claude to fail with non-empty stdout on both attempts, and asserts the recorded `StageFailed` payload has `Output` equal to the seeded stdout and `Message` equal to the concise post-retry phrase ("AI returned empty/invalid notes after retry").
- The assertion runs through the real `transport.Generate` → generator → `SelectBody`/`ResolveFailure` → `surfaceAndUnwind` chain (no hand-constructed `*ai.GenerationError`, no direct call to `notesFailureOutput`/`failureMessage`).
- The existing abort-before-mutation coverage is preserved (the new test does not delete or weaken `TestRelease_PriorTag_NotesFailureAbort_AbortsBeforeMutation`'s guarantees).
- No production (non-test) source files change.
- All project gates pass: `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues).

**Tests**:
- This task is itself the test. It must FAIL if the generator's "generating notes: %w" wrap layer, the `SelectBody` carrier plumbing, or the stdout-seeding contract regresses, and PASS against the current as-built chain.
