# Implementation Review: Token-Aware Diff Sizing

**Plan**: token-aware-diff-sizing
**Verdict**: Fail

## Summary

All 104 plan tasks across 12 phases are delivered. Per-task verification found one blocking defect (7-12): in pretty mode a failed exclusion check stopped the notes stage's live line for the rest of the stage, so nothing showed while the notes were written. The change-set verification found one in-scope product defect that no single task could see: the notes diff disabled neither colour nor an external diff tool, so `color.diff=always` or `diff.external` stripped the per-file headers that the exclusion check, chunking and the saved chunk notes all read. Both are corrected in this session, together with twelve other contained corrections: stale or false comments left by the feature's own changes, an inert "no tag" test guard, regenerate's untestable chunk-notes wiring, and two missing cmd-level tests. All five gates pass on the corrected tree. The review fails on one item. A wall-clock test in the chunk wave flakes under CPU load, and choosing its fix (re-tuned margins or a deterministic deadline comparison) is a decision about what the test pins, so it goes back to planning. Real-git runs against throwaway repos settled the `--notes-file` keep-out, unwind, tag-byte and ignore-check behaviours. Sixteen criteria were not measured by reading or by the change-set verification; thirteen of them are five-gate or suite criteria, and the gates pass on the corrected tree.

## QA Verification

### Specification Compliance

#### 1. Purpose and Scope · 2. The Lean Default AI Command · 3. Quote-Aware `ai_command` Parsing · 4. The Per-Call Size Cap · 5. Per-Verb Exclusion Lists · 6. The Exclusion Check · 7. Routing a Release's Notes · 8. Chunked Generation
- The lean default command is exactly the §2.1 string, and the only floor of the resolution chain — internal/config/config.go:89, :583-605 — read: AICommandFor trims to test for blank, returns the raw value, and the default is the last candidate; an explicit value at any level is untouched (§2.4).
- POSIX splitting rules (§3.1): whitespace, single quotes, double quotes escaping only `"`, `\`, `$`, backtick and newline, backslash outside quotes, backslash-newline continuation, adjacent parts joining, and `""`/`''` as one empty word; no other shell syntax — internal/cmdline/split.go:27-122 — read: `inWord` is set by a quote alone, so `--tools ""` yields an empty argument; `$`, `*`, `|` etc. fall to `appendRune`.
- The transport splits by those rules and runs the program directly through the runner — internal/ai/transport.go:333-343, :215-218 — read.
- Malformed commands fail at load at every level, whatever the verb, with the three §3.2 messages; a blank command is skipped, per the corrigendum — internal/config/config.go:844-879, called from Load at :428-430 — read: `[commit]` is checked on a release run too, and `'' --flag` hits `words[0] == ""`.
- The §3.3 whitespace-split comments were rewritten at every cited site — internal/ai/transport.go:112, :192-193, :329-332; internal/config/config.go:121-123, :576-577; internal/aitransport/aitransport.go:31 — read; `grep -i 'whitespace-split\|whitespace split\|by whitespace\|strings.Fields split'` over the three packages' non-test files finds none left about ai_command.
- Release and regenerate resolve through VerbRelease (180s, `[release]` → shared), commit through VerbCommit (60s) — internal/config/config.go:94-96, :642-670; internal/engine/release.go:1317-1322; internal/engine/regenerate_fresh.go:152; internal/commit/run.go:822 — read.
- `max_prompt_bytes`: shared-only, default 1,500,000; `<= 0` is rejected with the §4.1 message; a float or string gets the friendly type message — internal/config/config.go:70, :325, :431-433, :473, :834-839, :508-539 — read.
- The removed `max_diff_lines` names its replacement in the §4.6 wording — internal/config/config.go:550-551 — read.
- New keys take the strict path: schema fields, `typeErrorMessages` entries and SoT rows for `max_prompt_bytes`, `[release]`/`[commit]` `diff_exclude` and `[release].diff_keep` — internal/config/config.go:325-364, :473-486; internal/config/metadata.go:118-151 — read.
- Every call is bounded by the cap (§4.1):
  - single-pass size check: internal/notes/generate.go:227
  - chunk and piece calls: room = target − fixedPart, with framing numbered at the bound — internal/notes/chunk.go:369-371, :377-381, :439-444
  - merge: internal/notes/chunk.go:70-72
  - re-merge: internal/notes/remerge.go:53
  - classify call: internal/notes/classify.go:59
  - exclusion-check call: internal/notes/exclusioncheck.go:129-133
  - commit: internal/commit/generate.go:150, :164-170
  - read: each compares the composed prompt's length with the cap before sending.
- Commit over the cap goes to `$EDITOR` with `diff too large to summarise — opening editor`, including an over-cap `r` nudge — internal/commit/run.go:56, :357-359, :680-682 — read.
- Exclusion levels add up: release notes get shared + `[release]` + built-ins + the plain-mode version_file; commit gets shared + `[commit]`; neither reads the other's level; commit's preflight ignores every level — internal/engine/regenerate_fresh.go:127-133; internal/notes/assemble.go:29-32, :117-128; internal/commit/generate.go:126, :131-140; internal/commit/source.go:108-128 — read.
- `.mint.toml` is a built-in exclude beside CHANGELOG.md, on both the forward and regenerate paths — internal/notes/assemble.go:29-32 — read; both paths build the Assembler from notesExcludeConfig (internal/engine/release.go:1016, internal/engine/regenerate_fresh.go:121).
- The diff and Change Map share one exclusion set and run at the repo root — internal/notes/changemap.go:48-66; internal/notes/assemble.go:183-191 — read.
- Routing precedence (§7.1) is supplied, then first release, then empty diff, then `--no-ai`, then the AI path; the AI path (§7.2) is check, then cache reuse, then size check — internal/notes/select.go:183-245 — read: the check and reuse both sit after the degenerate and no-ai branches, and GenerateFromDiff does the size check.
- Exclude answers re-assemble the diff, and the Change Map, the cache key and later `r` calls all see them through the shared Assembler; a diff they empty gets the maintenance body — internal/notes/select.go:214-220, :249-259; internal/notes/assemble.go:106-108 — read.
- Regenerate runs no check and touches no cache (§6.1, §7.2) — internal/engine/regenerate_fresh.go:66, :100 — read: GenerateFromRange is called directly, and a grep for NoteCache/ExclusionCheck in the engine finds only release.go.
- When the check runs (§6.1): it is skipped under `--notes-file`, on a first release, for an empty diff and under `--no-ai` (precedence above), and when keep covers every path — internal/notes/exclusioncheck.go:124-128 — read.
- The check's list (§6.2): uncovered paths from the post-exclusion diff minus the keep list, each with change status and diff size; release command and timeout; no diff carried — internal/notes/exclusioncheck.go:124-129, :273-302, :342-361; internal/engine/release.go:1017, :1064 — read.
- Condensing by folder (§6.3), for both the check list and the Change Map: past a tenth of the cap, largest runs first with deterministic ties, under-share text unchanged — internal/notes/condense.go:10-12, :55-118; internal/notes/changemap.go:175-186; internal/notes/exclusioncheck.go:306-321 — read.
- Reading the answer (§6.4): "none" is case-folded; suggestions must match an uncovered path; no usable suggestion means a failed check — internal/notes/exclusioncheck.go:144-151, :249-263 — read.
- Pauses (§6.5):
  - one gate per suggestion, in order
  - a suggestion is skipped when earlier answers cover every uncovered path it matches
  - matches count paths still in the diff, kept files included and listed
  - `e`/`k` keys, Enter = keep
  - internal/notes/exclusioncheck.go:176-241; internal/engine/release.go:1188-1236; internal/presenter/gate.go:32-35 — read.
- Recording (§6.6):
  - answers are held until the last pause and written to `[release]` only — header, inline and dotted forms preserved, other bytes untouched
  - `.mint.toml` is staged by path and committed with the constant subject
  - the ignored-file route writes without committing
  - narration line
  - internal/notes/exclusioncheck.go:191-196; internal/config/appendrelease.go:35-49, :130-159, :254-358; internal/engine/release.go:1090-1133; internal/record/commit.go:107-131, :213-225 — read, including go-toml v2.3.1's Raw ranges (KeyValue spans the whole expression; InlineTable spans `{`), which addInlineKeys relies on.
- Pause, recording and unwind failures (§6.6):
  - Ctrl-C after a pause returns the cancel and records nothing
  - closed stdin aborts and unwinds without recording (it is not a generation cause, so it does not reach the floor)
  - a failed write, check-ignore or commit is surfaced with the write error or git's stderr and reverted by the unwind
  - a landed answers commit is counted and re-committed after the reset
  - a failed reset or re-commit warns with the restore step and drops "kept" and "repo clean"
  - internal/notes/exclusioncheck.go:183-188; internal/engine/notesfloor.go:32-40; internal/engine/release.go:550-560, :1999-2006; internal/engine/unwind.go:114-207, :242-245, :291-323, :353-379 — read.
- Commit-subject bodies leave out answers commits — internal/notes/resolve.go:110 — read.
- Unattended and dry runs (§6.7): an unattended run reports after the notes, also with the fallback list and with the floor's next moves, each with the `[release]` `diff_exclude` line built from the config as loaded; an attended dry run previews at the pause position; nothing is written in either — internal/engine/release.go:1063-1073, :570-573, :588-593; internal/engine/notesstage.go:86-111; internal/engine/notesfloor.go:18-26; internal/presenter/plain.go:226-241 — read.
- A failed check (§6.8): every one of the five cases warns `couldn't check for files that don't belong in the notes` with the AI's output; no classifier runs; a cancel propagates — internal/notes/exclusioncheck.go:129-151, :161-167; internal/engine/notesstage.go:51-59 — read.
- Chunked generation, map-reduce (§8.1, §8.2, §8.5): parallel chunk calls, then exactly one merge; the same Change Map in every call; at most four calls in flight; the wave drains before the merge; no concatenation fallback — internal/notes/chunk.go:28-35, :57-80, :116, :168-197 — read: `canSend` caps inFlight at 4; the run loop exits only at inFlight == 0; a failure returns an error, never joined notes.
- Packing and pieces (§8.3, §8.4): path order; fill until the next file won't fit; a file too big for a chunk becomes pieces, cut at hunks, then lines, then UTF-8 character boundaries; each piece repeats the header and names k of n; no bytes are dropped — internal/notes/chunk.go:280-294, :377-406, :419-433, :439-515 — read.
- What each call is told (§8.6): instructions, map, framing, slice and reminder for a chunk; instructions, map, framing, the notes in chunk order and reminder for the merge; the merge body passes through verbatim — internal/notes/chunk.go:542-575 — read.
- Calls that cannot be made (§8.7): ErrNoRoomForDiff when the fixed part fills the cap or the halved target; ErrMergeTooLarge for an over-cap merge — internal/notes/chunk.go:378-381, :441-444, :70-72; internal/notes/size.go:12-19 — read.
- Chunking has no setting and no consent prompt (§8.8) — internal/config/config.go:321-366 — read: no oversize key exists.
- Size narration uses the decimal format of the §16.1 corrigendum, for the check list, the pauses and the split line — internal/bytesize/bytesize.go:10-26 — read.
- The four non-test gates — measured, from the repo root:
  - `go build ./...` → ok
  - `gofmt -l .` → no output
  - `go vet ./...` → ok
  - `golangci-lint run` → `0 issues.`

#### 9. Failure Classification · 10. Halving After an Under-Cap Overflow · 11. Failures in a Chunked Run · 12. The Remediation Floor
- Lint and build gates are clean over the delivered tree — project root — measured: `go build ./...` → exit 0; `gofmt -l .` → empty; `go vet ./...` → exit 0; `golangci-lint run` → "0 issues."
- §9.1 trigger: only a generation failure (a *ai.GenerationError, non-zero exit or empty body after the retry) is classified. A timeout, a missing binary, a split error or a cancel reads OTHER with no classify call — internal/notes/classify.go:53-57; internal/ai/transport.go:214-259, 297-316 — read
- §9.5 no error text: a GenerationError whose streams are empty or whitespace-only skips classify, because Output() trims each stream only to test emptiness — internal/notes/classify.go:55; internal/ai/transport.go:97-105 — read; pinned by TestRelease_NotesCallFails_ClassifiesOnlyWhenEitherStreamHoldsText (internal/engine/notes_classifyskip_test.go:79)
- §9.2/corrigendum: a classify prompt over the cap is not sent and reads OTHER — internal/notes/classify.go:58-61 — read; pinned by TestGenerator_ClassifyPromptAgainstTheCap_SentOnlyWhenItFits and TestRelease_CapBelowTheClassifyPrompt_NoClassifyCallAndTheOriginalOutputShows
- §9.2 prompt shape: it carries both codes with one-line glosses, both streams fenced as captured output that is not instructions, and the one-code instruction — internal/notes/classify.go:87-108 — read
- §9.2 trimming: a stream over 4,000 bytes keeps its first and last 2,000 bytes cut at character boundaries, with an omitted-byte count line. The head stops before a straddling character and the tail starts after one — internal/notes/classify.go:113-137 — read; pinned by the StreamOver4000Bytes and CutInsideMultiByteCharacter tests (internal/notes/classify_test.go:231, 275)
- §9.2 the classify call uses the same transport, so it runs the release's ai_command and timeout. Regenerate's transport resolves through VerbRelease — internal/notes/classify.go:62; internal/engine/regenerate_fresh.go:148-153 — read
- §9.4 answer reading: leading whitespace, quotes, backticks and curly quotes are stripped; at the end, those plus any punctuation are stripped. The remainder is then compared case-insensitively; only SIZE is SIZE and anything else is OTHER — internal/notes/classify.go:74-85 — read; pinned by the seven-answer table in internal/notes/halve_test.go:80 and the "sentence"/"both codes" cases at :206-207
- §9.5 classify-call failure: a generation failure, timeout or missing binary on the classify call reads OTHER, and readFailure returns the original call's failure, so the ✗ output is the notes call's own — internal/notes/classify.go:41-47, 66-68 — read; pinned by TestRelease_ClassifyCallFails_TheLineShowsTheNotesCallsOutput
- §9.5 cancel during classify: it is returned in place of the failure and propagates as context.Canceled — internal/notes/classify.go:43-45, 63-65 — read; pinned by TestRelease_CtrlCDuringAClassifyCall_Fallback_AbortsWithNoFurtherCall
- §9.5 the single retry stays for every call, classify included, and the transport is unchanged — internal/ai/transport.go:222-258 — read
- §9.1 scope: classify runs only from the Generator's notes calls (single-pass, chunk, merge and re-merge). The exclusion check calls the transport directly and commit has no classify — internal/notes/exclusioncheck.go:135; internal/notes/chunk.go:83, 210-214; internal/notes/generate.go:232 — read: `grep -n 'Generate(\|readFailure\|\.generate('` over internal/notes, and `grep -i classif` over internal/commit (no classifier)
- §10.1 single-pass SIZE: it is re-cut at half the composed single-pass prompt and becomes a chunked run that counts as the first halving. If that round has no room, it fails as ErrTooLargeForModel with no location and sends no call — internal/notes/generate.go:237-240; internal/notes/chunk.go:41-48 — read
- §10.1 chunk SIZE: the slice is re-cut at len(prompt)/2 by the first wave's packing and piece rules (recut → cutParts/cutPieces), and the new parts take the slice's place in chunk order — internal/notes/chunk.go:217-220, 253-275, 410-415 — read
- §10.1 merge SIZE is never halved: it fails as ErrMergeTooLarge and carries the call's own error — internal/notes/chunk.go:82-91 — read; pinned by TestGenerator_MergeClassifiedSize_FailsAsTooLargeToMergeWithNoFurtherCall
- §10.2 bound: initial parts carry halvings 0, single-pass parts 1, and each re-cut adds 1. Halving is allowed while halvings < 3, so a fourth SIZE fails the slice as ErrTooLargeForModel naming its part — internal/notes/chunk.go:112, 147-151, 217, 236-237, 256 — read; pinned by TestGenerator_ChunkHalvedThreeTimes_FourthSizeFailsTheSliceNamingItsPart (internal/notes/halvebound_test.go), which asserts 4 overflows, 4 classifies and "(part 3 of 12)"
- §10.3 hint: only the single-pass, chunk and piece calls report CallSucceeded with their whole prompt. The merge, classify and exclusion-check calls do not. The hint prints only when a halved run's merge wrote the body, as the exact byte count — internal/notes/generate.go:233-235; internal/notes/chunk.go:230; internal/engine/notesstage.go:78-80, 143-152, 272-280; internal/engine/release.go:592 — read
- §10.3 regenerate --all: the hint prints once, at the end of the batch, on every exit path: a per-version error (decline, cancel, surfaced failure) at :204 and normal completion at :218. A batch gate passes no LowerCap, so no version prints its own — internal/engine/regenerate_batch.go:204, 218, 319, 330, 338, 467; internal/engine/notesstage.go:240-268 — read
- §11.1/§11.2 wave end: any failure that is not SIZE with halving left records the first final failure. canSend then stops new calls, a later failure is neither classified (send checks w.ended()) nor recorded, and calls already in flight still record their notes — internal/notes/chunk.go:168-197, 209-248 — read; pinned by the seven internal/notes/waveend_test.go tests, including TestGenerator_Wave_CallInFlightFailsAfterTheWaveEnded_IsNotClassified, which asserts no classify of the later failure and no new part start
- §11.1 no merge after a failed chunk: writeParts merges only when run returns nil — internal/notes/chunk.go:57-66 — read
- §11.3 failed merge: a prompt over the cap (ErrMergeTooLarge, no call), a SIZE answer (ErrMergeTooLarge wrapping the call's failure) or any other failure (callError "merging") all go through keepChunkNotes, so every chunk's notes are saved — internal/notes/chunk.go:57-91 — read; pinned by TestGenerator_MergeFails_SavesEveryPartsNotesAndNothingElse
- §11.4 contents: finished notes in chunk order, byte for byte, with one blank line between. When any part has none, the file opens with the bold "**Notes missing:**" line listing every file without notes (a partly-failed piece file marked "(partly)"), then a blank line — internal/notes/chunknotes.go:48-58, 81-129 — read; pinned by internal/notes/savenotes_test.go and notesmissing_test.go
- §11.4 no file when no chunk finished, and none on a cancel: keepChunkNotes returns the failure unchanged when nothing finished, and saveFailedChunkNotes skips a context.Canceled — internal/notes/chunknotes.go:49-52, 68-77 — read; pinned by TestGenerator_NoPartFinished_SavesNothingAndNamesNoFile, TestGenerator_CtrlCWhileChunkCallsAreInFlight_SavesNothing and TestGenerator_CtrlCAtTheMerge_SavesNothing
- §11.4 location: {UserCacheDir}/mint/chunk-notes/{repoScope(root)}/{tag}.md. It is overwritten per tag, separate from the note cache's mint/cache (whose Prune cannot reach it), and wired for release and regenerate — internal/notescache/chunknotes.go:30-55; internal/notescache/cache.go:190-192; cmd/mint/main.go:193, 305, 475 — read
- §11.4/corrigendum save failure: the AI failure is returned unchanged. The keeper records the save error, and warnUnsaved emits one warning with the error beneath it after the ✗ line, on release, regenerate and batch skip — internal/notes/chunknotes.go:72-75; internal/engine/chunknotes.go:33-64; internal/engine/release.go:568, 582; internal/engine/regenerate_interactive.go:240 — read
- §11.5 Ctrl-C mid-wave: run returns ctx.Err() with no parts or notes ahead of any earlier failure, nothing is saved, and ResolveFailure returns the cancel unchanged under fallback — internal/notes/chunk.go:184-186; internal/notes/resolve.go:55-57 — read
- §11.6 per-call timeouts: the wave sets no deadline of its own, the transport gives each attempt context.WithTimeout, and a chunk timeout is a final failure with no classify call — internal/notes/chunk.go:118-122; internal/ai/transport.go:282-291 — read
- §12.1 release routing: a cancel or a non-generation error never reaches the floor (reachesNotesFloor). An attended real run gets the c/e/a choice; an unattended or dry run gets the ✗ line, output, saved pointer and next moves, then Unwind. Under fallback, the commit-subject list ships with one warning (cause phrase, output, both --notes-file commands, or only the release command on a dry run) — internal/engine/notesfloor.go:17-38; internal/engine/release.go:563-587; internal/engine/notesstage.go:113-139 — read
- §12.1/corrigendum fallback move: it shows only when the run is not attended, so an attended dry run omits it — internal/engine/notesfloor.go:44-53 — read
- §12.2 choice: `c` gives the commit subjects; `e` edits a copy of the saved chunk notes, or an empty body. An empty or whitespace `e` and a missing editor (ErrEditorReturnToGate) re-present the choice, while any other editor error is surfaced and returned. `a` is the default. Every error leads to Unwind on release, and the chosen body takes KindFloor's y/n/e gate, which has no `r` — internal/engine/notesfloor.go:128-192; internal/engine/release.go:576-579, 1842-1847, 1905-1910 — read
- §12.3 regenerate: it never reads on_notes_failure. An attended run gets the same choice (`c` = VersionNoAIBody) and the floor body goes to regenerate's review gate without `r`; an unattended run stops with regenerate's next moves — internal/engine/regenerate_interactive.go:234-251; internal/engine/notesfloor.go:58-61, 196-205; internal/engine/regenerate_write.go:239-240 — read
- §12.3 --all: skip-and-continue. A cancel ends the batch. Each skip names its CauseText phrase and saved-file pointer in its warning and in the summary, with the failed call's output beneath the warning — internal/engine/regenerate_batch.go:251-257, 319-327, 386-392, 434-437 and the batchSummary function — read
- §12.4 phrases: timeout, missing tool, too large for the model, AI call failed / AI returned empty notes (by whether Output holds text), no room, and part notes too large to merge. Only call-owned failures take " (part k of n)" or " (merging)": NoRoom and MergeTooLarge return bare, and the single-pass first-round no-room has no callError — internal/notes/resolve.go:140-197; internal/notes/size.go:12-19 — read; pinned by internal/notes/causetext_test.go (7 tests, run above)
- §12.4 next moves: the Always moves (the --notes-file route, saved or written; --no-ai; the fallback move when unattended) come first, then the cause moves. Size: lower the cap, the lean pointer, diff_exclude. No room: prompt, else context, else raise the cap. Timeout: raise the timeout. Missing binary: install or set ai_command. OTHER: the lean pointer only for a non-default command. A merge too large takes no cause moves — internal/engine/notesfloor.go:44-126 — read; pinned by the notes_floor_* and regenerate_floor_nextmoves tests run above
- §12.4 retired phrases: "empty/invalid notes" and "diff too large" no longer reach release-notes output, and ErrDiffTooLarge is consumed only by commit — grep: `grep -rn -e 'empty/invalid notes' -e 'diff too large' --include='*.go' .` → only internal/commit/run.go:56 and internal/notes/size.go:7; ErrDiffTooLarge used only in internal/commit
- §9.6 the ✗ line: Message is the CauseText phrase, Output is the GenerationError's verbatim output through the whole wrap chain, and SavedNotes renders "part notes saved to <path>" beneath it in pretty and plain mode — internal/engine/release.go:1442-1452, 2001-2014, 2042-2057; internal/presenter/pretty.go:577-589; internal/presenter/plain.go:203-215 — read
- §9.7 lean-command move: it points at the README's "The shipped default" section without naming claude flags, and that heading exists — internal/engine/notesfloor.go:79; README.md:301 — read
- Concurrency of Progress calls: every wave Progress callback runs under w.mu. run only returns once inFlight is 0 and re-acquires w.mu after Wait, so notesStage's largestCall/halved/parts reads on the main goroutine are ordered after the writes — internal/notes/chunk.go:168-182, 222-231 — read

#### 13. `--notes-file` · 14. One Non-AI Body: Dropping `[release].fallback` · 15. Chunked Notes at the Review Gate and in the Cache · 16. Progress Narration
- The four non-test CLAUDE.md gates are clean on the delivered tree — whole repo — run: `go build ./...` (exit 0), `gofmt -l .` (empty), `go vet ./...` (exit 0), `golangci-lint run` → "0 issues."
- The notes file is read once, before root, config, any gate or stash, and its body is held in memory — internal/engine/release.go:354-358 (Release reads it first); cmd/mint/main.go:150 (regenerate reads it before ResolveRoot); regenerate_interactive.go:270-273 (the engine uses Supplied and never re-reads) — read
- `--notes-file` with `--no-ai` is exit 2; with `--all` or `--source` exit 2; an empty or whitespace file is exit 1 with "`--notes-file P is empty`"; a missing file and `--notes-file ""` are exit 1 with "reading --notes-file P: …"; refusals leave the repo untouched — cmd/mint/flags.go:102-105, cmd/mint/regenerate_flags.go:151-160, internal/engine/release.go:978-991 — measured: the eight refusal invocations against a scratch repo gave exactly those messages and exit codes, with HEAD and the tree unchanged
- The usage lines match §13.1 word for word — cmd/mint/usage.go:41, :58 — measured: `mint release --help` and `mint release regenerate --help` print them
- A supplied body wins over first-release, degenerate and `--no-ai`, and skips the exclusion check, the cache and every AI call — internal/notes/select.go:184-186 (returns before AssembleDiff, the check, the reuse hook and the generator) — read; measured: no AI was configured or invoked in any `--notes-file` run
- The supplied body's review gate is y/n/e with no `r` on both verbs, and `-y` accepts inside the presenter with no engine skip — internal/engine/release.go:609 with gateForKind :1842-1847; internal/engine/regenerate_write.go:239-240 (supplied → reviewGate with NotesReviewGateNoRegen) — read; measured: `-y` regenerate and release runs exit 0 through the gate
- Exclusions are root-anchored and literal; ignored files, files outside the repo and no file give the zero exclusion — internal/git/exclusion.go:14-25; internal/engine/release.go:922-962 — read; measured: a notes file kept under `.git/` released cleanly with nothing committed
- Clean-tree check, autostash, pre_tag `add -A`, bookkeeping and regenerate's changelog commits all carry the keep-out — internal/preflight/preflight.go:102-113; internal/engine/autostash.go:54-61; internal/record/commit.go:33-51, :80-101, :155-178; internal/engine/regenerate_write.go:347-358 — read and measured (see [3-2] and [3-3] above)
- The keep-out reset is a soft reset plus `restore --staged --worktree` with an exclude-only pathspec, and is used by both the forward unwind and regenerate's reset — internal/engine/unwind.go:197-211, :291-311; internal/engine/regenerate_write.go:304 — measured (see [3-4] above)
- The tag annotation is subject + blank line + body under `--cleanup=verbatim`, with one "\n" added only when the body lacks it — internal/release/release.go:106-112, :138-145 — measured (see [3-8] above)
- `--no-ai` and `on_notes_failure = fallback` share one builder: `{lastTag}..HEAD` subjects without answers commits, else the maintenance body; regenerate's list drops the bookkeeping commit under any prefix — internal/notes/resolve.go:54-115 — read; measured: `mint release -y --no-ai` tagged "chore(release): pre-tag artifacts…/fix: tweak feature/feat: add feature" (no bookkeeping subject), and an empty range gave "Maintenance release — no notable source changes"
- `[release].fallback` fails at load with the exact §14.2 message — internal/config/config.go:553-554 — measured: `mint release` over a .mint.toml carrying `fallback = …` printed "invalid .mint.toml: release.fallback has been removed — every non-AI body is the commit-subject list; supply hand-written notes with --notes-file", exit 1
- on_notes_failure's SoT row and README row say `fallback` ships the commit-subject list; no fixed-string wording or `fallback` key row remains — internal/config/metadata.go:134; README.md:256 — read
- `r` on chunked notes makes one merge call over the kept part notes with only its own nudge; otherwise it re-runs from the size check, so the ladder applies and a chunked `r` keeps new merge inputs — internal/engine/chunknotes.go:115-125; internal/notes/remerge.go:51-57, :62-70; internal/notes/generate.go:200-240 (the nudge rides in instructions; kept inputs exclude it) — read; release_remerge_test.go:66-260 pins the call count and prompt
- The `r` never re-runs the exclusion check — internal/engine/release.go:1340-1350 (GenerateWithContext on the generator, not the selector, over the assembler already carrying the Exclude answers) — read
- A failed `r` (call failure or closed stdin at the nudge) saves the part notes and names the command; a cancel saves nothing — internal/engine/release.go:1867-1895 (keepFailed on the AskLine error and the regenerate error); internal/notes/chunknotes.go:68-77 (context.Canceled skips the save); internal/engine/chunknotes.go:68-83 — read
- Every review-gate abort on a release unwinds (decline, failed `r`, failed editor, closed stdin) — internal/engine/release.go:609-614 (any reviewGate error → Unwind) — read; release_reviewgate_unwind_test.go:114-230 pins it
- Regenerate's single and `--all` gates use the same `r` and save under `mint release regenerate <version>`; a failed `r` ends the batch — internal/engine/regenerate_write.go:250-257; internal/engine/regenerate_batch.go:336-341, :463-479 — read
- A chunked body is cached with its merge inputs on dry and real runs; reuse restores them for `r`; entries are keyed on diff+version+instructions, TTL-bound and pruned — internal/engine/release.go:1033-1054 (reuse restores entry.Merge; Write carries chunkNotes.merged; Prune); internal/notescache/cache.go:44 (TTL), :48-56 (Entry.Merge), :68 (Key), :104-119 (Write), :130 (Lookup TTL check), :170 (Prune) — measured with a fake `ai_command`: the `--dry-run` made check + 4 chunk + 1 merge calls and wrote an entry holding 4 part notes; the following `mint release -y` made only the check call, reused the entry and tagged the merged body
- Plain mode prints one line per event, a finished part included — internal/presenter/plain.go:244-264 — measured: "notes: checking for files that don't belong in the notes", "notes: 37 KB release, in 4 parts, 0..4 of 4 done" (five lines), "notes: merging 4 parts", "notes: generated in 4 parts, merged (0.1s)"
- Pretty texts match §16.1: "Writing notes: {size} release, in {n} parts · {k} of {n} done", the two halving lines, "merging {n} parts", and "✓ Notes written ({n} parts, merged) ({elapsed})" — internal/presenter/pretty.go:520-546; internal/presenter/presenter.go:535-540; internal/engine/notesstage.go:293-295 — read; notesprogress_test.go:22-217 pins the exact lines
- The split size is the composed single-pass prompt, also carried through a halved single pass, and the part total grows on halving — internal/notes/generate.go:225-240; internal/notes/chunk.go:33, :46, :231, :259 — read
- Decimal size format: one place below 10, whole from 10, KB below 1 MB, bytes below 1 KB — internal/bytesize/bytesize.go:10-25 — read; measured: "37 KB" in the plain run
- An `r` that becomes chunked opens its own notes line timed from the `r`; a one-call `r` narrates nothing, as the base revision did — internal/engine/notesstage.go:185-235 — read against the base `regenerateBody` (git show 6aa345e:internal/engine/release.go:1511-1535)
- The follow-ups (lower-cap hint, -y suggestions) print after the notes; at an unattended floor the suggestions print with the next moves — internal/engine/release.go:569-574 (NextMoves then showSuggestions), :591-593 (ShowNotes → hintLowerCap → showSuggestions) — read

#### 17. `mint commit`'s AI-Failure Output · 18. Documentation and the Setup Guide
- An attended commit AI failure renders the warn `could not generate a message, opening editor` with the AI's output verbatim beneath it, then opens the editor — internal/commit/run.go:388-393 — read: `aiFailureOutput(err)` feeds `Warning.Output`, then `runEditorFallback`. Pinned by internal/commit/run_aifail_output_test.go:58, which asserts the exact Warning{Label, Message, Output} and one editor launch; it goes red if Output is dropped.
- An unattended commit (-y or non-TTY) AI failure carries the output beneath `no AI message and no interactive editor available` — internal/commit/run.go:474-476 — read: `surfaceOutput(p, "editor", errNoMessageSource, aiOutput)` under `editorUnavailable()` (:268). Pinned by run_aifail_output_test.go:103, which asserts the exact StageFailure for both UnderYes and NonTTYStdin and that no warning is emitted.
- The output shows once, not twice: the attended run carries it on the warn, and the attended no-editor fail-loud (ErrNoEditor) does not repeat it — internal/commit/run.go:478-487 — read.
- A failed `r` regeneration carries its output on the warn and routes to the empty editor. A nudge over max_prompt_bytes takes the oversized note with no output — internal/commit/run.go:676-684 and :411-414 — read. The first is pinned by run_aifail_output_test.go:144.
- --no-ai, an oversized prompt and an all-excluded diff carry no AI output, and their lines are unchanged — internal/commit/run.go:335, :359-362, :374-377 — read. Pinned by run_aifail_output_test.go:177 (attended and unattended): a failing claude is seeded so any leak would show.
- The output is extracted through the %w chain: Generator wraps transport errors with `generating commit message: %w` (internal/commit/generate.go:154-156), and `errors.As` reaches *ai.GenerationError (internal/commit/run.go:801-807) — read. A timeout or missing binary yields "" (no carrier) — read.
- One composition rule: GenerationError.Output puts stdout first, then stderr, drops whitespace-only streams and trims trailing whitespace. The transport packs the retry attempt's raw streams — internal/ai/transport.go:97-104, :244, :256 — read.
- Both presenters render Warning.Output and StageFailure.Output verbatim beneath the line — internal/presenter/plain.go:305-314 (Warn) and :207-209 (StageFailed); internal/presenter/pretty.go:583-584 and :695-698 — read.
- §17.2: commit makes no classify call — the AI failure goes straight to the fallback with no extra claude invocation — read: internal/commit/run.go:388. Pinned by run_aifail_output_test.go:256 (exactly two claude invocations carrying the same prompt).
- §17.2: commit has no chunking, exclusion check or message file — the Generator makes one `transport.Generate` call after `checkPromptSize` (internal/commit/generate.go:150-158) — read.
- Commit's shipped timeout stays 60s — internal/config/config.go:96 (`DefaultCommitTimeout = 60 * time.Second`), resolved at :642-645 — read.
- The setup guide's AI-transport paragraph covers the lean default (spliced from config.DefaultAICommand), the per-call cap, chunking, classification, halving with the lower-the-cap hint, the floor, 180s for release beside 60s for commit (spliced from constants), and the commit-subject list as the one non-AI body. It scopes on_notes_failure to release — internal/setupguide/setupguide.go:161-182 — read, and run: `go run ./cmd/mint setup` (rendered to the scratchpad). The old `default 60` and "fall back to the commit-subject / fallback body" wording are gone.
- The guide's diff_exclude advice names the levels: procedure step 4 sends release-only noise to [release].diff_exclude and lockfiles and build output to the shared list. The etiquette "diff_exclude scope" splits shared, [release] and [commit] — internal/setupguide/setupguide.go:224-226, :313-332 — read.
- The guide's PONR line carries the one exception, both the re-committed answers commit and the ignored untracked .mint.toml write, matching the 2026-10-21 corrigendum — internal/setupguide/setupguide.go:265-271 — read.
- The guide adds no flag. The one flag in the rendered guide, `--force` on the mint init line, was already there at base — read: scanned the rendered guide for ` --x`/` -x`.
- The guide's config reference renders from config.MetadataRows() with no hand-written key metadata. The rendered table has max_prompt_bytes, the [release] and [commit] diff_exclude rows and [release].diff_keep, and no max_diff_lines or [release].fallback rows. timeout reads "180 on release, 60 on commit". The on_notes_failure description names the floor and the commit-subject list — internal/setupguide/setupguide.go:429-447, internal/config/metadata.go:117-154 — read, and run: `go run ./cmd/mint setup`.
- README per-key tables: max_prompt_bytes (:241), [release].diff_exclude (:261), [release].diff_keep (:262), [commit].diff_exclude (:280). The shared timeout is "`180` on release, `60` on commit" (:240), with [release]/[commit] timeout at "shared, else 180/60" (:260, :279). No max_diff_lines or [release].fallback rows remain. Every README line stating the timeout default names both (:240, :295, :299) — README.md — read: `grep -n 'max_diff_lines\|60\b' README.md`.
- The README's unwind promises name the exception: the feature list at README.md:30 and the surgical-unwind bullet at README.md:353 — read.
- The lean default is documented in full, with what it drops and where conventions go (`[release]`/`[commit]` `context`/`prompt`): README.md:301-324. The floor's lean pointer names that heading (internal/engine/notesfloor.go:79) — read. Pinned by internal/config/readme_ai_command_test.go:42, :67.
- ai_command's quoting rules are documented where ai_command is, and the shared row links to them — README.md:239, :326-348 — read.
- --notes-file on release (README.md:100, :107) and regenerate (:153, :161), the exclusion check and its pauses (:115-123), chunking (:127) and the saved chunk-notes file (:131, :163) sit with their commands — README.md — read.
- CLAUDE.md's config seam says max_prompt_bytes is shared-only and diff_exclude has shared, [release] and [commit] levels that add — CLAUDE.md:37 — read. The surgical-unwind invariant carries the answers exception — CLAUDE.md:42 — read. The symbols CLAUDE.md names (NewForStartup, Seed/SeedSequence, ErrNotInteractive, ErrInputClosed, AskLine, MetadataRows, typeErrorMessages, AICommandFor, ReleaseShim, exitCode, aitransport.New, internal/cmdline, internal/bytesize) were each grepped and exist. CheckDiffSize is removed from both the code and CLAUDE.md.
- New keys have typeErrorMessages entries (max_prompt_bytes, the release/commit diff_exclude, release.diff_keep) — internal/config/config.go:473-486 — read.
- The --notes-file usage lines are in the curated texts — cmd/mint/usage.go:41, :58 — read. Pinned by cmd/mint/usage_test.go:171 (flag coverage) and :244/:255 (word-for-word lines).
- Code comments on the contracts this feature changed were rewritten. The whitespace split became POSIX-quote splitting (internal/ai/transport.go:192-195 and :329-332, internal/aitransport/aitransport.go:25-31). The line-count guard became the max_prompt_bytes guard (internal/commit/run.go:51-53 and :347-349, internal/commit/prompt.go:10-12, internal/commit/generate.go:99-101). The single 60s default became per-verb floors (internal/ai/transport.go:117-128). The fixed fallback string is now only the removed-key message at internal/config/config.go:554. Read, plus a grep of production .go for `strings.Fields|60s|max_diff|release.fallback|CheckDiffSize|DefaultTimeout`; the hits left are the editor's argv split and the commit-scoped 60s floor, both still true.
- Gates over the whole tree — measured: `go build ./...` → OK; `gofmt -l .` → no output; `go vet ./...` → OK; `golangci-lint run` → `0 issues.`
- The tree was left as found — measured: `git status --porcelain` before and after, compared with `cmp` → identical.

#### test surface — the change-set's test files, held against everything the specification asks them to guard
- The four non-test gates are clean — repo root — measured: `go build ./...` → exit 0; `gofmt -l .` → no output; `go vet ./...` → exit 0; `golangci-lint run` → `0 issues.`
- `assertNoTag` is vacuous, and the corrected helper passes — internal/engine/notes_floor_test.go:75-79 — measured: the probe and the corrected-argv engine run above (temp copy; `/usr/bin/diff -r` showed it differing from the tree only at :77)
- `assertUnwoundToStart` still guards "no tag" through assertNoMutation's `git tag -a` prefix — internal/engine/release_reviewgate_unwind_test.go:100-112, internal/engine/release_test.go:1306-1323 — read
- Every other negative argv assertion in the changed tests matches a production argv — read, enumerated by a script over the 206 changed test files:
  - `reset --hard` (internal/engine/unwind.go:294)
  - `push origin HEAD` (internal/engine/regenerate_write.go:375)
  - `log --format=%s` (internal/notes/resolve.go:22)
  - `-C root add|commit -m` (internal/record/commit.go:88,108,111,161)
  - unstage `-C root reset -q <sha> -- .mint.toml` (internal/engine/unwind.go:252 vs internal/engine/release_answersfailure_test.go:121-123)
  - the prefixed notes diff (internal/notes/assemble.go:184)
  - `gh release create … --verify-tag` (internal/publish/publish.go:112-115)
- The exclusion-check call detector matches the production prompt — internal/engine/exclusioncheck_helpers_test.go:12-16 vs internal/notes/exclusioncheck.go:347 — read
- The "no saved chunk notes" negatives walk the directory their store actually writes to, with a store wired — internal/engine/notes_savedchunknotes_test.go:38-44,67-79; internal/engine/release_remergefail_test.go:84-86; internal/engine/regenerate_reviewr_test.go:29,44; internal/engine/release_regenchunkfail_test.go:151-153 — read
- No test writes the developer's real user cache dir:
  - the release seam injects stores rooted in `t.TempDir()` (cmd/mint/release_seam_test.go:36-37,49-50)
  - a nil store saves nothing (internal/engine/chunknotes.go:29-33)
  - cmd `regenerate()` keeps its hard-wired `os.UserCacheDir` store (cmd/mint/main.go:193), as plan task 9-1 says, but its cmd tests drive only successes and single-pass failures (cmd/mint/regenerate_floor_test.go:17-114, cmd/mint/regenerate_exclusioncheck_test.go:58-129), so no chunk notes are ever saved there
  - read
- Environment and process-global reach is confined:
  - editor tests resolve the binary from the process env (internal/engine/notes_floor_edit_test.go:56-59)
  - the one `t.Setenv` test is non-parallel (internal/engine/release_reviewgate_unwind_test.go:252-253)
  - `t.Chdir`, the `os.Stdin` swap and the `os.Stdout`/`os.Stderr` swap run only in non-parallel tests (internal/engine/release_notesfile_test.go:327-379; internal/engine/regenerate_notesfile_test.go:455-461; cmd/mint/regenerate_notesfile_test.go:191-195; cmd/mint/usage_test.go:90-150)
  - no changed test adds a real-exec spawn (`git diff` of internal/ai/transport_test.go adds no NewExecRunner or exec)
  - read
- The `runRelease` usage-error test cannot reach real git if the flag check regressed: the notes file is read before any step — cmd/mint/notesfile_refusal_test.go:27-33, internal/engine/release.go:354-355 — read
- FakeRunner is safe for the parallel wave and fails on unseeded commands — internal/runner/fake_runner.go (`record` holds the mutex across append and outcome; `Invocations` returns a clone) — read
- The RecordingPresenter is not mutex-guarded, but every progress event reaches it under the wave's lock — internal/notes/chunk.go:222-232,259 (send takes the lock at :222 and reports at :230-231; halve runs under that lock and reports at :259) — read
- The four-in-flight bound is a real guard: an unbounded wave never sits at exactly 4 in flight, so `waitInFlight(4)` times out, and maxInFlight is checked — internal/notes/chunkwave_test.go:88-101,174-196 — read; measured under load: 10 and 20 concurrent `notes.test -test.run 'Wave|…|InFlight|Deadline'` processes ×15 passed it every run
- The wave-end guards assert both the starts that must not happen and the in-flight ends that must — internal/notes/waveend_test.go:155-198 — read; measured: the same stress rounds, no failures
- The 50ms-window batch skip-cause test is stable under load — internal/engine/regenerate_batch_skipcause_test.go:189-246 — measured: 24 concurrent race-built `engine.test -test.run '…WaveFailsForGoodAtPartThree…|TestGenerator' -test.count=40` → all exit 0
- The piece-cut guards exercise multi-byte cuts at 8 offsets and check byte-for-byte reassembly and the cap per piece — internal/notes/chunkpieces_test.go:42-45,130-192,239-275 — read
- Classify-answer normalisation and the OTHER fall-throughs (sentence, both codes, unknown word, failed classify) are pinned — internal/notes/halve_test.go:80,205-210 — read
- The lower-cap hint's exclusion of the merge self-checks its fixture (the merge is bigger than every part) — internal/engine/notes_lowercap_test.go:153-176 — read
- The lean default is pinned as a literal, not the production constant — internal/engine/lean_command_test.go:17-22; internal/commit/lean_command_test.go:16-21; internal/config/config_test.go:774 — read
- `.mint.toml` still reaches the commit AI diff — internal/commit/generate_test.go:1057-1073 (exact argv plus prompt content) — read
- The unattended suggestion lines append to existing entries per suggestion — internal/engine/release_exclusionsuggestions_test.go:197-235 — read
- Answers written into `.mint.toml` are reloaded through strict `config.Load` — internal/config/appendrelease_test.go:37,316 — read
- The README and setup-guide pins fail on a missing heading or section rather than passing empty — internal/config/readme_ai_command_test.go:13-30; internal/setupguide/setupguide_test.go:359-373 — read
- The regenerate `-y` prompt guard is not vacuous — internal/engine/regenerate_interactive_test.go:223-256 — read

### Plan Completion
- [x] Phases 1–12 acceptance criteria met, except any named below as not measured (7-12's criterion held only after its in-session correction)
- [x] All tasks completed or deliberately discarded (104 of 104 completed; none skipped or cancelled)
- [x] No scope creep

**Criteria not measured**
- [1-6] "The existing tests pass with no test file edited — `internal/engine/notesfailureoutput_internal_test.go`, `internal/engine/notesfailurewiring_internal_test.go`, `internal/commit/run_aifail_output_test.go`, `internal/ai/transport_test.go` among them — and all five gates pass"
- [6-11] "Rule: the existing tests pass with their semantics untouched. ... All five CLAUDE.md gates pass"
- [6-12] "Rule: the existing tests pass with their semantics untouched, and all five CLAUDE.md gates pass"
- [7-7] "Given a Ctrl-C at a pause, the run is cancelled before any notes call, as a Ctrl-C during the notes stage is (§6.6)"
- [7-14] "The five gates pass: `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues)"
- [7-15] "Rule: no Go file changes except `internal/config/readme_exclusion_check_test.go`, and the five gates pass"
- [8-6] "Given either warning, carrying out the steps it names, in the order it names them, leaves the branch at the starting commit with the answers committed on top (§6.6)"
- [9-1] "the existing suite stays green"
- [9-4] "The five gates in CLAUDE.md pass with no test added, removed or changed."
- [10-2] "every gate passes"
- [11-1] "Behaviour is unchanged. Every existing test stays green with its assertions as they stand, apart from the edits the deletion forces, which are listed under Do. All five gates in CLAUDE.md pass"
- [11-2] "all five gates in CLAUDE.md pass"
- [11-3] "With a trial engine-side skip keyed on deps.Yes … TestRegenerateRun_Yes_AlwaysCallsPrompt fails; with the trial reverted it passes"
- [11-3] "The existing suite stays green … and the five CLAUDE.md gates pass"
- [12-1] "All five gates in CLAUDE.md pass"
- [12-2] "Every other existing test stays green, and all five gates in CLAUDE.md pass"

### Code Quality

The code-quality findings were comment accuracy: doc and test comments that the feature's own changes had made false. They covered the Editor contract, the unwind header, CheckNarration's call order, GenerateWithContext's `r` claim, the commit regeneration stage's exits, the commit-graph test header, the batch-skip test's `on_notes_failure` claim, the `touchesConfig` doc, the `generousTimeout` comment, and the deleted `RegenerateRunRequest` doc. There was also one seam problem: regenerate's chunk-notes store was hard-wired inside the function every cmd test calls. All of these are corrected in this session. No issues outstanding.

### Test Quality

- `assertNoTag` matched a tag argv that production no longer issues, so its 12 call sites (the "before any tag" guards for the floor, pause and failed-`r` aborts) could never fail. Corrected: it now matches any `git tag -a`, and the engine suite stays green with it.
- Regenerate's config load and its chunk-notes wiring had no cmd-level test. Corrected with two new tests through the `regenerate()` seam; each fails when its fix is removed.
- The pretty presenter's Suspend/Warn/Resume bracket had no test. Corrected with a two-order test that fails on the old code.
- `TestGenerator_OverCap_WaveLongerThanTimeout_EachCallOwnDeadline_NotesWritten` depends on wall-clock margins and flaked twice in 600 runs under CPU load. Needs planning.
- No over-testing was flagged.

### Blocking Issues

None outstanding. The one blocking issue raised (7-12) is corrected in this session.

## Findings

### Needs planning

- **A15 — Wave deadline test flakes on wall-clock margins** (`internal/notes/chunkwave_test.go`; sources: test-surface-2)
  - What is wrong: `TestGenerator_OverCap_WaveLongerThanTimeout_EachCallOwnDeadline_NotesWritten` (:354, added in 2-6) needs each 200ms call to finish inside a 500ms per-attempt deadline, and each call to see at least 400ms left. Under CPU contention it failed 2 of 300 runs, with two symptoms: a call timing out, and "call 4 was given 353ms".
  - Failure: the `go test -race ./...` gate intermittently goes red on a loaded developer machine or CI runner, with a misleading "ai command timed out", on no defect.
  - How far the fix reaches: the shape is not settled. (a) Scale `callTime`/`timeout` and relax the lower bound to what a shared deadline could never give. This stays probabilistic and slower, and no check can show it no longer flakes. (b) Compare each call's absolute deadline across serial steps, optionally with a runner that ignores ctx. This is deterministic but changes what the test exercises end to end. Choosing between them is a decision about what the acceptance criterion pins.

### Corrected in this session

14 of 14 actions applied, none skipped, none reverted. Apply commit 78bfe3d: 33 files, 257 insertions, 98 deletions. The fix verifier repaired three comments that piecemeal application had left inaccurate: Warn's doc now says "a stage start or completion" clears a pending suspend; AssembleRange's doc points back to AssembleDiff for the flag explanation; and metadata_test.go's overlap note now names the `ai_command` row. Suite final state: green. `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race -count=1 ./...` and `golangci-lint run` (0 issues) all pass. Sabotage checks in a scratch copy confirmed that each new test fails when its fix is removed.

- **A9 — Failed exclusion check kills the notes spinner (pretty)** — **blocking** — corrected (`internal/presenter/pretty.go`, `internal/presenter/notescheck_test.go`, `internal/notes/exclusioncheck.go`; sources: 7-12-b1, 7-12-1, 7-13-1)
  - Change: `PrettyPresenter.Warn` stops only a live spinner and leaves a pending suspend in place, so the engine's Suspend/Warn/Resume bracket restores the notes line. Stage start and completion still clear the suspend through `stopSpinner`. The `CheckNarration` doc now covers the no-fit order, where `CheckFailed` fires with no `Checking`/`Checked`. `TestPrettyPresenter_FailedExclusionCheck_KeepsTheNotesLineRunning` covers both orders and failed on the old code.
  - Failure fixed: at a TTY, any release whose check failed printed the ⚠ line and then showed no notes line or chunk progress until `✓ Notes written`.
- **A14 — Notes diff argv lets colour or an external diff tool break headers** (`internal/notes/assemble.go` and the notes-diff argv pins in `internal/notes/{assemble,changemap,range,generate}_test.go`, `internal/engine/{regenerate_fresh,release_priortag,notes_reporoot,release_exclusioncheck_skip,release_exclusionanswer,release_diffexclude,notes_halve,notes_halveparts,release_builtin_exclude}_test.go`, `cmd/mint/regenerate_all_test.go`; sources: 1-purpose-and-scope-…-8-chunked-generation-1, 7-3-1)
  - Change: the notes diff now runs `git diff --src-prefix=a/ --dst-prefix=b/ --no-color --no-ext-diff {range} -- . {excludes}`, and the AssembleDiff and AssembleRange docs match. 28 argv pins are updated, among them the three negative `invokedWith` pins. Three of the pin files fell outside the action's file list but inside its grep scope. The regenerate_fresh_test.go:161 message now reads "want the built-in excludes only", and the two `…_ExcludesOnlyChangelog` tests are renamed `…_ExcludesOnlyTheBuiltIns`.
  - Failure fixed: with `color.diff=always` or a `diff.external` tool, the exclusion check saw one garbage path, chunking lost its per-file packing, and the saved chunk notes misreported missing files.
- **A1 — Regenerate's chunk-notes store hard-wired; its config load untested** (`cmd/mint/main.go`, `cmd/mint/regenerate_floor_test.go`, `cmd/mint/regenerate_exclusioncheck_test.go`, `cmd/mint/regenerate_notesfile_test.go`, `cmd/mint/regenerate_seam_test.go` (new); sources: 1-3-1, 2-5-1, 4-13-1)
  - Change: `regenerate()` takes a trailing `chunkNotes engine.ChunkNotesSaver`, and `runRegenerate` passes the user-cache store. The 9 test calls moved to the new signature. Two new tests: `TestRegenerate_ChunkedFreshRunFails_SavesTheFinishedPartsNotesInTheGivenStore`, and the two-row `TestRegenerate_UnloadableConfig_StopsBeforeAnyTagReadOrAICall` (malformed `ai_command`, removed `max_diff_lines`).
  - Failure fixed: dropping the store, or moving or unchecking regenerate's `config.Load`, left the suite green.
- **A5 — `assertNoTag` matches a stale argv and can never fail** (`internal/engine/notes_floor_test.go`; sources: 3-8-1, test-surface-1) — rescued
  - Change: it now matches any `git tag -a` by prefix; the engine suite stays green with it.
  - Failure fixed: a regression creating the tag on a floor abort, unattended floor stop, dry run, cancel or failed `r` passed all 12 call sites.
- **A2 — `generousTimeout` comment claims a single ~60s deadline** (`internal/ai/transport_test.go`; source: 1-4-1). Change: the false second sentence is deleted. Failure fixed: the comment stated a single 60s production deadline that the code falsifies.
- **A3 — Defaults table claims to be complete but lacks `max_prompt_bytes`** (`internal/config/metadata_test.go`; source: 2-1-1). Change: the `max_prompt_bytes` row is added and the `strconv` import restored. Failure fixed: the comment's completeness claim was false.
- **A4 — Regeneration-stage comment misses the over-cap Warn exit** (`internal/commit/run.go`; source: 2-2-1). Change: the comment now names the over-cap prompt closing via the oversized Warn. Failure fixed: readers expected a StageFailed that the code and its test rule out.
- **A6 — Editor doc says every error aborts** (`internal/engine/release.go`; source: 5-2-1). Change: the doc names `ErrEditorReturnToGate` returning to the gate or choice. Failure fixed: a new Editor following the contract would turn the floor's warn-and-return path into an abort.
- **A7 — Batch-skip test claims to override `on_notes_failure`** (`internal/engine/regenerate_batch_skip_test.go`; source: 5-12-1). Change: the header, doc, comment and message are reworded; the test is renamed `TestRegenerateAll_OldestVersionFails_BatchContinues`; the task reference is dropped. Failure fixed: a test claimed to prove an override of a setting regenerate never reads.
- **A8 — `GenerateWithContext` doc claims every `r` routes through it** (`internal/notes/generate.go`; source: 6-1-1). Change: the consumer clause is dropped. Failure fixed: the doc contradicted the merge-only `r` on chunked notes.
- **A10 — Commit-graph test header describes the two-commit graph** (`internal/engine/release_commitgraph_test.go`; source: 8-2-1). Change: the header now scopes the file to a first release's hook-artifact/bookkeeping combinations and states the tag rule as the last commit mint made; task and phase references are dropped. Failure fixed: the header stated a graph and tag rule that the code falsifies.
- **A11 — `touchesConfig` doc claims the commit argv names `.mint.toml`** (`cmd/mint/release_seam_test.go`; source: 9-1-1). Change: the doc no longer lists production calls. Failure fixed: it overstated what the guard catches.
- **A12 — Unwind header omits the kept ignored `.mint.toml` write** (`internal/engine/release.go`; source: 9-3-1). Change: the header now says the unwind leaves the uncommitted write to an ignored `.mint.toml` in place. Failure fixed: a maintainer would read the surviving write as a leak and reopen the dead end the §6.6 corrigendum closed.
- **A13 — `RegenerateRunRequest` lost its doc comment** (`internal/engine/regenerate_interactive.go`; source: 11-1-1). Change: a one-sentence doc is restored. Failure fixed: the type showed nothing in `go doc` or hover.

### Out of scope

- **A16 — Batch header describes skip-and-continue as future work** (quick-fix; `internal/engine/regenerate_batch.go`; source: 5-12-2). The file header is byte-identical to base 6aa345e4 and says skip-and-continue is still to come, while the loop already records skips. Failure: the file implementing the behaviour describes it as future work, against CLAUDE.md's no-stale-scope-claims rule.
- **A17 — Docs place `pre_tag` after the notes; the code runs it before** (quick-fix; `internal/setupguide/setupguide.go`, `README.md`, `internal/config/metadata.go`; source: 17-mint-commit-s-ai-failure-output-18-documentation-and-the-setup-guide-1). All five sites predate this work. Failure: an operator mapping a release script onto the hooks puts a step that reads the new CHANGELOG entry or the bumped version_file in `pre_tag`, where it reads the previous release's content. Conditions for the fix: no `|` in the rewritten SoT description, the guide's MarkerPipeline line kept intact, and the README still naming `pre_tag`.

### Discarded

0. Every one of the 23 findings raised was assessed valid; none was discarded.
