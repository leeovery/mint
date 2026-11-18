TASK: mint-release-tool-2-14 — `r` regenerate-with-context (loop) & no-AI gate variant

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented, matches ACs. engine/release.go reviewGate loop (ChoiceRegen case), regenerateBody (AskLine→Regenerate→re-show), gateForKind selects y/n/e/r vs y/n/e by notes.KindNormalAI, perRunRegenerator binds per-run AI closure, Regenerator seam. presenter/gate.go NotesReviewGate (y/n/e/r); engine.go FirstReleaseReviewGate (y/n/e). AskLine seam; ErrInputClosed. One-time line flows only into regen.Regenerate(ctx, line), never config — guaranteed by construction.

TESTS:
- Status: Adequate. release_test.go: r-then-y reaches sinks w/ regenerated body + AskLine once, empty-context legal, ErrInputClosed fail-loud + no mutation + Regenerator never consulted, multiple r loops final body to sinks, Regenerator error aborts, nil-Regenerator aborts cleanly, no-AI paths omit r (first-release/degenerate/--no-ai), normal-AI offers y/n/e/r. "Not persisted" covered at notes layer (config struct unchanged) + empty-context byte-identity + append tests.

CODE QUALITY:
- Followed conventions (accept-interfaces seams, single runner/presenter seams, errors wrapped w/ sentinels for errors.Is, table tests). SOLID good — reviewGate semantics only, regenerateBody isolates regen step. Low complexity, strong readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/engine/release_test.go (near TestRelease_GateMultipleRegen_FinalBodyReachesSinks, :1897) — add one engine-level assertion that the on-disk .mint.toml (or its absence) is byte-unchanged after an r regenerate run, so the "one-time context never persisted to [release].context" criterion is proven end-to-end through Release, not only at the notes-package layer.
- [do-now] internal/engine/release.go:1386-1387 — the comment says ErrNotInteractive "should be unreachable here — it is defended against anyway," but regenerateBody only special-cases the cause generically via abort(err); reword to state plainly that any AskLine error (including a defensively-handled ErrNotInteractive) is wrapped by abort, to avoid implying a dedicated branch that does not exist.
