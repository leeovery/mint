TASK: Pause on each usable suggestion (token-aware-diff-sizing-7-7, tick-c80e0d)

ACCEPTANCE CRITERIA:
- Given an attended release where the AI suggests `docs/design/` ("design documents, not shipped behaviour"), then `test/fixtures/` ("test data"), and both match uncovered changed paths, mint pauses on `docs/design/` showing the glob and its reason, then on `test/fixtures/` (§6.5)
- Given a pause, it offers `e` for Exclude and `k` for Keep, and a bare Enter picks Keep (§6.5)
- Given the AI answers "none", no pause shows (§6.4)
- Given a suggestion `build/` that matches no changed path, and a suggestion that matches only paths a `diff_exclude` level or `[release].diff_keep` already covers, neither gets a pause, and the other suggestions still pause (§5.2, §6.4)
- Given a suggestion `*.md` on a release whose only uncovered Markdown change is `docs/guide/intro.md`, it gets a pause (§6.4)
- Given every pause answered Keep, the notes are written from the same diff as with no check (§6.4, §6.5)
- Given the pauses in pretty mode, no spinner animates while a pause waits for its answer (§16.1)
- Given a Ctrl-C at a pause, the run is cancelled before any notes call, as a Ctrl-C during the notes stage is (§6.6)
- Given a pre_tag hook that left an artifact commit, and stdin closing at a pause, the run aborts with no notes call and no tag, and the unwind resets the branch to the starting commit, taking the artifact commit with it; the closed input is never taken as Keep (§6.6)
- The README's `release` section describes the pauses: one per suggestion, showing the glob and the AI's reason, with `e` to exclude and `k`, or Enter, to keep (§18.2)

STATUS: complete

SPEC CONTEXT: §6.4: the AI answers suggested globs with reasons, or "none"; a suggestion survives only if it matches at least one uncovered changed path, matched as git matches `diff_exclude`; the AI never excludes. §6.5 (with the 2026-10-19 corrigenda): each surviving suggestion gets one pause through the gate seam, in the AI's order; `e` Exclude, `k` Keep, Keep the default on a bare Enter; a suggestion whose uncovered matches earlier answers already decide raises no pause. §6.6: Ctrl-C at a pause cancels and records nothing; stdin closing aborts before any tag, unwinding as any pre-push failure (pre_tag artifact commit included), never read as Keep. §16.1: the spinner suspends for the pauses. §18.2: the README documents the pauses with `release`. §5.1: entries are git pathspec globs.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/exclusioncheck.go:123-159 (Check: "none" short-circuit at :144, usable-suggestion filter at :147, pauses preferred over report at :152-157)
  - internal/notes/exclusioncheck.go:176-197 (pause: AI order, skip when earlier answers decide every uncovered match, cancel checked after each pause at :183, a pause error returns before any answer is recorded at :186-188, record only after the last pause at :191)
  - internal/notes/exclusioncheck.go:249-263 (usableSuggestions over the paths the keep list leaves)
  - internal/notes/pathspec.go:15-43 (Go-side emulation of git pathspec matching: leading-directory match, `*`/`?` crossing `/`, lexical normalisation)
  - internal/notes/select.go:249-259 (checkExclusions: all-Keep returns the original diff unchanged; only Exclude answers re-assemble)
  - internal/engine/release.go:1063-1073 (exclusionCheck: pauses only when asksAtTerminal)
  - internal/engine/release.go:1184-1221 (exclusionPauses.Pause brackets Prompt with SuspendSpinner/ResumeSpinner; exclusionPauseGate declares e/exclude, k/keep, Default k; Question carries glob and reason)
  - internal/engine/release.go:551-575 (a pause's ErrInputClosed or a cancel leaves resolveBody as an error that is not a floor failure, so StageFailed then Unwind, taking the pre_tag artifact commit)
  - internal/presenter/gate.go:32-35 (ChoiceExclude "e", ChoiceKeep "k")
  - README.md:117 (release section: pauses, glob and reason, `e`/`k`/Enter, Ctrl-C and closed-stdin behaviour)
- Notes: The task context said that, as built, mint did no Go-side glob matching. The suggestion filter now matches with a Go emulation of git's no-magic pathspec rules (pathspec.go). This is a sound divergence. §6.4 asks only that matching follow git's rules, and doing it in process saves a git call per suggestion. A glob the emulation over-matches costs nothing, because Exclude answers still go to git as `:(exclude)` pathspecs (select.go:257). Globs git would reject, such as `../x`, absolute paths and magic pathspecs, match no changed path here and are dropped before any pause. The keep list is matched with the same emulation (exclusioncheck.go:125), and diff_exclude-covered paths are absent because git already removed them from the notes diff. The closed-input path cannot be read as Keep: Pause returns the error and pause() returns before decided.record.

TESTS:
- Status: Adequate
- Coverage:
  - internal/engine/release_exclusionpause_test.go:73 two pauses in AI order with exact Question text (glob + reason)
  - :90 choices exactly [e exclude, k keep], Default k; :114 a bare Enter through the real plain presenter's Prompt returns Keep
  - :136 "none" gives no pause
  - :149 build/ (no match), specs/ (taken out by [release].diff_exclude), docs/api/ (diff_keep) get no pause while test/fixtures/ still pauses
  - :182 `*.md` pauses on docs/guide/intro.md
  - :209 all-Keep: one notes call carrying the whole unchanged diff, tag body = AI notes
  - :235 every pause is immediately preceded by SuspendSpinner and followed by ResumeSpinner
  - :273 cancel at a pause: context.Canceled, one pause, no notes call, StageFailed then Unwound, reset to start with the artifact commit, no tag
  - :300 closed stdin with a pre_tag artifact commit: ErrInputClosed returned (an answers-recording error would fail errors.Is), no notes call, unwound to start, no tag
  - :319 unattended runs raise no pause
  - internal/notes/exclusionpause_test.go: unit-level order, none, keep-list skip, pathspec forms, a pause error stops further pauses, cancel overrides the answer, Exclude globs returned in order, all-Keep returns none
  - internal/notes/pathspec_internal_test.go: git-matching table including `*.md` vs nested paths, directory prefixes, normalisation
  - internal/config/readme_exclusion_check_test.go:23 README release-section text for the pauses
- Notes: The notes-level unit tests overlap the engine tests on order, none and no-match. Each layer pins its own seam, which fits the project's test idioms, so this is not over-testing. Each test would fail if its behaviour broke: wrong order or Question text, a missing default, a pause on a covered glob, an un-suspended spinner, or closed input taken as Keep (the error would no longer be ErrInputClosed).

CODE QUALITY:
- Project conventions: Followed. Gates go through Prompt(gate) only, the engine never reads stdin, spinner suspension follows the cacheReuse pattern, ErrInputClosed is surfaced by the caller, and the tests use FakeRunner/RecordingPresenter in external test packages with t.Parallel().
- SOLID principles: Good. SuggestionPauses is a one-method consumer-side interface in notes and the engine adapts the presenter to it. The check stays presenter-agnostic.
- Complexity: Low
- Modern idioms: Yes (slices.ContainsFunc/DeleteFunc/Clip, strings.Lines, strings.Cut)
- Readability: Good. Doc comments on Check, WithReport, exclusionCheck and exclusionPauses match the code as built.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "Given a Ctrl-C at a pause, the run is cancelled before any notes call, as a Ctrl-C during the notes stage is (§6.6)": reading settles the engine side. A cancelled context at a pause returns context.Canceled, makes no notes call and unwinds (exclusioncheck.go:183, release_exclusionpause_test.go:273). What a real keypress does depends on the presenter's input mode, which the test simulates rather than drives. In pretty mode on a raw-capable terminal, Ctrl-C arrives as byte 0x03 and returns ErrInputClosed (internal/presenter/pretty.go:949-952), which aborts and unwinds through the closed-input path. In plain line-read mode, the SIGINT cancels ctx but the blocked line read (internal/presenter/plain.go:430-434) only returns on the next Enter or EOF. To settle: on a real TTY, press Ctrl-C at a pause in pretty mode and in `--plain`. Check that the run ends promptly, with no notes call, no tag and no answers recorded.
