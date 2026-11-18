# Analysis Report: Token Aware Diff Sizing (Cycle 4)

## Stats

- Total findings: 2
- Deduplicated findings: 2
- Proposed tasks: 2

## Summary
Two findings cleared the floor and became two proposals; neither reverses a settled direction. The medium one: the notes Assembler runs its diff and Change Map from the process cwd, while the exclusion check matches globs root-relative. From a subdirectory, an Exclude answer therefore removes nothing, which breaks CLAUDE.md seam 1. The low one: the unwind's reset, restore and tag-delete warnings show the runner's exit text, where its unstage and re-commit warnings show git's stderr. Standards found nothing. Two stale comments in internal/notes are collected for direct correction.

## Comment Corrections

- internal/notes/assemble.go:10-11 — the package doc says the size check is "deliberately not here", but the package holds the max_prompt_bytes check (generate.go:227) and the chunk sizing it drives
  OLD: // bookkeeping file; embedded mode keeps the real source). The max_prompt_bytes size
// check is a later layer — deliberately not here.
  NEW: // bookkeeping file; embedded mode keeps the real source).
- internal/notes/generate.go:149 — "in later tasks" is workflow vocabulary, and the downstream write has shipped
  OLD: // (no parsing/splitting) for the downstream provider/changelog write in later tasks.
  NEW: // (no parsing/splitting) for the downstream provider/changelog write.

## Discarded Findings
- Duplication finding's "record twin": `record.CommitStaged` does not wrap its commit failures in `*record.GitError`. This part is split off and discarded; the unwind half is Task 2.
  - The ✗ lines for the bookkeeping, pre_tag artifact and changelog commits have carried no git output since before this feature. The pre-feature `CommitBookkeeping` and `CommitDirtyTree` also discarded the Result.
  - Wrapping alone would surface nothing. `failureOutput` (internal/engine/release.go:1991) reads only the answers-recording and AI-generation carriers, and `resetAndAbort`'s StageFailed carries no Output. So the finding's claim that `gitOutput` would become "the one reader for every commit step" does not match how the tree is wired.
  - The fix is a change to the stage-failure surface, outside this feature's scope. It is low severity and clusters with nothing else.
- The standards agent's summary also notes, without raising it as a finding, that the README pipeline line, the pre_tag rows (README, metadata.go) and the guide's pipeline section put pre_tag after the notes, while the engine runs it before them. The agent reads this as outside this feature's §18 scope, so it is not synthesized here.
