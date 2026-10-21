# Consolidation Findings: token-aware-diff-sizing (Phase 12)

## Findings

None. The two tasks touch separate surfaces (the notes Assembler's working directory; the unwind's warning Output). Assembled side by side, they leave no duplicated rule, no drift and no dead code. Task 12-2's `failedGitOutput` (internal/engine/release.go:1175) is now the one copy of the stderr-or-error-text rule. `gitOutput`, `unstageEdit`, `deleteTagIfMade` and all three `resetTo` steps go through it. Task 12-1's three Assembler reads (internal/notes/assemble.go:190, internal/notes/changemap.go:51, :57) are the only pathspec-bearing reads in internal/notes. Both production constructors (internal/engine/release.go:1016, internal/engine/regenerate_fresh.go:121) pass the `--show-toplevel` root.

## Comment Corrections

- internal/notes/assemble.go:87-91 — the second sentence lists what the Assembler holds as the runner and the ExcludeConfig. It leaves out the root the phase added, which is where every read now runs. It also repeats NewAssembler's doc, which already gives the injection reason and documents both root and exclude.
  OLD: // Assembler builds the release diff context through the CommandRunner seam. It
// holds the runner (so production wiring passes the os/exec-backed runner while
// tests pass a FakeRunner — no real git, fully scriptable) and the consolidated
// ExcludeConfig (diff_exclude globs + the strategy-aware version_file decision)
// layered on top of the built-in always-excludes.
  NEW: // Assembler builds the release diff context through the CommandRunner seam.
