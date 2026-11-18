TASK: Corrections (token-aware-diff-sizing-11-2) — route a failed release-notes call through the classify-and-halve ladder in the `mint setup` guide's AI-transport paragraph

ACCEPTANCE CRITERIA:
- `mint setup`'s AI-transport paragraph says: when a release-notes call fails with output and does not time out, mint asks the model, in one more call through ai_command, whether that call was too big for it (§9.1, §9.2)
- The same paragraph says a single call or chunk call the model found too big is re-sent in smaller parts, up to three halvings per call, and that when the smaller parts write the notes, mint suggests a lower max_prompt_bytes (§10.1, §10.2, §10.3)
- The paragraph sends only notes still unwritten to [release].on_notes_failure; the rest of that sentence, from "where an attended release is offered" through "Commit drops to the user's $EDITOR.", reads word for word as before (§12, §12.1)
- The paragraph names none of mint's flags: no ` -y`, `--yes`, `--no-ai`, `--notes-file` or `--dry-run` (§18.1)
- `grep -rn 'release follows' --include='*.go' --include='*.md' --exclude-dir=.workflows .` returns nothing (§18.1)
- Only internal/setupguide/setupguide.go changes; every pin in internal/setupguide/setupguide_test.go and cmd/mint/setup_test.go stays green as written, and all five gates in CLAUDE.md pass (§18.1)

STATUS: complete

SPEC CONTEXT: §18.1 requires the setup guide's AI-transport paragraph to cover failure classification (§9) and halving with the lower-the-cap hint (§10), stay flag-free, and be edited wherever this feature made it untrue. §9.1 runs the classifier only on a generation failure (non-zero exit or empty body after the transport's retry), never on a timeout, missing binary or cancel; §9.5 skips it when both streams are empty. §10.1-10.2 halve a single-pass or chunk call on SIZE, at most three times per slice; the merge is never halved. §10.3 has mint suggest `max_prompt_bytes = N` after halving writes the notes. §12 defines the floor as "where the ladder ends without AI notes", which is the only place on_notes_failure applies, and on release only (§12.1, §12.3).

IMPLEMENTATION:
- Status: Implemented
- Location: internal/setupguide/setupguide.go:175-182 (inside `overview()`'s raw string, under `### The AI transport`)
- Notes:
  - Commit cf1f7cb (the 11-2 commit) replaced "On failure, release follows [release].on_notes_failure:" with the task's prescribed text word for word and kept the remainder of the sentence unchanged. It touched only internal/setupguide/setupguide.go (8+/4-).
  - The claims match the code. The classify trigger is a `*ai.GenerationError` with non-empty `Output()`, and a timeout or missing binary never reaches it (internal/notes/classify.go:53-58). The halving bound is `maxHalvings = 3` (internal/notes/chunk.go:112), and SIZE halving applies only while `slot.halvings < maxHalvings` (internal/notes/chunk.go:217). The lower-the-cap hint is at internal/engine/notesstage.go:278. The text also matches README.md:295's routing clause and the ladder paragraph at README.md:129.
  - Later task 11-3 (commit b23fa5d) changed this text in two places. It turned "Notes still unwritten follow" into "A release's notes still unwritten follow" (:179), and it added "Regenerate never reads on_notes_failure and never ships the commit-subject list on its own." before "Commit drops to the user's $EDITOR." (:182). So AC3's word-for-word remainder now has one extra sentence. This is a deliberate later change that fits the spec, which says on_notes_failure is release-only (§12.1, §12.3). Nothing in 11-2's intent is lost, so it is not a finding.
  - The flag-free criterion holds. Lines 156-184 contain none of ` -y`, `--yes`, `--no-ai`, `--notes-file` or `--dry-run`.
  - Running the grep for 'release follows' (excluding .workflows) returns no matches, exit 1.

TESTS:
- Status: Adequate
- Coverage: The task limited the change to internal/setupguide/setupguide.go and added no new test, by design (AC6). Reading confirms every existing pin still matches the new text after whitespace flattening:
  - `TestGuide_OverviewCarriesAITransport` (internal/setupguide/setupguide_test.go:340): stdin/stdout, on_notes_failure, FATAL.
  - `TestGuide_AITransportParagraphNamesLeanDefault` (:375): untouched line 161.
  - `TestGuide_AITransportParagraphCoversThePerCallCapAndChunking` (:389): "default 1500000 bytes" across the :168-169 splice, "chunk calls", "one merge call" and "Chunking has no setting".
  - `TestGuide_AITransportParagraphNamesTheOneNonAIBody` (:824): "the commit-subject list, the one non-AI body", with no "fallback body" and no `--no-ai`.
  - `TestGuide_AITransportParagraphNamesTheFloorChoiceWithoutAFlag` (:859): the floorOffer, floorStop and floorMoves phrases from :843-845 sit across :180-182, and no listed flag appears.
  - `aiTransportParagraph` (:359) ends at the next "\n#", which is procedure()'s "## Setup procedure".
  - `TestRunSetupEmitsGuideAndExitsZero` (cmd/mint/setup_test.go:16) compares stdout against `setupguide.Guide()` itself, so it follows the new text.
- Notes: No pin covers the new classify-and-halve sentences themselves. The task's own AC6 excludes test changes, and leaving them unpinned does not make any shipped claim false, so this is not reported as a finding.

CODE QUALITY:
- Project conventions: Followed. The prose stays true to the as-built code, the guide stays flag-free, the text needs no backtick splice, and the doc comment on `overview()` (:109-121) is still accurate.
- SOLID principles: Good (prose-only change inside an existing emitter)
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "all five gates in CLAUDE.md pass" — settling this needs a run of `go build ./...`, `gofmt -l .`, `go vet ./...`, `go test -race ./...` (including internal/setupguide and cmd/mint) and `golangci-lint run` on the current tree.
