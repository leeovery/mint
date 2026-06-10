TASK: cli-presentation-8-1 (tick-056f50) — Restore the One-Place Mode-Branch/Stream-Split Invariant in Presenter Wiring

ACCEPTANCE CRITERIA:
- The mode-branch + stdout/stderr stream-split decision appears in exactly one place in internal/presenter; the other seam delegates to it (PREFERRED) OR the doc comments are corrected so no comment falsely asserts a single-place invariant the code does not satisfy (FALLBACK).
- New's and NewForStartup's public signatures are unchanged.
- NewForStartup still threads all four signals exactly as before (mode, term-width pretty-only, -y, stdin-interactive); plain still does not probe width.
- No behaviour change: existing presenter tests pass without modification to their assertions.
- No new exported symbols; change confined to internal/presenter; engine/main untouched.

STATUS: Complete

SPEC CONTEXT: Spec (spec:22-24) fixes the stream split as a backbone invariant ("fixed regardless of mode") but mandates no particular code structure. The "one place" requirement is an architecture-internal concern: cycle-3 task 7-2 added NewForStartup which re-implements the mode-branch + split inline, making New's prior "single wiring point" doc comment stale.

IMPLEMENTATION:
- Status: Implemented (via the sanctioned FALLBACK path)
- Location: wiring.go — New doc comment L8-20 + body L21-26; NewForStartup doc comment L28-63 + body L64-75.
- Notes: PREFERRED structural convergence NOT done — the mode-branch + split still physically lives in two places (New L22-25, NewForStartup L66-74). Explicitly permitted: criterion 1 offers the fallback; Do step 5 sanctions it when clean delegation isn't worth the contortion (NewForStartup returns the Presenter interface while branches build different concrete types with mode-specific setter chains). FALLBACK applied correctly: New's comment now "raw, lower-level wiring seam", no "single wiring point" claim, defers to NewForStartup; single-production-site language moved onto NewForStartup ("the ONE production construction site"). Accurate as scoped — New is test-only (grep: only wiring_test.go newSplit). Signatures unchanged. Four-signal threading verified (mode :65, width pretty-only :68, -y :69/:73, stdin-interactive :70/:74; plain doesn't probe width). No new exported symbols; engine/main untouched.

TESTS:
- Status: Adequate (doc-comment correction; regression guard is the existing suite passing without assertion edits)
- Coverage: wiring_test.go newSplit (:34-453) drives New across both modes (split contract); NewForStartup tests (:475,:499,:540) prove the production seam arms all gating axes; width-on-pretty via pretty_width_test.go.
- Notes: Correctly no new test (doc comments aren't behaviour); no assertion modified (proof of no behaviour change).

CODE QUALITY:
- Project conventions: Followed — full-sentence doc comments explain the why.
- SOLID principles: Good — New as narrow test seam, NewForStartup as production composition root.
- Complexity: Low (no control-flow change).
- Modern idioms: Yes — functional-options chaining unchanged.
- Readability: Good — cross-references clarify test-vs-production split.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES:
- [idea] internal/presenter/wiring.go:21,64 — the mode-branch + out/err split is still duplicated literally between New (L22-25) and NewForStartup (L66-74). The doc-only fallback is acceptance-compliant, but a future cleanup could express the "which mode → which constructor + WithErr" decision once (a small unexported helper returning the concrete presenter for NewForStartup to chain on, and the interface for New), genuinely satisfying the original one-place intent. Deferred deliberately; a design choice with a behaviour-equivalence bar, not a mechanical edit.
