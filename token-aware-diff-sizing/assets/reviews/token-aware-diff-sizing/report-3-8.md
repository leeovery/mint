TASK: The tag annotation carries the notes body byte-identical (token-aware-diff-sizing-3-8)

ACCEPTANCE CRITERIA:
- A release whose notes body carries `## ` headings, a line beginning `#123`, a line ending in two spaces and repeated blank lines creates its annotated tag with `--cleanup=verbatim` and pipes `{commit_prefix} Release {tag}`, a blank line, then that body byte-for-byte; the tag body, the changelog section and the provider release body are the same bytes (§13.3)
- No notes route creates the tag without `--cleanup=verbatim`: an AI body, a body edited at the review gate, a `--notes-file` body, the `--no-ai` commit-subject list, and the first-release and empty-diff bodies all go through the same tag argv (§7.1, §13.3)
- Apart from the added flag the tag step is unchanged: an annotated tag named `<tag>` with its message read from stdin (`-F -`), followed by `git push --atomic origin HEAD <tag>`

STATUS: issues_found

SPEC CONTEXT: §13.3 requires the body to reach the tag, changelog and provider release byte-identical, and the tag annotation to be the bookkeeping subject, a blank line, then the body with git cleaning nothing (`--cleanup=verbatim`). §13.3 also says the annotation ends in a newline: one is added after a body that lacks one, so a signed tag's signature starts on its own line, and that terminator belongs to the annotation rather than the notes. §7.1 lists the notes routes (`--notes-file`, first release, empty diff, `--no-ai`, AI path). CLAUDE.md's byte-identical-bodies invariant applies.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/release/release.go:108 (argv `git tag -a --cleanup=verbatim {tag} -F -`), internal/release/release.go:138-144 (composeTagMessage adds the one terminating "\n" when the message lacks it), internal/engine/release.go:760 (the only production TagAndPush call, so every notes route shares the argv), internal/engine/regenerate_tagsource.go:6-13 (reader doc updated)
- Notes: There is one production tag-creation site (release.go:108), and the only production caller of TagAndPush is engine/release.go:760, so AC2 holds by construction. The task's Do said composeTagMessage would stay "unchanged", but it now adds a terminator when the body lacks one. That change is sound and matches the spec: under verbatim, git no longer adds the trailing newline that `strip` used to add, so an unterminated body would run into a `tag.gpgSign` signature. §13.3 now records this rule, and before this change `strip` already ended every annotation in "\n", so it causes no regression. The Do also asked whether the tag-body reader still splits subject from body. Read against git's `%(contents:body)` semantics, it does: the split is at the first "\n\n" after the subject, blank lines after it are skipped, and the signature is dropped. Trailing whitespace inside the body has no effect on the split. ReadTagBody still returns raw stdout and trims only for hasBody. The production argv and the reader are correct.

TESTS:
- Status: Adequate
- Coverage: internal/engine/release_notesfile_test.go:104 runs the AC1 fixture (headings, `#123`, a two-space line ending, repeated blank lines). Through assertSuppliedBodyShipped (:64) it checks that the tag stdin, the changelog section and the provider stdin are byte-identical, and that the verbatim tag is followed immediately by the atomic push (AC3). internal/release/release_test.go:13 pins the exact argv and the message. internal/release/release_test.go:50 table-tests the terminator (no final newline, one newline, repeated newlines, trailing spaces). Through tagAnnotationBody/stdinOf, which fail the test when the exact new argv is missing, every route is pinned: AI, gate-edited (release_test.go:1074), `--no-ai`/fallback commit subjects (notes_fallback_warning_test.go, release_priortag_test.go:578), first-release (release_test.go:450, :1645) and empty-diff (release_priortag_test.go:620). All 12 positive and negative pinned sites the task listed were updated, including the `v1.2.4` not-created assertion at release_setversion_test.go:181. The tests are not over-built.
- Notes: One later negative assertion went vacuous against the new argv (see FINDINGS). A real-git round trip can't be settled by reading (see UNSETTLED).

CODE QUALITY:
- Project conventions: Followed. The mutation goes through git.Mutator with the body as []byte, and the WHY-comments are updated to match the code as built.
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None in the task's own code

BLOCKING ISSUES:
- None

FINDINGS:
- [in-scope] [contained] internal/engine/notes_floor_test.go:77 — `assertNoTag` still counts the pre-3-8 argv `git tag -a {nextTag} -F -`, which production no longer issues, so `countCmd` always returns 0 and the helper can never fail. It was added later (task 5-1, commit 816b9e0) against the stale argv. The 12 call sites across 7 files rely on it (notes_floor_test.go, notes_floor_edit_test.go, notes_floor_dryrun_test.go, notes_floor_nextmoves_test.go, notes_fallback_warning_test.go, release_exclusionsuggestions_test.go, release_reviewgate_unwind_test.go). This is the same vacuity the task closed for release_setversion_test.go:181. Fix: replace the condition with a prefix match on the existing helper, `if at := firstIndexWithPrefix(f, "git tag -a"); at != -1 {` (defined at internal/engine/release_preflighthook_test.go:31, same `engine_test` package, no new imports), so future argv changes can't make it vacuous again. — FAILS: a regression that creates (and pushes) the release tag on a floor abort, an unattended floor stop, a cancel, a failed regenerate or a dry run passes every test that calls assertNoTag, because the guard can't see the tag argv production actually runs.

UNSETTLED:
- "the tag body, the changelog section and the provider release body are the same bytes (§13.3)" — the tests show that mint pipes the fixture bytes under `--cleanup=verbatim`, but FakeRunner never runs git, which is the gap that hid the original bug. To settle it: in a scratch repo, run `git tag -a --cleanup=verbatim vX -F -` with the test's Markdown fixture message (with and without `tag.gpgSign`), then check that `git cat-file tag vX` shows the body byte-for-byte and that `git for-each-ref --format=%(contents:body) refs/tags/vX` returns the body (plus for-each-ref's record newline) with subject and signature split off.
