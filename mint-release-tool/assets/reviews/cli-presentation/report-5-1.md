TASK: cli-presentation-5-1 — Extract shared byte-purity ASCII-scan test helper

ACCEPTANCE CRITERIA:
- Exactly one definition of the byte-purity ASCII-scan exists (the helper); no inline copy remains in any of the nine sites.
- The two multi-stream sites (plain_test.go, gate_forbidden_test.go) assert both out and err via assertBytePureASCIIStreams.
- Each call retains its original per-site context string in failure output.
- `go test ./internal/presenter/...` and `go vet ./...` pass with no behavioural change.

STATUS: Complete

SPEC CONTEXT: Phase 5 (Analysis Cycle 1) DRY-consolidation task. The byte-purity contract — plain output is pure printable ASCII + newline, no ESC/CR — is a production invariant (glyphs/ANSI are pretty-only). Test-only consolidation; no production behaviour change. Contract lives in the test guards.

IMPLEMENTATION:
- Status: Implemented
- Location: helper bytepurity_test.go:15 (assertBytePureASCII), :36 (assertBytePureASCIIStreams); 9 call sites: plain_test.go:476,:519,:717,:790,:902; gate_skip_test.go:138; init_test.go:108; gate_sourcetarget_test.go:189; gate_forbidden_test.go:69.
- Notes: All nine inline scan loops replaced with one helper call each; grep finds exactly one definition. Both multi-stream sites call assertBytePureASCIIStreams asserting both streams. Each call carries its original context string. Other 0x1b matches in the package are distinct ESC-presence checks or an ANSI-strip helper, not copies of the full scan — correctly out of scope. Helper in package presenter_test; t.Helper() set in both.

TESTS:
- Status: Adequate (test-only consolidation; implementation and tests are the same code)
- Coverage: nine existing byte-purity guards preserved, routed through the shared helper; four-arm scan (ESC/CR/newline-allow/printable-range) reproduced faithfully; multi-stream wrapper labels out/err.
- Notes: No committed self-test for the helper — appropriate (no contract beyond the nine guards; committing one would over-test a util). Could not run go test/go vet (no shell); judged by reading — scan is byte-equivalent to original.

CODE QUALITY:
- Project conventions: Followed — switch-true, t.Helper() on both, variadic wrapper, fmt.Sprintf context.
- SOLID principles: Good — single responsibility; stream wrapper composes the single-buffer helper.
- Complexity: Low.
- Modern idioms: Yes — range-over-bytes, switch-true, variadic.
- Readability: Good — doc comments explain contract + single-definition intent.
- Issues: None blocking.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES:
- [idea] internal/presenter/bytepurity_test.go:39 — streamLabels caps named labels at out/err with a "stream N" fallback for a third+ buffer, but both sites pass exactly (out, errBuf); the fallback is unused generality. Decide whether to keep the open-ended variadic or tighten to (out, err *bytes.Buffer).
- [do-now] internal/presenter/bytepurity_test.go:21 — failure messages weave context in twice (reword of the original per-arm wording); cosmetic, align if exact-message parity with the pre-refactor strings is desired. Failure-path only.
