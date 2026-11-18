AGENT: duplication
FINDINGS: none
COMMENT_CORRECTIONS:
- internal/engine/unwind.go:315-317 — the clause added in task 12-2 cites warnPublishFailed as a "captured-output warn" the unwind mirrors. warnPublishFailed passes `cause.Error()`, the runner's exit text, which is the very form 12-2 moved the unwind away from. A reader who follows the pointer gets the wrong model of what goes beneath the warning.
  OLD: // action names the exact manual step; the failing step's own output rides the Warn's
// Output verbatim, mirroring the engine's other captured-output warns (warnPublishFailed
// et al).
  NEW: // action names the exact manual step; the failing step's own output rides the Warn's
// Output verbatim.
SUMMARY: Since cycle 4 the code changes are task 12-1 (the notes Assembler's diff and Change Map reads now run at the repo root) and task 12-2 (`failedGitOutput` now carries git's stderr under every unwind warning). Neither adds cross-site duplication. 12-2 left one copy of the stderr-or-error-text rule: `gitOutput`, `unstageEdit`, `deleteTagIfMade` and all three `resetTo` steps go through it. 12-1's three reads are the Assembler's only pathspec-bearing git calls, and both production constructors pass the `--show-toplevel` root. A re-survey of the plan's production code found nothing that clears the floor. Exclude composition (one site per verb), cause classification, the `r` rule, the cap checks, the paste-ready quoting, the answers-subject builder and matcher, and the cache key each still have a single source. Pretty and plain renderers share their halving, saved-notes and answers text helpers. One comment that 12-2 edited cites the wrong function as its model.
