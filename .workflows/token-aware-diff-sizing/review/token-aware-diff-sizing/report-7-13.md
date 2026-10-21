TASK: Condense the check's path list by folder (token-aware-diff-sizing-7-13)

ACCEPTANCE CRITERIA:
- Given a list that takes at most a tenth of the cap, the check's call lists every path on its own line (§6.3)
- Given a release adding 4,312 files under `vendor/` whose diffs total 38,000,000 bytes, so that its list would take more than a tenth of the cap, the check's call carries `vendor/: 4,312 files, 38 MB` in place of those 4,312 entries (§6.2, §6.3)
- Given a 4,312-file run under `vendor/` and a 12-file run under `docs/`, where collapsing `vendor/` alone brings the list under its share, `vendor/` collapses and the `docs/` files stay listed one per line (§6.3)
- Given the same release on two runs, the check's list is byte-identical both times (§6.3)
- Given `max_prompt_bytes` set, the share is a tenth of the configured cap: a list sent whole under the default condenses under a cap small enough that the list takes more than a tenth of it (§6.3)
- Given the AI suggests `vendor/` from that condensed line, the suggestion survives, and its pause counts the 4,312 files and their `38 MB` (§6.2, §6.4, §6.5)
- Given a list that, condensed as far as it goes, still takes more than a tenth of the cap but leaves the check's call within the cap, the call is sent with the condensed list (§6.2)
- Given a cap so small that the check's call cannot fit under it even with the list condensed, no check call is sent; mint warns `couldn't check for files that don't belong in the notes` with no output beneath it, and the release carries on with no suggestions (§4.1, §6.2, §6.8)

STATUS: issues_found

SPEC CONTEXT: §6.2 has the exclusion check send one call carrying only the uncovered changed-path list (path, change, diff size); past a tenth of `max_prompt_bytes` the list condenses by folder (§6.3: largest runs first, deterministic, until under its share; a list under its share renders as before), and a list that cannot get the call under the cap even condensed is a failed check (§6.8: one warning, AI output beneath it only where there is any, release carries on). §4.1: no call is ever bigger than the cap. §6.4: a suggestion survives by matching an uncovered changed path, so a folder glob from a summary line matches every path under it. §16.1's corrigendum sets the decimal size format the summary line uses.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/condense.go:10-131 — the shared folder-condensing rule, extracted from the Change Map: `condenseShare` (cap/10), `foldLine`/`runKey`/`runOf` (leading-directory runs), `condenseRuns` (collapse in rank order until `excess` is saved; the summary takes the run's first line's place), `rankRuns` (runs of 2+ lines whose summary is shorter; sorted by count desc, then kind, then folder — a total order over unique keys, so map iteration never leaks into the output)
  - internal/notes/exclusioncheck.go:304-321 — `checkList`: renders the full list, returns it unchanged when `len(full) <= share`, otherwise condenses with `excess = len(full) - share`; the per-run saving (`len(text)+3` per line) is exact against `renderCheckList`'s `"- "` lead and `"\n"` join
  - internal/notes/exclusioncheck.go:330-332 — `summarizeCheckRun`: `{folder}/: {n with thousands commas} files, {bytesize.Format(sum of diff bytes)}` → `vendor/: 4,312 files, 38 MB`
  - internal/notes/exclusioncheck.go:342-346 — the condensed prompt's intro gains a clause explaining summary lines
  - internal/notes/exclusioncheck.go:129-133 — composes the prompt, and when it exceeds `maxPromptBytes` sends no call and narrates `CheckFailed("")`, returning no suggestions
  - internal/notes/exclusioncheck.go:147 — suggestions are matched against `paths` (every uncovered changed path), not against list lines, so `vendor/` from a summary line survives and its pause counts every file under it
  - internal/notes/select.go:213, 249-253 — threads `cfg.MaxPromptBytes` (the resolved configured cap, default 1,500,000) into `Check`
  - internal/engine/notesstage.go:51-58 — the failed-check warning (task 7-12) renders the empty output as no output
- Notes: Every criterion holds on reading. Running condensing at leading-directory granularity follows the task's "same rule as the Change Map", and §6.3 leaves the choice of folder to the implementer. The Change Map's refactor onto the shared helper keeps its behaviour: same ranking, same overhead, same summary text via `summarizeNoveltyRun`.

TESTS:
- Status: Adequate
- Coverage:
  - AC1: internal/notes/exclusioncondense_test.go:84 — 40 paths under the default cap listed one per line, with no condensed-intro clause
  - AC2: :98 — 4,312 vendor files at 8,813 bytes each (38,001,656 bytes) plus one top-level file; the list is exactly `[src/main.go line, "- vendor/: 4,312 files, 38 MB"]`, and the condensed intro is asserted
  - AC3: :115 — 12 docs files plus the vendor run; docs stay one per line and vendor collapses. This would fail both under collapse-everything and under smallest-first ranking
  - AC4: :126 — two equal 30-file runs where collapsing either one is enough; 21 runs must agree, which exercises the tie-break against Go's randomised map order
  - AC5: :147 (notes level: whole under the default, condensed under a 10,000-byte cap) and internal/engine/release_exclusioncondense_test.go:13 (`max_prompt_bytes = 20000` in `.mint.toml` reaches the check through the engine and condenses 80 files)
  - AC6: :162 — the `vendor/` suggestion survives, and its pause carries 4,312 paths and `38 MB`
  - AC7: :184 — 100 top-level files (which cannot condense) plus 500 vendor files; the uncondensed list alone would exceed the cap, and the condensed list is sent while still over its share
  - AC8: :205 (table: instructions alone over the cap; a list that cannot be condensed) — no transport call, narration exactly `["failed: "]`, no pauses, no exclusions; and internal/engine/release_exclusioncondense_test.go:33 — no check call, one warning with empty Output, and the release completes on the commit-subject fallback
  - `sentPrompt` also asserts every sent prompt is at or under the cap
- Notes: The tests are focused and each maps to one criterion. Calls in pre-existing tests were updated mechanically to the new `Check(ctx, diff, maxPromptBytes)` signature with `config.DefaultMaxPromptBytes`, which leaves their assertions unchanged.

CODE QUALITY:
- Project conventions: Followed (external test packages, t.Parallel, table-driven where the shape fits, exact-line assertions on the sent prompt; presenter is the only output surface via CheckNarration)
- SOLID principles: Good — the condensing rule is one helper parameterised by a `summarizeRun` func, shared by both texts as §6.3 requires
- Complexity: Low
- Modern idioms: Yes (range-over-int, strings.Lines, slices.*)
- Readability: Good
- Issues: one doc comment the new no-fit path falsifies (see FINDINGS)

BLOCKING ISSUES:
- None

FINDINGS:
- [in-scope] [contained] internal/notes/exclusioncheck.go:15-17 — The `CheckNarration` doc says it is told "when the exclusion check's call starts and when it ends, and, after the end, when the check failed". This task's no-fit path (internal/notes/exclusioncheck.go:130-131) calls `CheckFailed("")` with no `Checking`/`Checked` before it, and the test at internal/notes/exclusioncondense_test.go:245 pins that order (`["failed: "]`). Reword the doc so it covers both cases, e.g. "CheckNarration is told when the exclusion check's call starts and when it ends, and when the check failed: after the call ends when the call failed or its answer was unreadable, and with no call at all when the call would not fit under the cap. output is the AI's captured output, or its unreadable answer, verbatim but for trailing whitespace, and empty when there is none." — FAILS: the interface contract states an ordering the code breaks. A `CheckNarration` implementation that relies on the documented order would mis-narrate a check whose call was never sent, for example by closing a check line in `CheckFailed` that `Checking` never opened. CLAUDE.md requires comments to change in the same commit as the behaviour they describe.

UNSETTLED:
- None
