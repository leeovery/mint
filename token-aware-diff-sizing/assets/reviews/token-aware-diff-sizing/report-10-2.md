TASK: Document the classify-and-halve ladder, the lower-the-cap hint and the merge-only `r` in the README (token-aware-diff-sizing-10-2)

ACCEPTANCE CRITERIA:
- The README's `release` section has a new paragraph beside its chunking paragraph. It says that a failed notes call (single-pass, chunk or merge) is classified by one more call through the release's `ai_command`. Only a generation failure whose AI output holds text on either stream is classified. A timeout, a missing AI tool, empty output, a classify prompt over `max_prompt_bytes` and a classify call that fails all go straight to the floor (§9.1, §9.2, §9.5)
- The same paragraph says what a SIZE answer does on the single-pass call or a chunk call: re-send that slice as calls aimed at half the failed call's composed size; rounds one after another, at most three per slice; a further SIZE fails the slice. SIZE on the merge call is a failed merge, and any other answer goes to the floor (§9.3, §10.1, §10.2)
- The same paragraph says that when halving wrote the notes, mint warns `set max_prompt_bytes = N`, N the exact byte size of the largest diff-carrying call that succeeded; under `regenerate --all` once, at the batch's end, however it ends. The "lower-the-cap hint" at README.md:121 then names a hint the page defines (§10.3, §16.3)
- The same paragraph says `r` on chunked notes re-runs only the merge over the part notes already written, with the nudge, and a change a part's notes missed is not re-read. README.md:127's "(`r` included)" then says what `r` does there (§15.1)
- The AI Transport routing clause says a release's failed notes call reaches `on_notes_failure` only once classification and halving have not written the notes; commit's `$EDITOR` fallback and the rest of the sentence unchanged (§9.1, §12.1)
- Behaviour is unchanged. Only README.md changes, every README pin in internal/config stays green, and every gate passes (§18, §18.2)

STATUS: complete

SPEC CONTEXT: §9 (failure classification: trigger, the classify call, codes, edges such as empty streams / over-cap classify prompt / failed classify call reading as OTHER), §10 (halving after an under-cap overflow: target half the failed call's composed size, at most three halvings per slice, serial rounds, merge never halved; §10.3 the lower-the-cap hint with exact bytes of the largest succeeded diff-carrying call, once at the end of a `--all` batch however it ends), §15.1 (`r` on chunked notes re-runs only the merge with the nudge; a change a part's notes missed is not re-read), §16.3 (the hint is a follow-up after the notes), §18/§18.2 (README stays true to what is built; chunking documented with `mint release`).

IMPLEMENTATION:
- Status: Implemented
- Location: README.md:129 (new paragraph, placed directly after the chunking paragraph at README.md:127 and before the chunk-notes-file paragraph at README.md:131); README.md:295 (reworded routing clause in `## The AI Transport`). Commit 4c977ca touches README.md only (3 insertions, 1 deletion).
- Notes: Every claim in the new paragraph was checked against the code:
  - Classify trigger and skips: internal/notes/classify.go:53-70 (only an `*ai.GenerationError` with non-empty `Output()`; over-cap classify prompt and a failed classify call read OTHER; cancel propagates). Whitespace-only streams count as empty: internal/ai/transport.go:97-105.
  - Single-pass SIZE re-cuts at `promptBytes/2` and counts as the first halving: internal/notes/chunk.go:41-48, called from internal/notes/generate.go:236-238. Chunk SIZE re-cuts at `len(prompt)/2` while `slot.halvings < maxHalvings`: chunk.go:217-219, `maxHalvings = 3` at chunk.go:112; a further SIZE fails as ErrTooLargeForModel and ends the wave: chunk.go:236-237.
  - New parts join the wave queue, `maxChunkCallsInFlight = 4`: chunk.go:116, 253-261.
  - Merge SIZE gives ErrMergeTooLarge, other answers a `(merging)` callError: chunk.go:82-91.
  - Narration strings quoted in the README match internal/presenter/presenter.go:537 and :539 verbatim.
  - Hint text matches `hintLowerCap` (internal/engine/notesstage.go:272-280), label `max_prompt_bytes`; fires only when halved and the merge wrote the notes (`suggestedCap`, notesstage.go:147-152); release prints it after the notes (internal/engine/release.go:592) and after a halved `r` (release.go:607); `--all` prints it on both exits (internal/engine/regenerate_batch.go:204, :218).
  - `r` on chunked notes dispatches to `Remerge` when merge inputs are held (internal/engine/chunknotes.go:115-124; internal/notes/remerge.go:51-57), including chunked notes reused from the dry-run cache (release.go:1035 restores `chunkNotes.merged`).
  - The `(see [`release`](#release))` anchor resolves to the `### `release`` heading (first heading slugging to `release`; the later `### `[release]`` gets `release-1`), matching the existing link at README.md:163.
  - README.md:109, :111, :121, :127 are unchanged, and the new paragraph now defines the behaviour they reference ("chunking exhausted", "the model rejected even the smallest parts", "the lower-the-cap hint", "(`r` included)").

TESTS:
- Status: Adequate
- Coverage: A README-only task adds no tests, as planned. The existing README pins were read against the edit: the tripwire substring "`timeout` seconds, default `180` for release and `60` for commit" (internal/config/readme_tripwire_test.go:229) survives intact at README.md:295; the AI Transport section pins (internal/config/readme_ai_command_test.go:71-79) hold text elsewhere in that section; the release-section pins in internal/config/notes_floor_docs_test.go and internal/config/readme_exclusion_check_test.go are positive `strings.Contains` checks on unedited paragraphs; the negative pins (stale single-timeout phrases at readme_tripwire_test.go:235, `| `fallback`` row prefix in fallback_removed_test.go, `max_diff_lines` row in max_diff_lines_removed_test.go) are not tripped by the new text. No test pinned the removed clause "release follows `on_notes_failure`".
- Notes: None

CODE QUALITY:
- Project conventions: Followed (CLAUDE.md "keep docs true to as-built" holds; no code or comments changed)
- SOLID principles: N/A (documentation only)
- Complexity: Low
- Modern idioms: N/A
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "every gate passes" — run `go build ./...`, `gofmt -l .`, `go vet ./...`, `go test -race ./...` and `golangci-lint run`. Reading found the change to be README.md alone and no README pin in the suite that the edit breaks, so only the test gate reads the changed file.
