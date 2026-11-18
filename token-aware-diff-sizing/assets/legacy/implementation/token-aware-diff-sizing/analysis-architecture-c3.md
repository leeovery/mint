AGENT: architecture
FINDINGS:
- FINDING: A single-version regenerate carries -y in two fields, and the engine reads only one of them
  SEVERITY: low
  FAILURE: `RegenerateRunRequest.Yes` is set by cmd/mint (`regenerateRunFlags`) and by the engine tests' `runReq` helper, but `RegenerateRun` never reads it. Whether a single-version regenerate is attended now changes what it does when its notes fail, and it learns that only from `ReleaseDeps.Yes` and `StdinInteractive`, through `deps.attended()`. Take a caller that marks the run -y on the request alone, at a terminal. It gets an attended run. When the notes generation fails, the floor offers its `c` / `e` / `a` choice instead of stopping with the next moves §12.3 requires. Under a -y presenter that choice auto-answers its default, Abort. The operator sees an abort with no next moves and no `--notes-file` pointer, and nothing catches the slip at compile time. The suite already shows the trap:
    - `regenChunkNotes.regenerate` passes `yes=true` to `runReq`. Its run is unattended only because its deps leave `StdinInteractive` false.
    - `regenerateUnattended` copies `run.deps.Yes` into the request, as though the engine read both fields.
  FILES: internal/engine/regenerate_interactive.go:100-103, internal/engine/regenerate_interactive.go:139, internal/engine/regenerate_interactive.go:241, internal/engine/notesfloor.go:18-20, cmd/mint/regenerate_run.go:36, internal/engine/regenerate_batch.go:464
  DESCRIPTION: The field was unread before this feature too, but then -y had no engine-side effect on a single-version run: the presenter answered every gate itself. This feature made attended versus unattended an engine decision (CLAUDE.md seam 4) and sourced it from `ReleaseDeps`. It also deleted the field's doc comment and left a bare `Yes bool` beside the new `Config` field. Its sibling `BatchRegenerateRequest.Yes` is read, since `gatePerVersion` skips the per-version gates on it, so the same-named field on the single-version request looks live. The struct's doc still lists "the -y flag" among its inputs. One fact now has two homes, one of them a decoy.
  RECOMMENDATION: Delete `RegenerateRunRequest.Yes`, its assignment in `regenerateRunFlags`, and "the -y flag" from the struct's doc comment. `ReleaseDeps.Yes` then becomes the only place a single-version regenerate learns -y. Drop the `yes` parameter from the `runReq` test helper, which the deleted field forces. The tests that need an unattended run already set `run.deps.Yes` or leave `StdinInteractive` false, so their assertions stand.
COMMENT_CORRECTIONS:
- internal/notes/assemble.go:113-115 — "mint does NO Go-side glob matching" is false now that `pathspecMatches` (internal/notes/pathspec.go) matches `diff_keep` globs and the AI's suggested globs in Go, including the Exclude answers that join these pathspecs through `Assembler.Exclude`
  OLD: // diff and Change Map git calls — the UNION of the exclusion tiers. git
// interprets each glob as a pathspec pattern — mint does NO Go-side glob matching — so a
// glob matching nothing is harmless and a force-added tracked file matching a glob is
  NEW: // diff and Change Map git calls — the UNION of the exclusion tiers. git
// interprets each glob as a pathspec pattern, so a glob matching nothing is harmless
// and a force-added tracked file matching a glob is
SUMMARY: Since cycle 2 the changes are the cmdline.Quote fix in resetTo's restore step and the README, and neither adds a seam. The pieces compose soundly:
- every AI call site checks the cap on its own composed prompt;
- the wave's locking holds;
- the shared Assembler carries Exclude answers into `r`;
- the keeper binds merge inputs to a failed `r`'s save, so a chunked `r` never double-saves;
- the answers commit's fold into MadeState covers every exit.
One low finding remains: a dead `Yes` field on the single-version regenerate request sits beside the live `ReleaseDeps.Yes` that now decides the notes floor. One comment still says mint does no Go-side glob matching, which this feature made false.
