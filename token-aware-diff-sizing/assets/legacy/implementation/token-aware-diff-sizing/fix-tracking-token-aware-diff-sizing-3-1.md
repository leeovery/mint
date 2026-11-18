## Attempt 1

ISSUES:
- /Users/leeovery/Code/mint/internal/engine/release_notesfile_test.go:221-249 — `TestRelease_NotesFile_AbortAtReviewGateEndsBeforeAnyTag` cannot fail if `n` stops aborting.
  - Line 227 seeds only `seedPriorTagReadGates`, and line 233 checks only that `err` is non-nil.
  - If the gate treated `n` as accept, the run would carry on into Record. `git add CHANGELOG.md` would hit the exhausted FakeRunner ("fakerunner: command \"git\" was not seeded") and surface as a "record" stage failure through `surfaceAndUnwind`.
  - That still returns a non-nil `*AbortError`, no `tag -a` or `commit` invocation appears, and the gate still showed the supplied body, so every assertion passes.
  - The executor's own report says this test "fails at a different point" when the supplied branch is switched off, which confirms its seeding cannot tell an abort from a later crash.
  FIX: Mirror `TestRelease_GateNo_AbortsNonZero` (release_test.go:526):
  - seed the full tail with `seedPriorTagNotesFileGit(f, root)`, so a non-aborting `n` would run through tag, push and `gh release create`, and the no-tag check would then fire
  - replace the `err == nil` check with `assertAbortNonZero(t, err)`
  - add `assertNoMutation(t, f)` and keep the `git commit` check
  - assert `!recorded(rec, presentertest.KindStageFailed)`: an abort at the gate surfaces no StageFailed, a Record failure does.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/notes/select.go:47-50 — the line break after "mint" leaves a broken fragment line; the text needs reflowing.
  OLD: 	// FirstRelease is true when there is no prior tag. It wins over everything but a
	// supplied body: mint
	// records FirstReleaseBody and never assembles a diff, runs the degenerate check,
	// consults --no-ai, or reaches on_notes_failure.
  NEW: 	// FirstRelease is true when there is no prior tag. It wins over everything but a
	// supplied body: mint records FirstReleaseBody and never assembles a diff, runs the
	// degenerate check, consults --no-ai, or reaches on_notes_failure.
- /Users/leeovery/Code/mint/internal/engine/release.go:236-240 — "the body read then is the one that ships" is false on the edit path, where the edited text ships. "exclusion check" names a step that is not built yet.
  OLD: 	// NotesFile is the --notes-file path, empty when not given. A relative path
	// resolves against the process working directory, the directory mint was run
	// from. Release reads it once, before any other step, and the body read then is
	// the one that ships: it wins the notes precedence outright, so no AI, exclusion
	// check, size cap, chunking, cache reuse or on_notes_failure is reached.
  NEW: 	// NotesFile is the --notes-file path, empty when not given. A relative path
	// resolves against the directory mint was run from. Release reads it once, before
	// any other step, so a later change to the file never reaches the run. Its body
	// wins the notes precedence outright: no AI path step runs and on_notes_failure
	// never applies.
- /Users/leeovery/Code/mint/internal/engine/release.go:821-825 — the moved comment restates the comparison the code shows and keeps the argument ("not a meaningful case to support…") rather than only its conclusion.
  OLD: // selectState builds the notes precedence inputs for this run. FirstRelease is
// detected by comparing current to the zero SemVer — a tagless repo resolves to
// {0,0,0}. An actual v0.0.0 tag is therefore treated as a first release (a real
// v0.0.0 release is not a meaningful case to support, and the selector simply records
// "Initial release." for it).
  NEW: // selectState builds the notes precedence inputs for this run. A tagless repo
// resolves current to the zero SemVer, so an actual v0.0.0 tag also reads as a first
// release.

NOTES:
- In the cache row of `TestRelease_NotesFile_WinsOverEveryOtherBodySource`, `reuseConfirmPrompted` can never fire alone. The cache entry is keyed on `priorTagDiff`, but no diff is seeded on the git timeline, so a misplaced supplied branch could never produce a matching key. The criterion is still genuinely covered by `gitDiffInvoked`, since a cache lookup needs an assembled diff, and by the selector unit test's reuse hook, which calls t.Errorf if consulted (select_test.go:318). No change needed.
- The empty post-exclusion diff case is covered structurally. If the supplied branch moved after the degenerate check, the prior-tag row's diff assembly would consume the next scripted `""` and ship StubBody, and `gitDiffInvoked` would fire. The selector unit test fails on any git call.
- A supplied body narrates the notes stage as "generating release notes…" and then "Generated release notes", the same as the as-built first-release and --no-ai paths. The spec does not cover narration for non-AI bodies.
- A read failure surfaces as a "notes" StageFailed before the RunStarted header (exit 1), as a `ResolveRoot` failure already does. The message wraps the `*PathError`, so the path appears twice. An empty path still counts as the flag left off. The refusal set, including the exact `reading --notes-file P: <cause>` text, belongs to task 3-5.
- `FirstReleaseReviewGate`'s name and its pre-existing doc (engine.go:76-100, "first-release notes are fixed-body") now describe a gate every non-AI Kind uses, including `KindSupplied`. This was already the case before this task and the diff does not touch it.
- When the regenerate `--notes-file` source task lands, it should reuse `readNotesFile` (release.go:836), together with whatever refusals 3-5 adds. That way the two verbs cannot drift apart on the read and refusal behaviour.
- In the README example, `# finished notes, no AI` starts at a different column from the other example comments. Cosmetic only.
- Nothing arrived beyond the enumerated inputs.
