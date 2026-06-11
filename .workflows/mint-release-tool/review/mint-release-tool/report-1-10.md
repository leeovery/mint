TASK: mint-release-tool-1-10 — Annotated tag & atomic push

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/release/release.go Releaser.TagAndPush: annotated tag via `git tag -a {tag} -F -` with message piped on stdin, composed as `subject\n\n{body}` reusing record.BookkeepingSubject. Atomic push exactly `git push --atomic origin HEAD {tag}`. Success returns Outcome{PointOfNoReturnCrossed:true}; rejected push wraps ErrPushRejected leaving ErrCommandNotFound unwrapped. All git via git.Mutator over runner seam. Orchestrator wiring consumes contract: nil err = PONR crossed; errors.Is(ErrPushRejected) drives made.TagCreated so rejected push deletes tag while tag-creation failure doesn't.

TESTS:
- Status: Adequate. Covers annotated-tag argv + exact piped message (`🌿 Release v0.0.1\n\nInitial release.`), exact atomic push argv, PONR signal on success, rejected push surfaces + ErrPushRejected + no PONR, tag-creation failure distinguishable + push never attempted, git-missing distinct at both steps, contended-lock-recovers via Mutator. Each test pins one behaviour, exact argv + stdin.

CODE QUALITY:
- Followed conventions (runner seam, focused tests, sentinels via errors.Is). SOLID good — single responsibility, DI via Mutator. Low complexity, %w double-wrap, strings.NewReader stdin, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
