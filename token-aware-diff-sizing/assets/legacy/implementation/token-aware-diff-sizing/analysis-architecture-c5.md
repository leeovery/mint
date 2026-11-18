AGENT: architecture
FINDINGS: none
COMMENT_CORRECTIONS:
- internal/notes/assemble.go:149-151 — "mint does no Go-side ... glob matching" is false: `pathspecMatches` (internal/notes/pathspec.go) matches these same globs in Go for the exclusion check's suggestions, pause counts and `diff_keep` coverage. Cycle 3 removed the identical claim from `excludePathspecs` in this file, and phase 12's rewrite of this paragraph kept it.
  OLD: // :(exclude) glob root-relative. GIT performs every exclusion via the :(exclude)
// pathspecs; mint does no Go-side hunk
// stripping or glob matching. The returned text is git's stdout verbatim — whatever
  NEW: // :(exclude) glob root-relative. GIT performs every exclusion via the :(exclude)
// pathspecs. The returned text is git's stdout verbatim — whatever
SUMMARY: Phase 12's two changes compose cleanly, and neither adds a seam:
- The notes Assembler now takes the `--show-toplevel` root. Its diff and both Change Map reads run through `RunInDir`, and both production constructors pass the root they already hold. Those three reads are the only pathspec-bearing reads in internal/notes, so `pathspecMatches`' root-relative matching and git's matching now agree from any directory.
- `failedGitOutput` is the one copy of the stderr-or-error-text rule, and every git step in the unwind and in regenerate's reset goes through it.
I re-checked the rest of the feature's git steps from a subdirectory against real git 2.56: the clean-tree probe, `--autostash`'s stash push and `resetTo`'s soft-reset-then-restore. Each covers the whole tree with its exclude-only `:(top,literal,exclude)` pathspec. The other mutations run with `-C root` or take no pathspec. The deadline-proof spy's no-op `RunInDir` blanks no assertion, because every test that uses it calls the transport directly. No other candidate clears the floor. The candidates I ruled out:
- the answers commit's whole-index `CommitFile`: it never runs beside a notes file, since `--notes-file` skips the check;
- the attended-predicate twins in engine and commit, and the three copies of the root passed to Assembler, Generator and Selector: each copy is set from one value at every production site;
- the run's state that the cycle 1–3 reports already ruled on.
One stale comment in internal/notes/assemble.go is collected for direct correction.
