## Attempt 1

ISSUES:
- internal/notes/changemap_test.go:480-506 (`TestNotesSource_DescribesChangeMapAsCoverageNotRanking`): the novelty half of the guard only runs when the file is named `changemap.go` (`if name == "changemap.go" && novelty.MatchString(line)`). If changemap.go is renamed or split, the "weighted above|headline" check silently scans nothing and the test stays green. Nothing asserts that the scan found any non-test source at all either. The project's existing source-scan guard defends against exactly this: internal/presenter/prompt_render_only_test.go:46-48 fails with "no non-test sources found to scan — the guard would be a false positive". Impact is low, but it is a guard that can pass while checking nothing.
  FIX: In the loop, set a `scannedChangeMap` bool when `name == "changemap.go"`. After the loop, `t.Fatal` if it is false ("changemap.go not found — the novelty guard would check nothing"). Also `t.Fatal` when no non-test source was scanned, mirroring prompt_render_only_test.go:46-48.
  ALTERNATIVE: Read the novelty half's file directly with `os.ReadFile("changemap.go")`, which fails loudly on a rename. It is simpler, but it duplicates the line-scan loop. I recommend the bool.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/notes/prompt.go:242-245 — the reflow stopped partway, leaving line 243 at 133 chars while the rest of the doc wraps near 85.
  OLD: // ResolveInstructions). changeMap is the coverage preamble from BuildChangeMap —
// where the release's changes landed, never a measure of importance. diff is the post-exclusion diff from AssembleDiff. The function
// is PURE: it only orders and joins, performing no IO and no transport — it
// produces the AI INPUT, never parses the AI output.
  NEW: // ResolveInstructions). changeMap is the coverage preamble from BuildChangeMap —
// where the release's changes landed, never a measure of importance. diff is the
// post-exclusion diff from AssembleDiff. The function is PURE: it only orders and
// joins, performing no IO and no transport — it produces the AI INPUT, never
// parses the AI output.
- /Users/leeovery/Code/mint/internal/notes/changemap.go:271-274 — line 273 was not reflowed (142 chars). "SECONDARY" is left over from the PRIMARY/SECONDARY tiering the diff removed from noveltyLines (now "first section").
  OLD: // renderMagnitude is the SECONDARY section: per-file churn rolled up to area
// (leading-directory) granularity and ranked by total churn descending — the
// readable form, since a flat per-file list is itself mush on big releases. Top-level files roll up under their own filename as their "area".
// Ties break alphabetically for deterministic output.
  NEW: // renderMagnitude is the second section: per-file churn rolled up to area
// (leading-directory) granularity and ranked by total churn descending — the
// readable form, since a flat per-file list is itself mush on big releases.
// Top-level files roll up under their own filename as their "area". Ties break
// alphabetically for deterministic output.
- /Users/leeovery/Code/mint/internal/notes/changemap_test.go:414-416 — the comment names `mixedReleaseGit`, an identifier that doesn't exist. The constants are `mixedReleaseNameStatus` and `mixedReleaseNumstat`.
  OLD: // mixedReleaseGit is a release touching every part of the map: a new package
// nested under an existing internal/ (internal/foo/ wholly added beside a modified
// internal/bar/), a new top-level package, a rename, a removal and a new root file.
  NEW: // mixedReleaseNameStatus and mixedReleaseNumstat are one release touching every
// part of the map: a new package nested under an existing internal/ (internal/foo/
// wholly added beside a modified internal/bar/), a new top-level package, a rename,
// a removal and a new root file.
- /Users/leeovery/Code/mint/internal/notes/changemap_test.go:425-426 — history narration ("as before the prompt stopped ranking by it"). The exact `want` and the argv assertion below already say it, so delete it. The OLD lines are tab-indented inside the function body.
  OLD: 	// The map is coverage metadata, built exactly as before the prompt stopped
	// ranking by it: same two git calls, same sections, same lines, same bytes.
  NEW:

NOTES:
- I re-ran every gate and all pass: go build, gofmt -l (empty), go vet, go test -race ./..., golangci-lint (0 issues).
- The executor says the byte-for-byte expectation was captured from the code before this change. That checks out: changemap.go's non-comment lines are identical to HEAD.
- prompt.go:22, inside the DefaultPrompt string, is 89 chars after the partial reflow. new-v3 wraps it as "…described below. Release\nnotes are…". This is cosmetic: whitespace in the prompt is immaterial and the tests normalise it.
- The source-scan guard bans the word "headline" anywhere in changemap.go. A future comment there that correctly uses this feature's own vocabulary (e.g. "the map does not pick the headline") will fail it. It would fail loudly, so it is not a finding.
- A bug in the existing code, outside this feature (§1.2 keeps rendering as is): without `-z`, git's `--numstat` prints a rename as `arch/{i386 => x86}/Makefile` (git-diff(1), "numstat" section; git 2.56.0 here), never as `old\tnew`. So `numstatPath`'s two-field branch never fires on real output, and a rename's churn rolls up under a `{a => b}` or `old => new` path. That puts it in the wrong area, e.g. `{legacy => core}/` or `legacy/`. The new fixture models the tab form, like the existing fixtures. It needs triage on its own.
- The dispatch carried nothing beyond the enumerated inputs.
