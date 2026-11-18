TASK: Task 1-2 — Populate the carrier on an empty/whitespace body surviving the retry (notes-failure-output-ugly-and-uninformative-1-2)

ACCEPTANCE CRITERIA:
1. A Generate call returning an empty body (Stdout: "") on both attempts returns a *ai.GenerationError retrievable via errors.As, still errors.Is-matching ErrGenerationFailed.
2. A Generate call returning a whitespace-only body (Stdout: "   \n\t\n") on both attempts returns the carrier; the carrier's Stdout field holds that whitespace verbatim (no transport-side trim).
3. When the body is whitespace-only on stdout but claude wrote a real message on stderr, the carrier's Stderr field holds that message.
4. The command is invoked exactly twice on the empty/whitespace-survives-the-retry case (single-retry ownership unchanged).
5. The non-zero-exit carrier path (Task 1-1) and the empty/whitespace carrier path return the same *ai.GenerationError type — unified, not divergent.
6. All project gates pass.

STATUS: Complete

SPEC CONTEXT:
Fix 1 (the load-bearing fix) carries claude's captured output through the transport so a downstream surfacing site can render the real message instead of a bare sentinel. The empty/whitespace-only body is the OTHER bad-content path alongside the non-zero exit (Task 1-1) — a zero-exit call whose stdout is whitespace-only (or whose informative text landed on stderr) must still carry whatever claude wrote. Invariant 4 requires single-retry ownership to be unchanged: the carrier is populated only AFTER the retry is exhausted, and the transport never trims (whitespace composition is Phase 2's settled rule). Invariant 5 requires the byte-identical success path to stay untouched.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/ai/transport.go
  - Generate, lines 222-232: the `!isValid(res.Stdout)` branch now returns `&GenerationError{Stdout: res.Stdout, Stderr: res.Stderr, ExitCode: res.ExitCode}` instead of the prior bare `return "", ErrGenerationFailed`. Verbatim res.Stdout/res.Stderr — no trim.
  - Generate, lines 216-221: the non-zero-exit branch (Task 1-1) returns the identical `&GenerationError{Stdout, Stderr, ExitCode}` construction — the two bad-content exits are unified onto the SAME type (AC #5).
  - attempt, lines 257-266: returns the whole `runner.Result` plus error; on a clean (zero-exit) attempt the success branch surfaces res so Generate can read res.Stderr/res.ExitCode behind a whitespace stdout (task item (f)). Signature change is internal to the package; Generate's public signature is unchanged.
  - GenerationError.Unwrap (line 85) returns ErrGenerationFailed, so errors.Is still routes the carrier; the doc comment (lines 43-55) and attempt's doc (lines 240-247) were honestly updated to describe BOTH provenances.
  - isValid (lines 300-302) trims only for the emptiness check, never mutating the body — the verbatim-carry guarantee holds.
- Notes: First-attempt whitespace still falls through to the retry unchanged (line 206 returns early only on a GOOD body). The classifyFatal short-circuits (timeout/missing-tool/cancel, lines 201-203 and 212-215) are untouched and return before the carrier path. The ai package imports only `mint/internal/runner` — no config import (Invariant 3 preserved).

TESTS:
- Status: Adequate
- Location: internal/ai/transport_test.go
- Coverage:
  - TestTransport_Generate_CarriesCapturedOutputOnEmptyBodySurvivingRetry (lines 545-566): seeds Result{Stdout: ""}, nil; asserts errors.Is(ErrGenerationFailed), errors.As(&genErr) succeeds, genErr.Stdout == "". Covers AC #1.
  - TestTransport_Generate_CarriesWhitespaceBodyVerbatimSurvivingRetry (lines 568-589): seeds Result{Stdout: "   \n\t\n"}, nil; asserts genErr.Stdout holds the whitespace verbatim. Covers AC #2 and the no-trim guarantee.
  - TestTransport_Generate_CarriesStderrWhenWhitespaceStdoutHidesRealStderr (lines 591-611): seeds Result{Stdout: "   ", Stderr: "Prompt is too long"}, nil; asserts genErr.Stderr == "Prompt is too long". Covers AC #3 and exercises the attempt-success-branch res threading (task item (f)) — this is the test that would fail if attempt discarded res on its nil-error branch.
  - TestTransport_Generate_InvokesCommandTwiceOnEmptyBodySurvivingRetry (lines 613-627): asserts len(r.Invocations()) == 2. Covers AC #4.
  - Existing table-driven TestTransport_Generate_RetriesOnceThenFailsOnBadContent (lines 182-229) was extended with an errors.As(&genErr) assertion (lines 210-215) across all three rows — empty / whitespace-only / non-zero exit — proving the unification (AC #5) at the shared seam while keeping the errors.Is + exactly-two-invocations assertions green.
- Notes: The FakeRunner returns the seeded Result verbatim with a nil error (fake_runner.go RunWith → outcome), which faithfully models a clean zero-exit attempt leaving res fully populated — the precondition the production path relies on. Seeding a real Stderr behind a whitespace Stdout is exactly the previously-untested shape that exposes the defect. No over-testing: each new test pins a distinct facet (empty Stdout, verbatim whitespace, stderr-behind-whitespace, invocation count); the table extension is a single shared assertion, not a duplicate. The dedicated empty-body tests overlap slightly with the table's `empty body` row, but each adds a field-level assertion the table does not (genErr.Stdout == "", invocation count isolated) — justified, not redundant.

CODE QUALITY:
- Project conventions: Followed. External test package (ai_test), t.Parallel() throughout, FakeRunner scripting, exact-value assertions on carrier fields, lowercase error messages with no trailing punctuation (GenerationError.Error). The change reuses the existing carrier type rather than introducing a divergent shape, honouring the unification the plan asked for. No config import (Invariant 3). The success-path carrier-free contract is pinned (line 63-66).
- SOLID principles: Good. The carrier remains a single-responsibility data carrier; attempt returns the whole Result so Generate owns the classification — clean separation.
- Complexity: Low. Generate's bad-content tail is two parallel branches returning the identical construction; no added branching.
- Modern idioms: Yes. errors.As/errors.Is, struct-literal carrier construction.
- Readability: Good. The WHY-comments (transport.go lines 222-232, 240-247) were updated true-to-as-built to describe the dual provenance — no stale scope claims left behind.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
