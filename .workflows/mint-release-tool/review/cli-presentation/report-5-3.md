TASK: cli-presentation-5-3 — Collapse the four pretty constructors into one with functional options (tick-c8a3d9)

ACCEPTANCE CRITERIA:
- PrettyPresenter has a single exported constructor NewPrettyPresenter(out io.Writer, opts ...Option); the three NewPrettyPresenterWith... variants and the WithInput setter are gone.
- The "force colour AND capture err AND script input" combination is reachable in one constructor call with three options, no setter chaining.
- Production wiring and all tests compile and pass against the new API; rendered output and the Presenter seam byte-for-byte unchanged.
- `go build ./...`, `go vet ./...`, `go test ./internal/presenter/...` pass.

STATUS: Complete

SPEC CONTEXT: Pure test-ergonomics refactor of the pretty presenter's construction surface. Orthogonal axes: colour profile, err writer (stream-split), gate input reader. Rendered behaviour and the Presenter seam unaffected.

IMPLEMENTATION:
- Status: Implemented
- Location: pretty.go:185 (Option type), :194 (WithProfile via renderer.SetColorProfile), :205 (WithErr), :214 (WithInput), :228 (NewPrettyPresenter build-then-apply), :241 (newPrettyPresenter core defaults); wiring.go:23,67 (call sites → NewPrettyPresenter(out, WithErr(...))).
- Notes: Three NewPrettyPresenterWith... constructors and WithInput method removed — grep confirms zero production references. Builder setters that aren't gap-patches (WithYes, WithInteractiveStdin, WithSpinnerFactory, WithTermWidth) retained. Defaults match prior behaviour exactly. WithProfile acts on already-built renderer so option order irrelevant. No drift.

TESTS:
- Status: Adequate
- Coverage: combined all-three-options (pretty_test.go:47 — force-colour+capture-err+script-input in ONE call, three assertions); WithProfile colour-on/off (:113,:142 via drivePretty); WithErr+WithProfile stream-split (gate_forbidden_test.go:135); WithInput scripted gate (gate_test.go:167, golden_transcript_test.go:167, pretty_test.go:571/593). "Behaves identically" anchored by unchanged golden transcript + bytepurity tests.
- Notes: Each axis individually + combined. Not over-tested; behaviour-focused.

CODE QUALITY:
- Project conventions: Followed — functional-options idiom, option doc comments, thin production entry.
- SOLID principles: Good — open/closed improved (future axis = new Option); single defaults core.
- Complexity: Low.
- Modern idioms: Yes — variadic functional options.
- Readability: Good.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
