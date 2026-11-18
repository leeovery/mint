TASK: Chunked-run progress narration (token-aware-diff-sizing-2-8, tick-9fff2e)

ACCEPTANCE CRITERIA:
- Pretty chunked run: the one notes line reads `Writing notes: {size} release, in {n} parts · {k} of {n} done`, k rising as each part finishes; then `Writing notes: merging {n} parts`; ends `✓ Notes written ({n} parts, merged) ({elapsed})`; no other line per part (§16.1)
- `{size}` is the composed single-pass prompt the size check measured against the cap: 3,100,000 bytes reads `3.1 MB` (§16.1, §7.2)
- Decimal units (1 MB = 1,000,000 B): one decimal below 10, whole number from 10 up, KB below 1 MB, plain bytes below 1 KB — 1,500,000 `1.5 MB`, 38,000,000 `38 MB`, 850,000 `850 KB`, 640 `640 B` (§16.1)
- A file's pieces each count as one part in `{n}` (§8.4, §16.1)
- Plain mode: each update its own line as it happens, one per finished part, then the merge and the completion (§16.2)
- A one-call run narrates as built, with no `Writing notes:` line (§16.1)
- `regenerate <version>` fresh source and each `regenerate --all` version narrate a chunked run the same way (§1.3, §16.1)

STATUS: complete

SPEC CONTEXT: §16.1 keeps a chunked run on the single notes line and rewrites it (split line with size and part count, merge line, a ✓ line naming the part count with elapsed time). The corrigenda fix `{size}` as the composed single-pass prompt the size check measured, fix the decimal size format, and keep the elapsed time on the ✓ line. §16.2 has plain mode print each update as its own line. §16.4 makes the updates new presenter events, rendered by each presenter in its own way, with the engine never touching spinners or TTY state. §8.4 sends a too-big file as pieces, each its own chunk call. §1.3 extends chunking to regenerate's fresh source and to --all.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - Events: internal/presenter/presenter.go:88-99 (interface: NotesProgress / NotesHalving / NotesMerging, with the stream-discipline exception and goroutine contract), :517-524 (NotesProgress payload), :542-545 (NotesMerge)
  - Pretty: internal/presenter/pretty.go:531-533 (split line via bytesize.Format, `·` separator), :542-544 (merge line), :548-554 (setSpinnerText: rewrites the running spinner, no-op with none running); internal/presenter/spinner.go:86-92 (SetText swaps the suffix under the briandowns lock its frame loop holds while drawing)
  - Plain: internal/presenter/plain.go:251-264 (one ASCII line per event: `notes: {size} release, in {n} parts, {k} of {n} done`, `notes: merging {n} parts`)
  - Recorder: internal/presenter/presentertest/recording.go:217-230
  - Size format: internal/bytesize/bytesize.go:10-26 (999_500 is the KB->MB boundary, so a count that rounds to 1000 KB reads `1.0 MB`; 9.95 is the one-decimal/whole-number boundary)
  - Producer seam: internal/notes/generate.go:25-50 (Progress interface; notes does not import presenter), :227-235 (len(prompt), the composed single-pass prompt, passed as promptBytes); internal/notes/chunk.go:33 (split, done 0), :73/:78 (Merging/Merged around the merge call), :227-231 (each success reported under the wave mutex, so k is monotonic and calls never overlap)
  - Engine relay: internal/engine/notesstage.go:39-41, :61-76 (relay to presenter), :283-295 (✓ sentence `Notes written (n parts, merged)` only once Merged fired; otherwise the as-built `Generated release notes`)
  - Sites: internal/engine/release.go:548/581 and :1019 (selector's generator wired to the stage); internal/engine/regenerate_interactive.go:229-231/252; internal/engine/regenerate_batch.go:315-317/329; internal/engine/regenerate_fresh.go:66, :83-85 (NotesSinks.wire); cmd/mint/regenerate_all.go:117 (fresh producer passes sinks; the single-version producer delegates to it at cmd/mint/regenerate_run.go:79-86)
- Notes: Plain wording is ASCII (`,` in place of `·`), as the byte-purity rule requires. The halved single-pass case emits Halved(0,n) in place of the done-0 split, then the split line from the first finished part, which matches the 2026-10-19 corrigendum. Concurrency is sound. Progress calls run under wave.mu. The engine goroutine is parked in cond.Wait while they run, and run() returns only once inFlight reaches 0. So no presenter call races a chunk goroutine's NotesProgress, and the briandowns frame loop reads Suffix under the same lock SetText takes.

TESTS:
- Status: Adequate
- Coverage:
  - Pretty line text sequence, the single spinner and the ✓ line as the only output line: internal/presenter/notesprogress_test.go:28 (no per-part line; `(64.2s)` elapsed asserted)
  - Plain line-per-event, byte purity: internal/presenter/notesprogress_test.go:70
  - Decimal format, including 1.5 MB / 38 MB / 850 KB / 640 B / 3.1 MB and boundaries: internal/bytesize/bytesize_test.go; wired through both presenters at internal/presenter/notesprogress_test.go:94
  - Pretty renders nothing with no notes line running: internal/presenter/notesprogress_test.go:54
  - Producer: split, each finished part, merge, size equal to the composed single-pass prompt: internal/notes/chunkprogress_test.go:55; pieces counted as parts: :81; single-pass reports nothing: :117; failed merge reports Merging but not Merged: :148
  - Engine: release chunked event sequence and ✓ sentence/detail/Blocking: internal/engine/notes_narration_test.go:107; plain release end-to-end: :130; under-cap as built: :175; regenerate fresh: :207; --all per version: :234; failed chunked run then fallback claims no merge: :264
  - cmd wiring of the progress sink for both producers: cmd/mint/regenerate_all_test.go:367 and :382
  - Recorder payloads: internal/presenter/presentertest/recording_test.go:166
- Notes: Each test would fail if its behaviour broke. internal/presenter/notesprogress_test.go:94 replays twelve of the boundary cases internal/bytesize/bytesize_test.go already pins, against both presenters. One or two cases would prove the wiring. This adds maintenance weight, not a defect, so it is not reported as a finding.

CODE QUALITY:
- Project conventions: Followed. Output goes only through the presenter (seam 3). The engine emits payload-driven events and never touches spinners. notes stays decoupled behind a consumer-defined Progress interface. Tests are external packages, use t.Parallel and assert exact rendered lines.
- SOLID principles: Good. Progress is a narrow consumer-side interface, and notesStage adapts it to the presenter.
- Complexity: Low
- Modern idioms: Yes (max builtin, slices, strings.Lines in tests)
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
