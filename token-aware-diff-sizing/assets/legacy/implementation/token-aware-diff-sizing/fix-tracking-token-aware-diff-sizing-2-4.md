## Attempt 1

ISSUES:
- /Users/leeovery/Code/mint/internal/notes/chunk.go:41-55 (`splitFileDiffs` sort, key at `headerLine` :57-60). The sort key is the raw header line. git quotes a path that has non-ASCII bytes or special characters: `diff --git "a/docs/caf\303\251.md" "b/docs/caf\303\251.md"`. Because `"` (0x22) sorts before `a` (0x61), every quoted file moves ahead of all unquoted ones.
  - What I checked: a real `git diff` in the scratchpad emits `alpha/a.go, docs/a.md, "docs/café.md", zeta/z.go`, which is correct path order. A replica of this sort turns it into `"docs/café.md", alpha/a.go, docs/a.md, zeta/z.go`.
  - Effect: the sort exists to guarantee path order, but here it breaks git's default path order. The file gets packed into the first chunk, away from its folder's siblings, which defeats §8.3's "keeps each area's files together".
  - The comment's claim "Sorting on the header line orders by path because the header leads with it" is false for these files.
  FIX: key the sort on the unquoted path, not the raw header.
  - Strip `diff --git `. If the rest starts with `"`, take `strconv.QuotedPrefix` and `strconv.Unquote` it; git's C-style quoting (`\ooo` octal, `\t`, `\n`, `\"`, `\\`) is valid Go string-literal syntax. Otherwise use the rest as is. Then drop the leading `a/`.
  - Rewrite the splitFileDiffs comment to state what holds: git emits path order, and the sort restores it when diff.orderFile has reordered the output.
  - Add a test in /Users/leeovery/Code/mint/internal/notes/chunk_test.go. Use files in git's real order with a quoted `docs/caf\303\251.md` header between `docs/a.md` and `zeta/z.go`, and a cap that fits one file per chunk. Assert the quoted file's part sits between those two.
  ALTERNATIVE: drop the sort and pack in git's emitted order, since git's default diff order is already path order. That is simpler and exact by default, but a user's diff.orderFile then sets the packing order. The fully correct form of this option adds `-O/dev/null` to AssembleDiff/AssembleRange, but that changes argv pinned by many tests and the single-pass diff order too. I recommend the path-keyed sort: a smaller blast radius, and it keeps the orderFile guard.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/engine/regenerate_batch.go:335-336 — the task named this file. The comment says a diff over max_diff_lines causes this reason, but release notes no longer run a line guard, so that is false. What is left only restates classifyNotesFailure, so delete it.
  OLD: 	// reasonDiffTooLarge is the skip reason for a notes failure caused by a diff over
	// max_diff_lines (the distinguishable ErrDiffTooLarge case).
  NEW: 
- /Users/leeovery/Code/mint/internal/notes/chunk.go:86-87 — restates the one-line strings.Join below it.
  OLD: // composeChunkPrompt assembles one chunk call's prompt: instructions, Change Map,
// part framing, the chunk's slice of the diff, then OutputReminder.
  NEW: 
- /Users/leeovery/Code/mint/internal/notes/chunk.go:98-100 — restates the assembly below it, "carries no diff" included.
  OLD: // composeMergePrompt assembles the merge call's prompt: instructions, Change Map,
// merge framing, every chunk's notes in chunk order, then OutputReminder. It carries
// no diff.
  NEW: 

NOTES:
- The README release paragraph (/Users/leeovery/Code/mint/README.md:108) and the setup guide (/Users/leeovery/Code/mint/internal/setupguide/setupguide.go:162-169) say chunk calls "each fit the cap" and "Every call mint makes is capped". Today that is not true for a single file bigger than a chunk's room, an over-cap merge, or room ≤ 0. Tasks 2-7 and 2-9 make it true, so the docs describe the end state.
- `chunkRoom` measures the framing at `len(files)` parts. When the real part count has fewer digits, packing can stop a few bytes short of a file that would just fit. It is a deliberate, documented trade, and negligible.
- `TestGenerator_PromptExactlyAtCap_MakesOneSinglePassCall` never asserts `len(want) == s.cfg.MaxPromptBytes`. It holds now because three modified files never condense the map, but nothing pins that the fixture sits exactly at the boundary.
- No test covers the `r` nudge reaching every chunk call and the merge on an over-cap `GenerateWithContext`. It is not one of this task's ACs, but the executor claims it and the code does it.
- The executor's off-script rewording of the `max_diff_lines` README row, metadata description and config.go comments ("not read: …") is accurate: no production code reads MaxDiffLines any more. Task 2-5 removes the key.
- `regenerate_batch.go:228` is untouched by the diff and still gives `"diff too large"` as its example skip reason. Release notes can no longer produce that reason, and 2-5 removes it.
- Nothing rode the dispatch beyond the enumerated inputs.

## Attempt 2

ISSUES:
- /Users/leeovery/Code/mint/internal/notes/chunk.go:70: `headerPath` panics on a valid git header. The `else if` slices `paths[2:half]` without checking `half >= 2`. A remainder of 3 bytes with a space in the middle gives `half = 1`, so `paths[2:1]` panics with `slice bounds out of range [2:1]`.
  - Real git emits such a header. With `diff.noprefix=true`, a user gitconfig setting that mint does not override (`AssembleDiff`/`AssembleRange` run plain `git diff {range} -- .`, assemble.go:178), I reproduced `diff --git x x` for a top-level one-character file in a scratch repo. A byte-identical copy of `headerPath` panics on it.
  - Any over-cap release with two or more files, one of them a one-character top-level path, crashes in `sort.SliceStable`. On a forward release that happens after the pre-tag artifact commit, so the commit is never unwound.
  - The same noprefix form also mis-keys a real top-level `a/` directory: `diff --git a/b.go a/b.go` sorts as `b.go`, because the trailing `TrimPrefix(path, "a/")` strips a real directory name.
  FIX: limit the equal-sides branch to git's prefixed form and check bounds before slicing. Condition: `len(paths)%2 == 1 && half >= 2 && strings.HasPrefix(paths, "a/") && paths[half:half+3] == " b/" && paths[2:half] == paths[half+3:]`, then take `paths[2:half]`. Keep the `a/` trim only on the quoted branch, where the unquoted value carries it. Any other remainder, noprefix included, keys on itself whole and cannot panic. Add a test in /Users/leeovery/Code/mint/internal/notes/chunk_test.go: an over-cap `Generate` whose diff carries noprefix-form sections (`diff --git x x`, `diff --git a/b.go a/b.go`) plus one ordinary file. Assert that `Generate` returns without error and that every section's diff appears exactly once across the chunk slices. It panics today.
  CONFIDENCE: high
- /Users/leeovery/Code/mint/internal/notes/chunk.go:52-54 and :58-75: renames are keyed on the old path, so the sort reorders git's default output. That contradicts the comment at :40-41 ("leaves it untouched otherwise") and §8.3's area rationale.
  - What I checked: in a scratch repo, `git diff` emits a rename in new-path order (`diff --git a/zz.go b/aa.go` before `diff --git a/mm.go b/mm.go`). Running that output through a copy of `splitFileDiffs` swaps them, because `headerPath` returns `zz.go b/aa.go`.
  - Effect: a file renamed and edited across directories (`legacy/auth.go -> auth/auth.go`) is packed among its old directory's files. The Change Map files it under its new area, and so does git's order. The code breaks the invariant its own comment states, the same class of defect round 1 raised for quoted paths.
  FIX: key a renamed or copied section on its destination path. In `headerPath`, before parsing the header, scan the section's lines for one starting with `rename to ` or `copy to `. If found, return the rest of that line, unquoted with `strconv.Unquote` when it starts with `"`. These lines carry bare paths with no `a/`/`b/` prefix; hunk lines always start with `+`, `-`, ` ` or `\`, so no content line can match. Add a test that feeds git's real order (a rename `a/zz.go b/aa.go` with `rename from`/`rename to` lines, then `mm.go`) with a cap that fits one file per chunk, and asserts the rename's part comes first. Combine this with the previous fix as one rewrite of `headerPath`.
  ALTERNATIVE: drop the sort and `headerPath` entirely and pack in git's emitted order. That order is path order by default, renames by new path, which removes both defects and all header parsing. The cost: a user's `diff.orderFile` then sets the packing order, which is no longer "path order" as §8.3 says, and `TestGenerator_OverCap_PacksFilesInPathOrderFillingEachChunk` would have to change. Pinning git's output with `-O/dev/null --src-prefix=a/ --dst-prefix=b/` would make that exact, but it changes argv pinned by many tests and the single-pass diff too. I recommend the targeted fix.
  CONFIDENCE: medium

NOTES:
- The README release paragraph (/Users/leeovery/Code/mint/README.md:108, "chunk calls that each fit the cap") and the setup guide (/Users/leeovery/Code/mint/internal/setupguide/setupguide.go:162-167, "Every call mint makes is capped") describe the end state. Today a single file larger than a chunk's room, room ≤ 0, or a merge over the cap still produces an over-cap call. Tasks 2-7 and 2-9 deliver that.
- `TestGenerator_PromptExactlyAtCap_MakesOneSinglePassCall` (chunk_test.go:242-255) never asserts `len(want) == s.cfg.MaxPromptBytes`. It holds today because the map does not condense at that cap, but nothing checks that the fixture sits exactly on the boundary.
- `generateChunked(ctx, instructions, changeMap, diff, maxPromptBytes)` and `composeChunkPrompt` take 5 parameters each. Tasks 2-6 to 2-10 add concurrency, pieces, narration and failure handling to this path, so a small struct for the chunked run's fixed inputs may compose better then. Nothing is wrong today.
- `CheckDiffSize`/`countDiffLines` (/Users/leeovery/Code/mint/internal/notes/size.go) are now unused in production, and `reasonDiffTooLarge`/`classifyNotesFailure` and the `CauseText` "diff too large" phrase remain. Task 2-5's plan text removes all of these, so they are not findings here.
- Round 1's three comment corrections are applied verbatim, and the quoted-path fix and its test behave as claimed.
- Nothing rode the dispatch beyond the enumerated inputs.
