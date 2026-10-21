## Attempt 1

ISSUES:
- /Users/leeovery/Code/mint/internal/notes/exclusioncondense_test.go:166-182 (AC7). No test has an uncondensed check prompt over the cap whose condensed form fits, and that is the task's whole Problem. At capBytes 20,000 the uncondensed list (100 top-level plus 50 vendor lines) is about 5.2 KB, so the prompt is about 6.2 KB and would be sent condensed or not. Mutation (a), which fails the check on exactly the vendored-tree releases this task exists for, survives every test in the repo.
  FIX: Raise the vendor run in this test so the uncondensed list alone exceeds the cap, e.g. `listedFiles(t, "vendor/pkg/file%03d.go", 500, 200)`. That gives about 21 KB of vendor lines plus 3.2 KB of top-level lines. Condensed, the list is about 3.2 KB: over the 2,000-byte share, and the prompt (about 4.2 KB) is under 20,000. Expect `topLines` plus `"- vendor/: 500 files, "+bytesize.Format(500*200)`. Add a precondition that fails the test unless `len(strings.Join(fullLines, "\n")) > capBytes`, building fullLines from listedFiles' returned lines, so the fixture cannot drift back to fitting uncondensed.
  CONFIDENCE: high
- /Users/leeovery/Code/mint/internal/notes/exclusioncondense_test.go:184-223 (AC8). Both subcases use cap 900, but composeCheckPrompt's fixed text alone is 935 bytes. Both refusals therefore come from the instructions, never from the list, and the subcase named "a list condensed as far as it goes" proves nothing about the list. Mutation (b), which ignores the list's size entirely and would send an over-cap call whenever the instructions fit, survives every test. The engine test at /Users/leeovery/Code/mint/internal/engine/release_exclusioncondense_test.go:33 has the same property (cap 900, "below the check's own instructions"). That is fine for its engine-level purpose; the gap needs closing at the notes level.
  FIX: Replace the second subcase with one where the instructions fit and the condensed list does not. For example, cap 3,000 with `listedFiles(t, "file%03d.txt", 100, 200)`: top-level paths cannot condense, so the list stays about 3.2 KB and the prompt about 4.1 KB, over 3,000, while the instructions (935) are under it. Give each table entry its own cap field. Keep the existing first subcase at 900, which covers instructions over the cap.
  CONFIDENCE: high
- /Users/leeovery/Code/mint/internal/notes/exclusioncheck.go:314. On a condensed list the prompt still tells the AI "Each line below is one changed path, with its change and the size of its diff:", then gives it `- vendor/: 4,312 files, 38 MB`. That is no path, has no change, and uses the prompt's own "glob: reason" answer shape. So the AI's input is false on exactly the releases this task targets (Outcome: "a list the AI can read whole").
  FIX: Have checkList report whether it condensed, e.g. return `(list string, condensed bool)`, and have composeCheckPrompt add a clause only when it did, e.g. "Each line below is one changed path, with its change and the size of its diff, or one folder's changed paths summed up as how many and the total size of their diffs:". An un-condensed prompt stays byte-identical, which keeps AC1 and every existing check test unchanged. Add one assertion to the AC2 test that the condensed prompt carries the clause, and one to the AC1 test that the un-condensed prompt does not. The cap guard already measures the composed prompt, so the extra bytes are counted.
  ALTERNATIVE: Reword the line unconditionally, e.g. "Each line below is a changed path, or a folder of them summed up, …". This is simpler, but every prompt changes, so check-prompt pins elsewhere need updating. I recommend the conditional form: what un-condensed releases get stays exactly as before.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/notes/exclusioncheck.go:31-32 — false since this change: a condensed call carries folder summary lines, not each path with its change and size
  OLD: // its notes. Its call carries no diff, only the changed paths the notes diff carries
// that the keep list does not cover, each with its change and the size of its diff.
  NEW: // its notes. Its call carries no diff, only the changed paths the notes diff carries
// that the keep list does not cover.
- /Users/leeovery/Code/mint/internal/notes/exclusioncheck.go:89 — "a listed path" is now loose, since a path under a summary line is not listed on its own line; this matches the rewording the diff already made in Check's doc
  OLD: // suggestion matching a listed path, in the answer's order, with what it matches
  NEW: // suggestion matching a path the list covers, in the answer's order, with what it matches

NOTES:
- When the call does not fit, Check emits CheckFailed with no Checking/Checked, so neither pretty nor plain mode shows the "Checking…" line. §16.1 says the check line shows "on every release that runs the check". Reading a no-call check as not running is defensible, and the notes test pins it (narration == ["failed: "]).
- Runs key on the leading path segment only, the Change Map's rule as the task directs. A repo with everything under one root (e.g. `internal/`) that adds a large `internal/testdata/` collapses its whole tree into one `internal/: N files` line, and the AI can no longer single out the subfolder. §6.3 leaves the choice of folders to the implementer, so this is not a finding, but it caps how useful the check is on such repos.
- The determinism test (exclusioncondense_test.go:124) checks that something condensed with an OR of two substrings. Byte-identity across 20 runs is the real assertion and is exact, and which tied folder collapses is the implementer's choice, so this is acceptable.
- The dispatch held only the enumerated inputs; nothing extra came with it.
