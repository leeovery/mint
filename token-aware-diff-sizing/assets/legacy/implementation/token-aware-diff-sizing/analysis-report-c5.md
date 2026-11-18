# Analysis Report: Token Aware Diff Sizing (Cycle 5)

## Stats

- Total findings: 0
- Deduplicated findings: 0
- Proposed tasks: 0

## Summary
All three agents returned no findings. Phase 12's two changes sit cleanly in the tree. The Assembler's diff and Change Map reads now run at the `--show-toplevel` root through `RunInDir`. `failedGitOutput` is now the one copy of the stderr-or-error-text rule for every unwind and regenerate-reset git step. The re-surveys of duplication, the spec and its corrigenda, and the seams turned up nothing that clears the floor. The agents collected two stale comments for direct correction, and no staging file is written. Both corrections were checked against the tree: each OLD text matches its file verbatim at the cited lines.

## Comment Corrections

- internal/engine/unwind.go:315-317 — the comment cites warnPublishFailed as a "captured-output warn" the unwind mirrors, but warnPublishFailed (internal/engine/release.go:1941) passes `cause.Error()`, the runner's exit text, which is the form task 12-2 moved the unwind away from
  OLD: // action names the exact manual step; the failing step's own output rides the Warn's
// Output verbatim, mirroring the engine's other captured-output warns (warnPublishFailed
// et al).
  NEW: // action names the exact manual step; the failing step's own output rides the Warn's
// Output verbatim.
- internal/notes/assemble.go:149-151 — "mint does no Go-side ... glob matching" is false: `pathspecMatches` (internal/notes/pathspec.go:15) matches these same globs in Go for the exclusion check's suggestions, pause counts and `diff_keep` coverage. Cycle 3 removed the same claim from `excludePathspecs`, and phase 12's rewrite of this paragraph kept it
  OLD: // :(exclude) glob root-relative. GIT performs every exclusion via the :(exclude)
// pathspecs; mint does no Go-side hunk
// stripping or glob matching. The returned text is git's stdout verbatim — whatever
  NEW: // :(exclude) glob root-relative. GIT performs every exclusion via the :(exclude)
// pathspecs. The returned text is git's stdout verbatim — whatever

## Discarded Findings
- The architecture agent ruled out four candidates without raising any of them as a finding:
  - the answers commit's whole-index `CommitFile`, which never runs beside a notes file because `--notes-file` skips the check;
  - the attended-predicate twins in engine and commit;
  - the three copies of the root passed to Assembler, Generator and Selector, each set from one value at every production site;
  - the run state the cycle 1–3 reports already ruled on.
  None is synthesized here.
- The standards agent's summary again notes the pre_tag ordering, without raising it as a finding. The README pipeline line, the pre_tag rows and the guide put pre_tag after the notes, while the engine runs it before them. The mismatch predates this feature, and §18.1 limits the guide to edits this feature makes untrue. As in cycle 4, it is not synthesized here.
