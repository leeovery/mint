TASK: cli-presentation-3-3 — Line-read input model: case-insensitive parse, empty-Enter default, re-prompt loop

ACCEPTANCE CRITERIA:
- Empty Enter returns the gate's declared Default.
- Case-insensitive: N/n -> n, Y/y -> y, E -> e, R -> r (declared keys only).
- Unrecognised input re-prompts; never returns a non-declared choice; never silently accepts.
- Whitespace-only line treated as empty-Enter => default; documented and tested.
- Repeated unrecognised lines keep re-prompting; a subsequent valid line is accepted.
- EOF returns a non-nil error (no infinite loop, no silent default-accept).
- Parse/loop core is mode-agnostic and uses the gate's declared set — no hardcoded y/n/e/r.

STATUS: Complete

SPEC CONTEXT: "Gating & -y Orthogonality" (spec:100-116) — line-read, empty Enter = default = accept, case-insensitive, unrecognised key (incl. a/q) re-prompts, never silently accepts. Source/target prompts (3-7) reuse this model. Dependencies (spec:302-305) — a/q superseded, so unrecognised.

IMPLEMENTATION:
- Status: Implemented
- Location: prompt.go:47 (parseChoice — trim, empty->Default, lowercase+g.Has), :74 (readChoice — render/read/parse loop, errPromptEOF only on EOF with no usable line), :17 (errPromptEOF), :117 (bufferedReader — persistent *bufio.Reader); plain.go:345 (Prompt), pretty.go:745 (Prompt); injectable input via NewPlainPresenterWithInput / pretty WithInput; production wiring threads os.Stdin at NewForStartup.
- Notes: Whitespace-only => default (spec's recommended option), documented prompt.go:36-43. Both presenters share parseChoice/readChoice verbatim — only render closure differs. a/q reconciliation falls out: gates declare only y/n/e/r. Memoised reader avoids dropping buffered tail across re-prompts.

TESTS:
- Status: Adequate
- Coverage (prompt_test.go, per-mode via gateDrivers table): empty-Enter default (:20), case-insensitive (:39), unrecognised re-prompts then accepts (:71), a/q re-prompt (:94), whitespace-only default (:113), repeated unrecognised then valid (:133), EOF returns error (:156), mode-agnostic equivalence (:183), declared-set honoured (:215), hint from declared keys (:237).
- Notes: Re-prompt counts verified via "Continue?" render count (behavioural). Not over-tested — table driver collapses plain/pretty arms.

CODE QUALITY:
- Project conventions: Followed — io.Reader injection + bufio, sentinel, typed Choice.
- SOLID principles: Good — parseChoice/readChoice cleanly separated and shared, rendering injected as closure.
- Complexity: Low.
- Modern idioms: Yes — ReadString EOF contract handled, memoised reader avoids buffered-tail bug.
- Readability: Good.
- Issues: None blocking.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES:
- [quickfix] internal/presenter/prompt_test.go:156 — the documented positive EOF path (a VALID trailing line with no newline at EOF, e.g. "y"+EOF returns the choice with nil error) is only exercised indirectly. Add a per-mode case asserting input "y" (no newline) against NotesReviewGate() returns ChoiceYes, nil err.
- [do-now] internal/presenter/prompt.go (plain.go:345 / pretty.go:745) — cross-reference the whitespace-only=>default rule from each Prompt doc comment (currently only on parseChoice).
