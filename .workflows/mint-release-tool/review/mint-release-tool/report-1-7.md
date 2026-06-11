TASK: mint-release-tool-1-7 — Adopt the as-built Presenter seam & recording fake

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (adopt-and-verify, no drift). engine.go has cross-spec boundary doc comment, FirstReleaseReviewGate (hand-built y/n/e Gate literal with Subject="notes"/AcceptEcho="accepted"/Default=ChoiceYes), ReviewDecision, AbortError. Event emission in release.go: RunStarted, ShowPlan, ShowNotes, StageFailed, structured Warn (separate Label/Message/Output) incl. post-PONR warnPublishFailed. Engine depends only on presenter.Presenter — no parallel interface or fake.

TESTS:
- Status: Adequate. engine_test.go pins gate shape (y/n/e only, no r), drives ReviewDecision through RecordingPresenter (NextChoices, single KindPrompt, At(0)), proves ErrNotInteractive and ErrInputClosed map to *AbortError with non-zero ExitCode preserving sentinel. Structured Warn asserted across multiple tests. Compile-time presenter.Presenter conformance assertion.

CODE QUALITY:
- Followed conventions (errors.Is/As, typed Choice, DI seam, table-driven parallel tests). SOLID good — engine is pure consumer of inverted seam. Low complexity, good readability with precise cross-spec doc comments.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
