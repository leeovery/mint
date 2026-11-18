TASK: cli-presentation-4-2 — regenerate per-version narration with verb-shaped closing summary (omits url; --all oldest→newest, one block each)

ACCEPTANCE CRITERIA:
- regenerate per-version blocks reuse existing stage/notes/gate events (no new per-block events); presenter renders blocks in engine emit order.
- --all renders one block per version in engine emit order (oldest→newest) — presenter does not reorder.
- The per-block start-of-run line renders engine-supplied Action regenerating, not releasing.
- A freshly-generated block renders the four-choice notes-review gate; a reused-notes block renders the two-choice reuse confirm — driven by the gate the engine passes.
- The closing summary omits the {url} field entirely (no dangling ·/separator) in both modes.
- Under --all the closing summary summarises the set (engine-supplied summary text), incl. single-version --all.
- A failed/aborted regenerate suppresses the success closing summary (reusing the Phase 2 flag).

STATUS: Complete

SPEC CONTEXT: "Cross-Verb Rendering" / "End-of-run line" — regenerate uses same stage/notes/gate vocabulary, narrated per version (--all oldest→newest), no URL, closing summary summarising the set; failure suppresses success line; gate inventory (four-choice fresh, two-choice reuse).

IMPLEMENTATION:
- Status: Implemented
- Location: presenter.go:441-509 (RunVerb discriminator + RunResult.Verb/Summary), :56-63 (RunStarted doc note: block ordering engine-owned, presenter renders linearly); plain.go:428-451 (RunFinished suppression-first + verb switch; VerbRegenerate "done: {project} {Summary}" no URL); pretty.go:881-906 ("{leaf} regenerated {project} {Summary}" no URL tail); plain.go:138-140 / pretty.go:350-353 (RunStarted engine Action verbatim); gate.go:136-172 (NotesReviewGate/ReuseConfirmGate from Phase 3).
- Notes: No new rendering events — per-version block reuses RunStarted/StageSucceeded/ShowNotes/Prompt. No per-version ordering logic; blocks render in emit order. Clean ownership boundary with 4-4 (4-2 owns regenerate arm/content; 4-4 generalised the dispatch table). No drift.

TESTS:
- Status: Adequate
- Coverage (plain_test.go:612-776, pretty_test.go:421-609 parallel suites): single-version block + URL-less close (plain 618/pretty 427); --all multi-block emit order (725/459); start uses "regenerating" (pretty 482, plain 629/94); --all single-version set summary (646/500); close omits URL no dangling sep (665/522); fresh→four-choice gate (749/569); reuse→two-choice no e/r (763/591); failed block suppresses close (680/537); abort path (runfinished_dispatch_test.go:208); payload round-trip + iota order (presenter_test.go:277,297,326).
- Notes: Each spec edge case + Tests bullet mapped. Behaviour-focused. Dangling-separator checks scoped to summary tail (no false middot matches). Per-mode duplication justified.

CODE QUALITY:
- Project conventions: Followed — exhaustive switch on RunVerb enum, engine-supplied data verbatim, byte-purity guard.
- SOLID principles: Good — single responsibility; additive iota-0 discriminator (open/closed).
- Complexity: Low.
- Modern idioms: Yes — typed Choice/RunVerb, builder options, shared writef.
- Readability: Good — doc comments state engine-vs-presenter ownership + no-dangling-separator.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
