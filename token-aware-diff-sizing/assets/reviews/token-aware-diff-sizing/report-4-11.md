TASK: Mark the files the saved notes are missing (token-aware-diff-sizing-4-11)

ACCEPTANCE CRITERIA:
- Given a 3-part wave that fails for good at part 3, which carries `api/handler.go` and `api/routes.go`, after parts 1 and 2 finished, the file's first line is `**Notes missing:** no notes were written for {files} — add them before releasing.`, with `{files}` naming both files. A blank line follows, then parts 1 and 2's notes (§11.4)
- Given parts the wave never started before it ended, or in-flight calls that failed after it ended, their files are listed in the marker too (§11.2, §11.4)
- Given a file sent as pieces, some of which finished and some not, the marker lists it marked `(partly)`. A file none of whose pieces finished is listed unmarked (§11.4)
- Given a slice that halved, files whose halved parts finished are not listed and files whose halved parts did not finish are. A file the re-cut split into pieces, some of which finished, is marked `(partly)` (§10.1, §11.4)
- Given every part finished and the merge failed, the file has no marker and opens with part 1's notes (§11.3, §11.4)
- The README's description of the saved file names the marker and says to fill in the files it lists before releasing (§11.4, §18.2)

STATUS: complete

SPEC CONTEXT: §11.4 puts a one-line bold marker at the top of the saved chunk-notes file whenever any slice has no notes, listing every such file and marking `(partly)` a file whose pieces only partly failed, because the file is picked up later (floor `e`, `--notes-file`) possibly after terminal output is gone. §11.2: after the first final failure no new calls start, in-flight calls finish and their notes are kept. §10.1: a SIZE answer re-cuts a slice into halved parts (possibly pieces) that take its place in chunk order. §11.3: a failed merge saves every chunk's notes. §18.2: the README documents the saved file.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/chunknotes.go:48-58 — keepChunkNotes prefixes `notesMissingLine(...) + "\n\n"` to the joined finished notes when any part has none
  - internal/notes/chunknotes.go:98-129 — notesMissingLine: walks parts in chunk order, records per path whether some part carrying it has notes and whether some has none; lists every path with a missing part, appending ` (partly)` when the path also has a written part; returns "" when nothing is missing
  - internal/notes/chunknotes.go:132-142 — chunkPart.paths: a piece's file, or every file section's headerPath for a whole-file chunk
  - internal/notes/chunk.go:57-66 — writeParts passes the wave's final slots (`sent`) and their notes to keepChunkNotes for both a wave failure and a merge failure
  - internal/notes/chunk.go:168-193 — wave.run returns every slot (including queued, never-started ones and halved replacements in their slice's place) with "" for a part with no notes
  - internal/notes/chunk.go:226-240 — a call succeeding after the wave ended still records its notes; a call failing after the end leaves "" (listed)
  - internal/engine/chunknotes.go:33-40 — the engine's save closure writes the marker-bearing string verbatim
  - README.md:131 — names the marker line, what it lists, `(partly)`, the no-marker merge-failure case, and "Tidy the notes, filling in the files the marker lists and removing the marker line, before releasing them with `--notes-file`"; README.md:163 carries it to regenerate's fresh-source file
- Notes: Each criterion traces to the code. Never-started parts and halved replacements that never ran stay in `w.slots` with "" notes, so their files are listed; a halved slice's finished replacement parts mark their files written; a whole-file chunk re-cut into pieces yields pieces carrying `file`, so mixed outcomes produce `(partly)`. A merge failure reaches keepChunkNotes with every part's notes non-empty, so notesMissingLine returns "" and the file opens with part 1's notes. A successful call can never leave "" because the AI transport rejects empty/whitespace bodies (internal/ai/transport.go:326), so "" reliably means "no notes". The diff is assembled with pinned `--src-prefix=a/ --dst-prefix=b/` (internal/notes/assemble.go:184), so headerPath yields repo paths. The `{files}` separator (backtick code spans joined by ", ") is the implementer's choice per the task context.

TESTS:
- Status: Adequate
- Coverage (internal/notes/notesmissing_test.go):
  - :21 WaveFailsForGoodAtItsLastPart — fixture confirms `api/handler.go` + `api/routes.go` framed as part 3 of 3; asserts the exact saved string: marker naming both, blank line, parts 1 and 2's notes (criterion 1)
  - :58 WaveEnds_MissingFilesIncludeUnstartedPartsAndCallsThatFailedAfterTheEnd — area4 fails after the end, areas 5–6 never start (asserted); marker lists area1, area4, area5, area6 (criterion 2)
  - :111 FileSentAsPiecesPartlyFinished — `src/partly.go` (partly) and fully-failed `src/unwritten.go` unmarked (criterion 3); with exactly four pieces and the dispatch loop holding the lock until inFlight reaches the bound, all four calls start, so the expected saved body is deterministic
  - :128 SliceHalved_ListsOnlyTheFilesWhoseHalvedPartsDidNotFinish — only `area2/c.go` listed, its halved siblings' notes kept (criterion 4, first half)
  - :173 FileTheReCutSplitIntoPiecesPartlyFinished — `big/one.go` (partly) after a re-cut into two pieces (criterion 4, second half)
  - :216 EveryPartFinishedAndTheMergeFailed — exact saved string equals the joined notes with no marker (criterion 5)
  - :236 PrefixedHeaders — marker names the repo path from a/ b/ headers
  - Engine-level tests carry the marker through the file write (internal/engine/notes_savedchunknotes_test.go:113, internal/engine/release_regenchunkfail_test.go:189, internal/engine/regenerate_regenfail_test.go:207)
- Notes: Every test asserts the exact saved string, so a broken marker, separator, ordering, `(partly)` rule or blank line fails them. Not over-tested: each test covers a distinct criterion; the overlap with savenotes_test.go:136 (which also asserts a marker) is incidental to that task-4-10 test's own subject. The README criterion is documentation and needs no test.

CODE QUALITY:
- Project conventions: Followed (external test package, t.Parallel, scripted transports, exact-output assertions; WHY-comments accurate to the code)
- SOLID principles: Good — marker composition is isolated in notesMissingLine; path extraction is a method on chunkPart
- Complexity: Low
- Modern idioms: Yes (slices.DeleteFunc/Clone, strings.Builder)
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
