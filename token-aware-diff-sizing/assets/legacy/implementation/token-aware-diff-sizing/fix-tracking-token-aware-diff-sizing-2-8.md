## Attempt 1

ISSUES:
- `internal/engine/notesstage.go:26-42`, reached from `internal/engine/release.go:485-490`. `notesStage` records `parts` in `PartsDone`, which fires at the split with done=0. `done()` then names the part count whenever a split happened, whatever the outcome. Under `[release] on_notes_failure = "fallback"`, a failed chunk call or a failed merge goes to `Selector.resolveNormalPathFailure` (`internal/notes/select.go`), which returns the commit-subject list as `KindFallback` with a nil error. `Release` then calls `stage.done()`, and the run prints `✓ Notes written (2 parts, merged) (…)` (plain: `notes: generated in 2 parts, merged (…)`) while shipping a body no merge produced. I reproduced this on a scratch copy for both cases: the second chunk call failing, and the merge failing after both chunks succeed. The notes stage succeeded with Sentence "Notes written (2 parts, merged)" and the shipped body was "subject one\nsubject two\n". Under `-y` in CI, that log line claims AI-merged notes for a release that shipped the commit-subject list. Single-pass fallback reads `Generated release notes` as built.
  FIX:
  - Make the completion claim depend on the merge having happened. Add `Merged(parts int)` to `notes.Progress` and to `noProgress` in `internal/notes/generate.go`.
  - In `generateChunked` (`internal/notes/chunk.go:31-35`), call `g.progress.Merged(len(parts))` only after `g.transport.Generate` returns the merge body with a nil error.
  - In `notesStage`, keep `PartsDone`/`Merging` as relays to the presenter. Set `s.parts` only in `Merged`, so `done()` names the part count only after a successful merge and otherwise keeps the as-built `Generated release notes`/`generated`. The presenter gets no new event.
  - Tests in `internal/engine/notes_narration_test.go`: a release with `max_prompt_bytes = 20000` and `[release]\non_notes_failure = "fallback"`, run twice (second chunk call fails; merge call fails). Assert the notes stage's `StageSucceeded` is the as-built `Generated release notes`/`generated` and the shipped body is the commit-subject list.
  - Test in `internal/notes/chunkprogress_test.go`: a failed merge reports `Merging` but no `Merged`.
  - Update the `notesStage` doc to match.
  ALTERNATIVE: pass the selector's kind in `release.go` (`stage.done(kind)`, naming parts only for `notes.KindNormalAI`); the regenerate sites have no fallback, so they need no change. The diff is smaller, but the claim then rests on the selector's label rather than on the merge having happened. Every later site that swaps a body in inside the notes stage (Phase 5's floor `c`/`e`) would also have to remember the rule. I recommend the first approach.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
These comments are pre-existing, but this change makes them false: `setSpinnerText` now rewrites the spinner's text after it is created, so the remembered text is the current text, not the start text. In the pretty.go field comments, each line starts with one tab.
- `internal/presenter/pretty.go:168-172` — the field is now also set by `setSpinnerText`, so it is no longer only the creation text and is not set only where a spinner is created.
  OLD: 	// activeSpinnerText is the dim start text the active spinner was created with —
	// remembered so SuspendSpinner can carry it over to suspendedText and ResumeSpinner
	// can recreate the spinner on the SAME stage line. It is set wherever a spinner is
	// created (StageStarted and ResumeSpinner) and is only meaningful while
	// activeSpinner is non-nil.
  NEW: 	// activeSpinnerText is the dim text the active spinner is showing — remembered so
	// SuspendSpinner can carry it over to suspendedText and ResumeSpinner can recreate
	// the spinner on the SAME stage line. It is only meaningful while activeSpinner is
	// non-nil.
- `internal/presenter/pretty.go:184` — it holds the text shown at suspend time, which can differ from the start text.
  OLD: 	// suspendedText is the dim start text of the spinner SuspendSpinner stopped,
  NEW: 	// suspendedText is the dim text the spinner SuspendSpinner stopped was showing,
- `internal/presenter/pretty.go:427` — SuspendSpinner remembers the current text, not the start text.
  OLD: // clear the suspend flag), REMEMBERS the spinner's start text so ResumeSpinner can
  NEW: // clear the suspend flag), REMEMBERS the spinner's current text so ResumeSpinner can
- `internal/presenter/pretty.go:444` — the same falsified "start text".
  OLD: // SAME stage line via the remembered start text and Start()ing exactly one — so the
  NEW: // SAME stage line via the remembered text and Start()ing exactly one — so the
- `internal/presenter/spinner.go:57` — the seam now includes SetText.
  OLD: // briandowns spinner so the presenter depends only on the Start()/Stop() seam, not
  NEW: // briandowns spinner so the presenter depends only on the StageSpinner seam, not

NOTES:
- The executor left `r` silent on purpose (the returned generator has no progress attached). That gap is correct: Phase 6 task 6-5 owns narrating an `r` that becomes a chunked run.
- `formatSize` (`internal/presenter/size.go`) writes a value that rounds into the next unit in that unit: 999,500 B reads `1.0 MB`, not `1000 KB`; 9,950 B reads `10 KB`. This reads the spec's "KB below 1 MB" against the rounded value. It is consistent and tested. Go's `%.0f`/`%.1f` round exact halves to even (10,500 B → `10 KB`, 11,500 B → `12 KB`, 1,250,000 B → `1.2 MB`); the spec does not say how halves round.
- `NotesMerging` is emitted before the merge prompt is composed. Once task 2-9 refuses a merge prompt over the cap, the line will briefly read `merging {n} parts` for a merge that is never sent, before the ✗ line. That is 2-9's call to make.
- `TestRelease_OverCap_PlainMode_PrintsEachUpdateAsItsOwnLine` (`internal/engine/notes_narration_test.go:130`) matches the split lines by suffix, although the size (23,459 B → `23 KB`) is deterministic. The exact lines are pinned at presenter level in `internal/presenter/notesprogress_test.go`.
- The stream-discipline note at `internal/presenter/presenter.go:70-73` has a broken reflow ("— any other / narration (Warn, ShowPlan, …) would / interleave…"). Its content is correct.
- Nothing arrived beyond the enumerated inputs.
