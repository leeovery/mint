AGENT: standards
FINDINGS:
- FINDING: CLAUDE.md's unwind invariant still says the answers commit is the only thing an unwind keeps
  SEVERITY: low
  FAILURE: CLAUDE.md is the working contract for changing the code. Its "Surgical unwind, then PONR" invariant says every pre-push mutation is unwound and that "the one exception is the exclusion check's answers commit". Since the 2026-10-21 corrigendum to §6.6 (task 8-9) there is a second deliberate keep: when git ignores an untracked `.mint.toml`, the answers are written but not committed, and an abort or failure after answering leaves that write in place (`a.edit = nil` before the unwind). A maintainer or agent working from the contract sees a pre-push file write that survives an unwind and reads it as a leak. If they add it to the unwind, the dead end the corrigendum closed comes back: a repo that ignores `.mint.toml` is asked the same questions on every re-run after an abort. The `release_answersignored` tests would catch the change, but only after the work is done, and the contract would still be wrong.
  FILES: CLAUDE.md:42, internal/engine/release.go:1105-1113, internal/engine/unwind.go:11-14
  DESCRIPTION: §18.3 says the invariant must state the exclusion check's exception to the unwind, and §18 says CLAUDE.md stays true to what is built, changed in the same commit as the behaviour. Task 8-9 updated the README (line 117) and the unwind.go header comment for the ignored-config case but not CLAUDE.md, so the contract now names one exception where the code has two.
  RECOMMENDATION: Rewrite the exception sentence in CLAUDE.md's "Surgical unwind, then PONR" bullet to cover both outcomes of the answers. For example: "The one exception is the exclusion check's answers: the unwind resets past their commit with the rest, then re-commits them on the starting commit and narrates it as kept; where git ignores `.mint.toml`, the uncommitted write is left in place."
COMMENT_CORRECTIONS:
- internal/config/config.go:56-59 — "abort … fails loud and tags nothing" is false now that an attended release under abort is offered the floor's choice and can still tag; the closed-set sentence repeats onNotesFailureValues' comment
  OLD: // defaultOnNotesFailure is the out-of-the-box notes-failure policy: "abort" — when
// the normal AI path fails, mint fails loud and tags nothing. The opt-in alternative
// is "fallback". These two are the closed valid set: an absent key defaults to abort
// and any other non-empty value is rejected by Load (see validateOnNotesFailure).
  NEW: // defaultOnNotesFailure is the notes-failure policy an absent on_notes_failure key
// resolves to.
- internal/engine/regenerate_batch.go:184-186 — single-version regenerate has no "on_notes_failure=abort default" to override, since regenerate never reads on_notes_failure and an attended single-version run gets the floor's choice
  OLD: // per version). A per-version FAILURE is CAUGHT, RECORDED with a human reason, and the
// loop CONTINUES — consciously OVERRIDING the single-version on_notes_failure=abort
// default (one version whose notes fail must not kill the others).
  NEW: // per version). A per-version FAILURE is CAUGHT, RECORDED with a human reason, and the
// loop CONTINUES, so one version whose notes fail does not stop the others.
- internal/engine/regenerate_batch.go:268-270 — the same false claim that single-version regenerate has an on_notes_failure=abort default
  OLD: // Per-version FAILURES are caught here and returned as a *skippedVersion rather than
// surfaced as a terminal abort: this is the skip-and-continue contract that overrides
// the single-version on_notes_failure=abort default. The covered failures are:
  NEW: // Per-version FAILURES are caught here and returned as a *skippedVersion rather than
// surfaced as a terminal abort: skip-and-continue is the batch's floor. The covered failures are:
SUMMARY: The implementation matches the specification and its corrigenda on every decision checked: the lean command, quote-aware parsing, the cap, chunking, classification, halving, the floor, the exclusion check and its answers, --notes-file, narration, commit, and the docs and messages the spec fixes. What drifted is documentation only: CLAUDE.md's unwind invariant misses the ignored-.mint.toml keep added on 2026-10-21, and three comments still describe on_notes_failure behaviour this feature replaced.
