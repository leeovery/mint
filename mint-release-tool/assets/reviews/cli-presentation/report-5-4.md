TASK: cli-presentation-5-4 — State the ASCII/case-fold precondition on SourceGate/TargetGate

ACCEPTANCE CRITERIA:
- SourceGate and TargetGate each carry a doc comment stating the "ASCII enumerated values only" precondition and the reason (case-folding parse + byte-pure AcceptEcho).
- No functional/code change beyond comments; rendered behaviour and the seam unchanged.
- `go vet ./...` and `go test ./internal/presenter/...` pass.

STATUS: Complete

SPEC CONTEXT: spec:116 establishes source/target prompts use the same case-insensitive line-read model and -y auto-accept echo in key:value vocabulary, relying on ASCII enumerations but not stating that dependency. This task converts the implicit reliance into a stated precondition. Documentation-only by design.

IMPLEMENTATION:
- Status: Implemented
- Location: gate.go:185-192 (SourceGate), :210-218 (TargetGate).
- Notes: Both constructors carry a "Precondition:" paragraph: option keys and def must be ASCII, with two reasons — (1) parseChoice case-folds via strings.ToLower before matching, (2) AcceptEcho = string(def) must stay byte-pure ASCII for the plain echo. Notes the contract is NOT enforced and a future non-ASCII broadening must revisit. Claims verified: parseChoice (prompt.go:52) does Choice(strings.ToLower(trimmed)) before g.Has; byte-purity guard (bytepurity_test.go:26) flags bytes <0x20||>0x7e; function bodies (gate.go:193-201,219-227) unchanged.

TESTS:
- Status: Adequate (documentation-only change, no new test required)
- Coverage: documented mechanisms already exercised by gate_sourcetarget_test.go — case-fold by TestSourceGateCaseInsensitiveAndEmptyEnterDefault (:244, "GITHUB"→"github"), echo byte-purity by TestYesSourceEchoIsBytePureASCII (:183). Both pass unchanged.
- Notes: Correctly no behavioural test added (a doc-comment-text test would over-test).

CODE QUALITY:
- Project conventions: Followed — comment style matches gate.go voice.
- SOLID principles: N/A (no logic change).
- Complexity: Low (unchanged).
- Modern idioms: Yes (unchanged).
- Readability: Good — precondition states exact mechanisms and the unenforced contract.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
