TASK: `r` on chunked notes re-runs only the merge (token-aware-diff-sizing-6-1, tick-fc7f49)

ACCEPTANCE CRITERIA:
- Given an attended release whose notes were written as 6 parts and a merge, when the operator chooses `r` at the review gate and enters the nudge `lead with the export feature`, the AI tool receives exactly one call. It carries the release's instructions with the nudge in them, the Change Map, the six parts' notes in chunk order, the merge framing and the closing reminder, and none of the diff. No chunk call and no single-pass call is made (§8.6, §15.1)
- Given that call returns a body, it is shown as the new notes and the review gate `Release with these notes?` asks again. Accepting it ships that body byte-identical as the tag message, changelog entry and provider release (§8.6, §15.1)
- Given a second `r` with the nudge `group the fixes`, its one call carries the same six parts' notes the wave produced and only `group the fixes`: neither the first nudge nor the first re-merge's output (§15.1)
- Given notes written by a single-pass call that was classified SIZE and re-cut into 3 parts, `r` makes exactly one call, a merge over those 3 parts' notes carrying the nudge (§10.1, §15.1)

STATUS: issues_found

SPEC CONTEXT: §15.1 has `r` on chunked notes re-run only the merge call, reusing the per-chunk notes already produced and adding the operator's nudge: one call, as on a single-pass release. §8.6 fixes what the merge carries (release's instructions, Change Map, every chunk's notes in chunk order, merge framing, closing reminder) and has its output pass through byte-identical. §8.7 holds chunk notes in memory for the run. §10.1 makes a single-pass call that fails SIZE a chunked run, so its notes are chunked notes. §2.2/§11.6 put every release-notes call, the merge included, through the release's ai_command and per-call timeout. The 2026-10-19 §15.1 corrigendum has the nudge ride in the release's instructions via appendOneTimeContext.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/release.go:607-609 — the review gate's `r` is built from perRunRegenerator over the run's chunkNotesKeeper
  - internal/engine/release.go:1340-1350 — perRunRegenerator: wired deps.Regenerator overrides; otherwise RegenRun.regenerator over chunkNotes.regenRun(progress), falling back to GenerateWithContext
  - internal/engine/chunknotes.go:97-125 — regenRun reads the keeper's merge inputs live (heldMerged); RegenRun.regenerator makes one Remerge call while merge inputs are held, otherwise the verb's size-check generation
  - internal/engine/release.go:1032-1036 and 1019 — the first generation's sinks hand a written chunked run's merge inputs to the keeper (keepMerged); a reused cache entry sets them from entry.Merge
  - internal/notes/remerge.go:13-16, 51-57 — MergeInputs (instructions without one-time context, Change Map, part notes in chunk order); Remerge composes composeMergePrompt(appendOneTimeContext(instructions, nudge), changeMap, partNotes), refuses an over-cap prompt as ErrMergeTooLarge, and sends it through sendMerge on the same transport
  - internal/notes/remerge.go:62-70 and internal/notes/generate.go:225-240 — chunkedBody keeps releaseInstructions (pre-nudge), so a later `r` carries only its own nudge; the halved path (generateHalved) keeps merge inputs the same way, covering AC4
  - internal/notes/chunk.go:562-575 — composeMergePrompt / mergeFraming shared by the first merge and the re-merge, so the re-merge carries the same Change Map, framing, part headings in order and OutputReminder
  - internal/engine/release.go:1867-1894 — regenerateBody reads the nudge via AskLine, re-shows the result with ShowNotes; reviewGate (1819-1827) loops so the gate asks again
- Notes: Remerge never calls keepMergeInputs, so the keeper's merge inputs stay those of the wave across successive `r`s (AC3). The wired generator is a copy of the one resolveBody built over aiTransport(deps, cfg) (release.go:1017-1018), so the re-merge goes through the release verb's ai_command and timeout. No diff assembly, change-map build or exclusion check runs on a merge-only `r`. The doc comments the task names (Regenerator interface release.go:71-89, ReleaseDeps.Regenerator 150-159, reviewGate, regenerateBody, perRunRegenerator) are true to the as-built routing; README.md:129 documents the behaviour.

TESTS:
- Status: Adequate
- Coverage (internal/engine/release_remerge_test.go):
  - TestRelease_RegenOnChunkedNotes_MakesOneMergeCallCarryingTheNudge (66-106): eight non-check claude calls (six chunks, merge, one re-merge), every argv the release's lean ai_command, the re-merge prompt exactly equal to the first merge's prompt with the nudge spliced into the instructions (so Change Map, framing, parts, reminder all pinned), no `diff --git `, parts 1..6 in order, exactly one diff assembly. Covers AC1.
  - TestRelease_RegenOnChunkedNotes_ReShowsTheReMergeAndShipsItByteIdentical (108-155): ShowNotes sequence merge-then-re-merge, review gate question asked twice, tag annotation / CHANGELOG section / gh release stdin all equal the trailing-space re-merge body. Covers AC2.
  - TestRelease_SecondRegenOnChunkedNotes_ReMergesTheWavesNotesWithOnlyItsOwnNudge (157-194): nine calls; second re-merge equals the first merge with only `group the fixes`; neither the first nudge nor the first re-merge output present; tag carries the second body. Covers AC3.
  - TestRelease_RegenOnHalvedSinglePassNotes_MakesOneMergeCallOverTheThreeParts (234-277): single-pass call fails, classify SIZE, three chunk calls, merge, one re-merge (seven calls); re-merge equals the first merge with the nudge; three parts in order; tag ships the re-merge. Covers AC4.
- Notes: Tests run the production path (newDeps wires no deps.Regenerator; release_test.go:78-90), use FakeRunner/RecordingPresenter, are external-package and parallel. Each would fail if `r` re-ran the wave, re-assembled the diff, dropped or doubled the nudge, or reordered parts. Exact-prompt equality makes the extra Contains checks partly redundant but they state the criteria directly; not over-tested.

CODE QUALITY:
- Project conventions: Followed (AI through the transport seam built once per run, output via presenter, byte-identical body, consumer-defined Regenerator interface, sentinel ErrMergeTooLarge)
- SOLID principles: Good — the `r` rule lives once in RegenRun.regenerator and is shared with regenerate's fresh path; the keeper is the single holder of merge inputs
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: one doc comment in the delivered change-set still states the pre-feature `r` routing (see FINDINGS)

BLOCKING ISSUES:
- None

FINDINGS:
- [in-scope] [contained] internal/notes/generate.go:119-120 — GenerateWithContext's doc still says "It is the AI support for the review gate's `r` (regenerate) choice", unqualified, while this task routed `r` on chunked notes to Remerge instead; the range counterpart's doc (internal/notes/generate.go:156-158) was qualified for the same change. Replace "It is the AI support for the review gate's `r` (regenerate) choice:" with "It is the AI support for the review gate's `r` (regenerate) choice on notes not written by a chunked run (chunked notes re-merge through Remerge instead):" — FAILS: a reader of the forward path's generator is told every `r` re-runs the whole AI path through this method, contradicting the merge-only `r` the code now takes, against CLAUDE.md's keep-comments-true-to-as-built rule and §18.3. Comment-only remedy; non-blocking.

UNSETTLED:
- None
