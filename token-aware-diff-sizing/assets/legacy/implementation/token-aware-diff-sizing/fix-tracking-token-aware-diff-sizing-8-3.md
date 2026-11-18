## Attempt 1

ISSUES:
- /Users/leeovery/Code/mint/internal/engine/release_answersheld_test.go:111 and :144: `countCmd(f, "git", "tag", "-a", minorTag, "-F", "-")` can never match.
  - Production tags with `git tag -a --cleanup=verbatim {tag} -F -` (/Users/leeovery/Code/mint/internal/release/release.go:108).
  - `countCmd` matches the whole command line exactly (/Users/leeovery/Code/mint/internal/engine/release_surgicalunwind_test.go:341).
  - I confirmed this in a scratch copy: on a successful release that does tag, the test's command line counts 0 and the production command line counts 1.
  - Result: the "no tag" condition of criteria 1 and 2 is not checked. In the Ctrl-C test nothing else catches a tag. The stdin test catches one only indirectly, through the unwind summary.
  FIX: Replace both three-line blocks with `assertNoMutation(t, f)` (/Users/leeovery/Code/mint/internal/engine/release_test.go:1306). It matches any command starting `git tag -a`, so it survives argument changes, and it also rules out a push, an autostash and a provider release, none of which may happen before the tag. I applied this in a scratch copy and both tests pass with it.
  ALTERNATIVE: `if tagAt(f, minorTag) != -1 { t.Errorf(...) }` (/Users/leeovery/Code/mint/internal/engine/release_commitgraph_test.go:32). It matches the exact production command line and names the tag, but goes dead again the next time the arguments change. I recommend the prefix match.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/engine/release_answersheld_test.go:94: the comment is false. At a terminal, the pretty presenter reads keys in raw mode (`term.MakeRaw` turns off signal generation), and `readKeyChoice` reads Ctrl-C as byte 0x03 and returns `ErrInputClosed`, so no SIGINT is raised and the run is not cancelled. Outside raw mode, SIGINT cancels the run, but the line read keeps waiting. The two never happen together.
  OLD: // The terminal reads a Ctrl-C as closed input while the signal cancels the run.
  NEW: // A Ctrl-C reaches the run as a signal cancelling it or, at the raw-mode key read, as closed input; this pause delivers both.
- /Users/leeovery/Code/mint/internal/engine/release_answersheld_test.go:30-31: the clause after the colon only lists the four diff constants the function body joins.
  OLD: // seedThreeSuggestionNotes scripts the notes diff the three suggestions are asked
// about: the export source, the design doc, the fixtures and the README.
  NEW: // seedThreeSuggestionNotes scripts the notes diff the three suggestions are asked about.

NOTES:
- I ran checks in a scratch copy of the repo, since deleted, to confirm the "nothing recorded" assertions test something today:
  - The dry run at a terminal records 2 PreviewGate events, and the `-y` dry run 1 ExclusionSuggestions event.
  - Each of the four failing-check cases records exactly one Warn, and "none" records none.
  - The `-y` test asserts its own precondition (both suggestions printed). The dry-run and none-or-failing tests don't, so they would keep passing if a seeding change stopped them reaching the check. Optional hardening: assert the two PreviewGate events or one ExclusionSuggestions event, and one check-failed warning per failure case.
- At a terminal, a Ctrl-C at a pause takes the closed-input route (raw mode, no signal). Test 2 covers that behaviour after two answers, and test 1 covers the cancel route (SIGTERM, or SIGINT outside raw mode). Both real routes are covered; only the line-94 comment is wrong.
- The fifth failure case, a list too big to send, is not tested on an attended run. Its branch (/Users/leeovery/Code/mint/internal/notes/exclusioncheck.go:130-133) returns before any pause, so nothing can be recorded there. The test the executor cites (`TestRelease_ACheckCallTooBigForTheCap_...`) runs unattended and asserts nothing about `.mint.toml` or an answers commit, so it covers the branch, not the recording.
- In the `-y`, dry-run and none-or-failing tests, `assertNothingRecorded` repeats the byte-identical `.mint.toml` and no-staging checks that `suggestedRun.releaseWith` already makes. A fault would be reported twice; nothing is missed.
- Gates I re-ran: `go build ./...`, `gofmt -l .` (empty), `go vet`, `go test -race -count=1 ./...` (all pass), `golangci-lint run ./internal/engine/...` (0 issues).
