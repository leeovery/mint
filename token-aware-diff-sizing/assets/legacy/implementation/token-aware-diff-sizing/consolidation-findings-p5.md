# Consolidation Findings: Token Aware Diff Sizing (Phase 5)

## Findings

### F1: The notes-failure cause precedence is written twice, once for the phrase and once for the moves
- **Class**: duplication
- **Failure**: `notes.CauseText` and the phase's `causeMoves` each test the same six sentinels in the same order. The order matters because `ErrTooLargeForModel` and `ErrMergeTooLarge` wrap the call's own `ai.ErrGenerationFailed` (chunk.go:43, :76, :228). Say a cause is added to `CauseText` and not to `causeMoves`, such as the RATE_LIMIT code spec §9.3 expects to be added later. The ✗ line names the new cause and `reachesNotesFloor` sends it to the floor. `causeMoves` then drops it into the `ErrGenerationFailed` arm if it wraps a generation failure, and to no moves otherwise. An unattended agent on its own `ai_command` gets told to switch to the lean command because of a rate limit. Nothing fails: no compile error, no lint (the repo has no golangci config, so `exhaustive` is not on), and no test, because each cause's moves are pinned only for the causes that exist today.
- **Evidence**: internal/notes/resolve.go:125-148 (the `CauseText` switch: NoRoom, MergeTooLarge, TooLargeForModel, Timeout, CommandMissing, GenerationFailed); internal/engine/notesfloor.go:79-99 (the `causeMoves` switch: same six, same order). Consumers that already key on `CauseText`'s known bit: notesfloor.go:32-38 (`reachesNotesFloor`), regenerate_batch.go:384-389 (`notesSkipCause`), release.go:1834-1843 (`failureMessage`).
- **Proposed shape**: define the precedence once in `notes`: a closed `Cause` enum with `func ClassifyFailure(err error) (Cause, bool)` holding the single ordered switch. `CauseText` maps the `Cause` to its phrase and keeps its location suffix and its output-dependent wording for a generation failure. `causeMoves` switches on the `Cause` instead of re-testing sentinels. `reachesNotesFloor` uses the `ok`. Output stays byte-identical. Adding a cause becomes one new constant that both switches visibly have to handle.
- **Bank**: 5-5 reviewer, "one cause classification that both the ✗ phrase and the floor moves key on"; 5-5 reviewer, "derive the floor ✗ phrase and its cause moves from one cause classification".

### F2: The two `--notes-file` commands are hand-built at six sites
- **Class**: duplication
- **Failure**: The commands that put saved chunk notes to use are built by string concatenation everywhere they print. A change to either command's form made at one site leaves the others printing the old form. That could be F3's quoting, a renamed verb, or a changed flag. No test fails, because each test pins its own literal, so the operator or agent gets a command that no longer works at the surfaces that were missed. It is noticed only on the failed re-run.
- **Evidence**:
  - `mint release --notes-file P`: internal/engine/notesfloor.go:46 (via `notesFileMove`, :69), internal/engine/notesstage.go:62, internal/engine/notesstage.go:70.
  - `mint release regenerate V --notes-file P`: internal/engine/notesfloor.go:59 (via `notesFileMove`, :69), internal/engine/notesstage.go:61, internal/engine/regenerate_batch.go:256.
  - The placeholder form `--notes-file <file>`: internal/engine/notesfloor.go:67.
- **Proposed shape**: one engine helper, `notesFileCommand(base, path string) string`, rendering `base + " --notes-file " + path`, where `base` is `"mint release"` or `"mint release regenerate " + version`. `notesFileMove`, `warnFallback`, `warnDryRunFallback` and `savedNotesPointer` call it and stop concatenating. The `<file>` placeholder form stays a separate literal in `notesFileMove`'s no-file branch, so F3's quoting never touches it. Output stays byte-identical.
- **Bank**: 5-13 reviewer, "build the put-saved-chunk-notes-on-a-released-version command in one helper instead of three string literals". Its sub-claim about the `part notes saved to ` prefix (presenter.go:552, notesstage.go:80, regenerate_batch.go:256) is not carried: that is wording drift between surfaces, and no command breaks over it. 5-3 reviewer, "one helper should render every `--notes-file <path>` command mint prints" (its quoting half is F3).

### F3: Every printed `--notes-file` command pastes the saved path unquoted
- **Class**: behaviour
- **Failure**: The chunk-notes path keeps any space or shell metacharacter in the repo path, because `repoScope` turns only separators into dashes (internal/notescache/cache.go:189-191). A repo at `~/Code/My App` therefore prints `mint release --notes-file …/chunk-notes/-Users-lee-Code-My App/v1.2.0.md`. An operator or agent that pastes or runs it gets a split argv. `--notes-file` gets the path only up to the space, and the rest becomes a stray argument. The re-run the next moves promised would work fails, either as a usage error or with `reading --notes-file …: no such file`. The same happens to the fallback warning's two commands and the `--all` skip's regenerate command.
- **Evidence**: the command sites in F2 (notesfloor.go:69; notesstage.go:61, :62, :70; regenerate_batch.go:256), and internal/notescache/cache.go:189-191 (the path scope keeps spaces).
- **Proposed shape**: inside F2's `notesFileCommand`, run the path through a POSIX single-quoting helper. Put it beside `internal/cmdline.Split` as its inverse, `cmdline.Quote`, with a round-trip test against `Split`. It quotes only when the path holds a byte outside `[A-Za-z0-9_@%+=:,./-]`, writing `'` as `'\''`. Every path the current tests pin (t.TempDir paths) then prints unchanged. Display-only paths stay raw: the `part notes saved to P` line and `tidy the saved notes at P`. The README's `<path>` forms need no change.
- **Bank**: 5-13 reviewer, "every printed `--notes-file <path>` command pastes the path unquoted"; 5-3 reviewer (its quoting half).

### F4: `RangeNoAIBody` stays exported after `VersionNoAIBody` superseded its one outside caller
- **Class**: dead-code
- **Failure**: Task 5-9 exported `notes.RangeNoAIBody` for the regenerate floor's `c`. Task 5-10 replaced that call with `VersionNoAIBody`, which drops the version's `Release {tag}` bookkeeping commit. The export now has no production caller outside `notes`. It sits beside its replacement with a near-identical signature (`revRange` against `revRange, tag`). A later version-range caller that picks the export puts the `🌿 Release vX` bookkeeping subject into that version's notes, which is exactly the defect 5-10 fixed. Nothing flags it: the output is a plausible subject list, and `unused` ignores exported identifiers.
- **Evidence**: internal/notes/resolve.go:66-78 (`resolveFallbackBody` is now a one-line wrapper over the exported `RangeNoAIBody`, its only non-test caller); internal/engine/notesfloor.go:185 (the regenerate floor calls `VersionNoAIBody`); internal/notes/noai_test.go:195-240 (the export's three tests; the same argv and maintenance-body behaviour is pinned through `NoAIBody` at noai_test.go:14-72 and :144-193, and the exact git-failure message through `VersionNoAIBody` at noai_test.go:308-318); commits fc0a724d (added it) and 018622a1 (swapped it out).
- **Proposed shape**: fold `RangeNoAIBody` into an unexported `rangeNoAIBody`, or into `resolveFallbackBody` itself, which keeps the `{lastTag}..HEAD` range. Drop the three `TestRangeNoAIBody_*` tests, since the `NoAIBody` and `VersionNoAIBody` tests already pin the same argv, maintenance-body and git-failure behaviour. `VersionNoAIBody` is then the only exported way to list a range other than `{lastTag}..HEAD`.

## Comment Corrections

Lines are shown from `//` onward with the file's own leading indentation (tabs inside struct and func bodies).

- CLAUDE.md:35: seam 4 says business logic always calls `Prompt(gate)` and never branches on `-y`. The notes floor now skips its gate for an unattended run, from `ReleaseDeps.Yes`/`StdinInteractive` (notesfloor.go:18-20, release.go:523, regenerate_interactive.go:243), and must keep doing so. A maintainer who enforces the seam as written would route the floor through `Prompt` under `-y` and print `notes: aborted (-y)` for a choice the run was never offered.
  OLD: 4. **Gates:** business logic ALWAYS calls `Prompt(gate)` — `-y` auto-accept (and its echo) happens INSIDE the presenter, never as an engine-side branch. `ErrNotInteractive` is already rendered by the presenter (wrap it, add no narration); `ErrInputClosed` is NOT rendered (the caller surfaces it). Free-text reads go through `AskLine` — the engine never reads stdin.
  NEW: 4. **Gates:** business logic answers a gate only through `Prompt(gate)` — `-y` auto-accept (and its echo) happens INSIDE the presenter, never as an engine-side branch. Whether an unattended run (`-y` or non-TTY stdin) is offered a gate at all is the engine's decision, from the startup `Yes`/`StdinInteractive` signals threaded into its deps: the notes floor stops an unattended run with next moves instead of asking. `ErrNotInteractive` is already rendered by the presenter (wrap it, add no narration); `ErrInputClosed` is NOT rendered (the caller surfaces it). Free-text reads go through `AskLine` — the engine never reads stdin.

- internal/presenter/presenter.go:173-175: "the engine ALWAYS calls Prompt at a gate point; it never branches around the call on -y" is false since the notes floor skips its gate unattended.
  OLD: 	// auto-accept echo and returns gate.Default with a nil error. The engine
	// therefore ALWAYS calls Prompt at a gate point; it never branches around the
	// call on -y (the echo is a rendered event, not engine-printed text).
  NEW: 	// auto-accept echo and returns gate.Default with a nil error. The engine never
	// answers a gate itself on -y: the echo is a rendered event, not engine-printed
	// text.

- internal/engine/regenerate_interactive.go:22-27: "this orchestration never branches around a Prompt call on -y — it asks every applicable gate unconditionally" is false. `RegenerateRun` now skips the notes floor's gate when `deps.attended()` is false (line 243).
  OLD: // -y handling mirrors the forward path: the engine ALWAYS calls Prompt at every gate
// (source, target, confirm); the -y SKIP happens INSIDE the presenter, which renders
// the accept echo and returns the gate Default. So this orchestration never branches
// around a Prompt call on -y — it asks every applicable gate unconditionally, and a
// -y presenter answers each with its default + echo. (Flags pre-fill the source/target
// answers and skip ONLY those questions; the confirm still fires unless -y.)
  NEW: // -y handling mirrors the forward path: the engine calls Prompt at the source, target
// and confirm gates whatever -y holds; the -y SKIP happens INSIDE the presenter, which
// renders the accept echo and returns the gate Default. (Flags pre-fill the
// source/target answers and skip ONLY those questions; the confirm still fires unless
// -y.)

- internal/engine/regenerate_interactive.go:140-142: the engine never reads `RegenerateRunRequest.Yes`; `-y` reaches `RegenerateRun` as `ReleaseDeps.Yes`. "Every gate is still called" is false since the notes floor, and `RegenerateWriteRequest` has no `Yes` for it to "thread to".
  OLD: 	// Yes is the -y flag. It does NOT change which gates the engine calls — every gate
	// is still called and the presenter skips internally — but it threads to the write
	// request for the confirm/review gate's internal skip.
  NEW:

- internal/notes/resolve.go:35-38: "the run stops before the tag" is false. An attended release under abort takes a body at the notes floor and tags. The comment also carries workflow vocabulary ("a later wiring task").
  OLD: //     original failure with %w so errors.Is(abortErr, <cause>) still holds. Nothing is
//     tagged — the run stops before the tag (the presenter surfaces it in a later
//     wiring task); this resolver just returns the abort error and NO body, and runs
//     NO git (no fallback record is built when aborting).
  NEW: //     original failure with %w so errors.Is(abortErr, <cause>) still holds. It returns
//     NO body and runs NO git (no fallback record is built when aborting).

- internal/engine/release.go:969-971: the caller no longer always aborts on a `SelectBody` error, because an attended real run takes a body at the notes floor. "BEFORE any mutation" also contradicts Stage 4's own comment, since a pre_tag artifact commit may already be in `made`.
  OLD: //     SelectBody runs the precedence over the run's SelectState. A SelectBody error is
//     an on_notes_failure=abort notes failure: it returns the error for the caller to
//     surface and abort on, BEFORE any mutation.
  NEW: //     SelectBody runs the precedence over the run's SelectState.

- internal/notes/generate.go:146-148: regenerate does not read `on_notes_failure`. A single-version run answers the failure at the notes floor. This phase rewrote the twin claim in regenerate_fresh.go:26-29 but missed this one, which also cites a task id.
  OLD: // generator — single-mode fresh routes on_notes_failure at a higher layer and 5-12's
// --all intercepts the surfaced failure for skip-and-continue, so this method must NOT
// swallow it.
  NEW: // generator, never swallowed: routing it is the caller's.

## Spec Defects

### S1: §12.4's failed-merge example contradicts the section's own phrase table and next moves
- **Claim**: §12.4 (specification.md:675): "A failed merge's guidance reads, for example: `notes couldn't be merged; part notes saved to <path> — tidy them and release with --notes-file <path>`."
- **Observed**: The same section's table gives a failed merge the phrase `part notes too large to merge`, or the call's phrase with a `(merging)` suffix, and the landed code renders exactly that (internal/notes/resolve.go:125-148). The ✗ line is followed by `part notes saved to <path>` on its own line (internal/presenter/presenter.go:551-553). The guidance is the next moves: `tidy the saved notes at <path>, filling in any files its marker lists, and release with mint release --notes-file <path>` (internal/engine/notesfloor.go:69). Nothing prints "notes couldn't be merged": a grep of internal/, cmd/ and README.md finds no match.
- **Read**: Spec stale. The example predates the §12.4 phrase table and the next-moves form beside it, and contradicts both. A reader who matches it would build a phrase and a one-line guidance shape the spec rules out elsewhere. It should show the as-built rendering or be dropped.
