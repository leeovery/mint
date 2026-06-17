AGENT: standards
FINDINGS: none
SUMMARY: Implementation conforms to specification and project conventions. All five acceptance criteria are met, the four AI-seam invariants are preserved (verified by transport tests and the commit isAIFallback errors.Is routing), and all project gates pass (go build, gofmt -l, go vet, go test -race, golangci-lint — 0 issues).

VERIFICATION NOTES (not findings — recorded for traceability):

- AC#1 (StageFailure.Output populated by the settled composition rule): internal/engine/release.go:1622 notesFailureOutput composes the carrier's captured streams stdout-first then stderr, single-newline joined, trailing whitespace trimmed via strings.TrimRightFunc(out, unicode.IsSpace), interior preserved. Emptiness is decided on the trimmed view per stream; both-empty → "". Pinned by notesfailureoutput_internal_test.go (8 cases) and end-to-end by release_priortag_test.go.

- AC#2 (concise top-line Message): internal/engine/release.go:1674 failureMessage derives the concise phrase via notes.CauseText (errors.Is over the %w chain) for the four sentinels; preflight GateError branch ordered first; defensive cause.Error() fallback retained for unmapped causes. The %w chain is left intact (TestFailureMessage_LeavesMatchableChainIntact). assertConcisePhrase pins absence of ':', 'failed', and a leading 'notes' label.

- AC#3 (both surfacing paths render identically): surface (regenerate single-version/interactive, regenerate_interactive.go:207 and the batch deterministic pre-read read failure regenerate_batch.go:271) and surfaceAndUnwind (forward release, release.go:461) both set Output: notesFailureOutput(cause). TestSurface_PopulatesOutputIdenticallyToForwardPath asserts byte-identical Message+Output across the longer abortError chain and the shorter "generating notes: %w" chain.

- AC#4 (padStage gap unchanged): pretty.go StageFailed still uses padStage(s.Name) at line 540/541; no padStage edit. Only the message text in pretty_test.go:TestPrettyPresenterFailedRegenerateSuppressesClose changed ("claude failed" → "AI returned empty/invalid notes after retry"). gate_forbidden_test.go / askline_test.go untouched.

- Out-of-scope boundaries respected: resetAndAbort (regenerate_write.go:353) inherits the concise Message via failureMessage but does NOT call notesFailureOutput (Output empty) — matches spec lines 116-117. Batch per-version production-failure skip (regenerate_batch.go:288) still routes through reportSkip/classifyNotesFailure untouched — matches spec lines 119-122.

- AI-seam invariants (CLAUDE.md seam #5): (1) errors.Is(err, ErrGenerationFailed) preserved via GenerationError.Unwrap() returning the sentinel (transport.go:73); commit's isAIFallback (commit/run.go:791) still matches. (2) context.Canceled passthrough unchanged in classifyFatal (transport.go:262) and pinned carrier-free in TestTransport_Generate_DoesNotRetryCancel and the no-deadline variant. (3) transport stays content-agnostic — no config import; carrier holds raw Stdout/Stderr/ExitCode. (4) single-retry ownership unchanged — carrier packed only after the retry (TestTransport_Generate_InvokesCommandTwice* both shapes). (5) byte-identical success path untouched (TestTransport_Generate_ReturnsValidBodyUnchanged + carrier-leak guard).

- Conventions: GenerationError.Error() is lowercase, no trailing punctuation (transport.go:68); custom-type-carrying-data is the idiomatic choice over a separate return value (matches golang-error-handling best practice #8 and the *hooks.HookError precedent). Type name GenerationError follows the XxxError convention. Comments are WHY-comments kept true to as-built.

- Gates: go build ./... OK; gofmt -l . prints nothing; go vet ./... OK; go test -race ./... all packages ok; golangci-lint run reports 0 issues.
