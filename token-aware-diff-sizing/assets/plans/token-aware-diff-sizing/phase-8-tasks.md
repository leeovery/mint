# Phase 8: Recording exclusion answers — 8 tasks

## token-aware-diff-sizing-8-1

### Task 8-1: Write exclusion answers into `.mint.toml`'s `[release]` table

**Problem**: Phase 7's pauses let the operator answer Exclude or Keep on each suggestion, but an answer governs only the release in hand. Unrecorded, the same question comes back on every later release and in every clone: a teammate, CI or an agent session elsewhere is asked it again (§6.6). The answer has to land in `.mint.toml`, the operator's own config, without disturbing anything else they wrote there.

**Solution**: Once the last pause is answered, write every answer of the run together into `.mint.toml`'s `[release]` table. An Exclude answer's glob joins `[release].diff_exclude`, and a Keep answer's joins `[release].diff_keep`. As needed:
- the glob is appended to a key that already exists, after its entries
- the key is added to an existing `[release]` table
- the `[release]` table is added to a file that has none
- with no `.mint.toml`, the file is created holding only that table.

A `[release]` table written inline (`release = { … }`) or with dotted keys (`release.version_file = …`) takes the answer in that form, inside the existing table. Everything else in the file is left as written: other keys, comments and formatting. Answers go to `[release]` only, never to the shared top-level `diff_exclude`, so commit messages still see those files.

**Outcome**: After the last pause, `.mint.toml` holds every answer in its `[release]` table, still loads under strict config, and otherwise reads exactly as the operator wrote it.

**Acceptance Criteria**:
- [ ] Given a `.mint.toml` whose `[release]` table sets `diff_exclude = ["specs/"]` and `diff_keep = ["docs/api/"]`, and an attended release where the operator answers Exclude on `docs/design/` and Keep on `test/fixtures/`, after the last pause the file loads with `[release].diff_exclude` holding `specs/` then `docs/design/`, and `[release].diff_keep` holding `docs/api/` then `test/fixtures/` (§6.5, §6.6)
- [ ] Given a `[release]` table that sets only `version_file` and `version_pattern`, and an Exclude answer on `docs/design/`, a `diff_exclude` holding `docs/design/` is added to that table. The file still has one `[release]` table, and no `diff_keep` key is added (§6.6)
- [ ] Given a `.mint.toml` holding a shared multi-line `diff_exclude` and a `[release.hooks]` table but no `[release]` table, and an Exclude answer on `docs/design/`, a `[release]` table is added whose `diff_exclude` holds `docs/design/`. The file loads with the shared `diff_exclude` unchanged and the hooks as before (§6.6)
- [ ] Given no `.mint.toml` and a Keep answer on `test/fixtures/`, the file is created holding only a `[release]` table whose `diff_keep` holds `test/fixtures/` (§6.6)
- [ ] Given a `.mint.toml` with a header comment, comments on their own lines and after values, blank lines, aligned `=` signs, a shared `diff_exclude`, and a multi-line `[release].diff_exclude` with comments among its entries, Exclude answers on `docs/design/` and `tmp/` leave every comment, blank line, key, table and existing entry byte-identical and in its place. Both globs follow the existing entries of `[release].diff_exclude`, and the shared `diff_exclude` is byte-identical (§6.6)
- [ ] Given a `.mint.toml` that writes its release table inline, `release = { tag_prefix = "v" }`, or with a top-level dotted key, `release.version_file = "VERSION"`, and an Exclude answer on `docs/design/`, the answer lands in that table in the form it is written: a `diff_exclude` holding `docs/design/` inside the inline table's braces, or a `release.diff_exclude` dotted key. No `[release]` header is added, the file loads, and every other line is byte-identical (§6.6, Corrigendum 2026-10-19)

**Do**:
- The file is `.mint.toml` at the repo root, the file `config.Load` reads (`internal/config/config.go`) (§6.6).

**Context**:
> §6.6: answers live in `.mint.toml` rather than in state on one machine, so a teammate, CI or an agent session elsewhere is not asked the same question again. They go to `[release]` only, never the shared list, so commit messages still see those files (§5.1). §6.5: on Exclude the glob joins `[release].diff_exclude`; on Keep it joins `[release].diff_keep` and is never raised again (§5.2).
>
> Corrigendum 2026-10-19 on §6.7: an Exclude answer appends to an existing key and never overwrites it. TOML rejects a key defined twice in one table and the config loads strictly, so a second `diff_exclude` line in `[release]` would break the next run's load. The unattended `[release].diff_exclude` line Phase 7 prints (task 7-10) is the key as an Exclude answer would leave it, so the two agree on its entries.
>
> The user's repos (2026-10-19): six of the eleven `~/Code/*/.mint.toml` files carry `[release.hooks]` with no `[release]` header (agntc, kit, mint, rules-engine, switchboard, tick), and several write multi-line `diff_exclude` arrays under explanatory comments. In TOML a `[release]` header may follow its `[release.hooks]` sub-table, and a key written after the last table header belongs to that table.
>
> As built:
> - `config.Load` reads `{root}/.mint.toml`, returns the defaults when the file is absent, and decodes strictly through `github.com/pelletier/go-toml/v2`. Decoding drops comments and layout, so a decode-then-encode round trip does not keep the file as written.
> - `[release].diff_exclude` and `[release].diff_keep` are Phase 7's keys (tasks 7-1, 7-4), and Phase 7's pauses produce the answers (tasks 7-7, 7-8).
>
> Corrigendum 2026-10-19 on §6.6, the table's form: TOML 1.0 forbids adding keys to an inline table from outside it and rejects a `[release]` header for a table dotted keys already defined, so adding a table would write a file the next run's strict load rejects, while rewriting the table into header form breaks "Everything else in the file is left as written". None of the user's eleven files writes `[release]` either way (2026-10-19), but `config.Load` accepts both.
>
> Task 8-2 commits the file; task 8-3 keeps an unfinished, unattended or dry run from writing it; task 8-8 covers a `.mint.toml` edit stashed by `--autostash`.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §5.1 Three levels of `diff_exclude`; §5.2 The keep list; §6.5 At a terminal: one pause per suggestion; §6.6 Recording the answers; Corrigenda (2026-10-19, §6.6, §6.7)

## token-aware-diff-sizing-8-2

### Task 8-2: Commit the answers after the last pause

**Problem**: An answer written only into the work tree reaches nobody else: until it is committed and pushed, a teammate, CI or an agent session in another clone is asked the same question (§6.6). Left uncommitted, it would also leave the operator's tree dirty after a release that commits everything else it changes.

**Solution**: Once the answers are written, stage `.mint.toml` by path and commit it alone through the mutator, with the fixed subject `chore(release): release-notes path answers for {tag}`. The prefix is constant, like the pre-tag artifact commit's, never `commit_prefix`. One commit carries every answer of the run. It sits on the release branch and goes out with the release's atomic push. Once it lands, one line says the answers were written to `.mint.toml`'s `[release]` table and committed, a new presenter event shown before the notes line resumes; plain mode prints it as its own line.

The answers are recorded before the first notes attempt, so they shape the diff every later step sees. Answers that leave no path in the diff are still committed, and the release gets the maintenance-release body. A later release, from this clone or from another after the push, reads the recorded answers from config: a kept glob's paths are never raised, and an excluded glob's paths never reach the notes. The README's description of the pauses says the answers are recorded in `.mint.toml`'s `[release]` table and committed with the release.

**Outcome**: The operator's answers ship with the release as one commit of `.mint.toml`, so no clone is asked the same question again.

**Acceptance Criteria**:
- [ ] Given an attended release of `v1.3.0` whose pre_tag hook changed `dist/bundle.js`, an Exclude answer on `docs/design/` and a Keep answer on `test/fixtures/`, the branch carries the pre-tag artifact commit, then one commit with the subject `chore(release): release-notes path answers for v1.3.0`, then the release bookkeeping commit. The answers commit's only change is `.mint.toml`, carrying both answers, and `git push --atomic origin HEAD v1.3.0` sends all three (§6.6)
- [ ] Given `commit_prefix = "🚀"`, the answers commit's subject is still `chore(release): release-notes path answers for v1.3.0`, while the bookkeeping commit reads `🚀 Release v1.3.0` (§6.6)
- [ ] Given that release in pretty mode, once the answers commit lands, one line says the answers were written to `.mint.toml`'s `[release]` table and committed, before the notes line resumes. In plain mode it prints as its own line. A run that records no answers shows no such line (§6.6, §16.1, §16.2, Corrigendum 2026-10-19)
- [ ] Given a release whose only changes are under `docs/design/`, and an Exclude answer on `docs/design/`, the answers commit is made and the release's body is the maintenance-release body, with no notes call (§6.6, §7.1)
- [ ] Given a released `v1.3.0` that recorded an Exclude answer on `docs/design/` and a Keep answer on `test/fixtures/`, the next release, whose range changes `docs/design/next.md`, `test/fixtures/case.json` and `src/export.go`, lists neither recorded path in its check call and pauses on neither. Its notes diff carries `test/fixtures/case.json` and no `docs/design/` change. The same holds when that release runs from a fresh clone of the pushed branch (§5.2, §6.5, §6.6)
- [ ] The README's `release` section says the pauses' answers are recorded in `.mint.toml`'s `[release]` table and committed with the release (§6.6, §18.2)

**Do**:
- `.mint.toml` is staged by path and committed through `git.Mutator`, as `record.CommitBookkeeping` stages its paths by name and commits them (`internal/record/commit.go`) (§6.6).
- The subject's prefix is a constant, as `pretagArtifactSubject` builds the pre-tag artifact commit's (`internal/engine/release.go`) (§6.6).
- The recording line is a presenter event in the existing model (`internal/presenter`): the pretty presenter renders it, the plain presenter as its own line, and `presentertest.RecordingPresenter` records it. The engine never touches spinners or TTY state (§16.4).
- The README text goes in its `release` section, beside Phase 7's description of the pauses (§18.2).

**Context**:
> §6.6: the commit sits on the release branch and goes out with the release's push. §7.2 and §15.1: the check's exclusions change the diff every later step sees, and its answers are recorded before the first notes attempt; when they leave the diff empty, the maintenance-release body applies. §5.3: `.mint.toml` is a built-in always-exclude from every release-notes diff and Change Map (task 7-3), so the answers commit's change never reaches this release's notes or a later regenerate's; task 8-7 keeps its subject out of every commit-subject list. §6.1: a `--notes-file` run never runs the check, so the notes file never meets an answers commit (§13.2).
>
> Corrigendum 2026-10-19 on §6.6, the narration: every commit mint makes before the tag is narrated as built, the pre-tag artifact commit inside the pre_tag stage's ✓ line and the bookkeeping commit in the record stage's line (`recordSentence`, `internal/engine/release.go`), so a silent commit of the operator's config would leave them to find it in `git log`. The line's wording is the implementer's. Plain's synthesised narration is byte-pure printable ASCII (`internal/presenter/bytepurity_test.go`). The pauses suspend the spinner (Phase 7, task 7-7), and the `Presenter.StageStarted` stream-discipline note keeps other narration out from between a blocking start and its completion unless wrapped in `SuspendSpinner`/`ResumeSpinner`.
>
> CLAUDE.md's "Surgical unwind, then PONR": every pre-push mutation is tracked and unwound on failure. Task 8-5 makes the answers commit the one exception; tasks 8-4 and 8-6 cover the write, the commit, the reset and the re-commit failing.
>
> As built:
> - `Release` (`internal/engine/release.go`) runs the pre_tag hook and its artifact commit at Stage 3, before the notes stage at Stage 4, and the bookkeeping commit at Stage 5, after the review gate. `git tag -a` tags HEAD, and `git push --atomic origin HEAD {tag}` sends whatever commits HEAD carries.
> - Preflight confirmed a clean tree before any change, and the pre_tag hook's changes are committed before the notes stage, so nothing else is staged when the answers commit is made.
> - The run's pre-push commits are counted in `MadeState` (`internal/engine/unwind.go`) as they land. `MadeState.Commits`'s comment (zero to two commits) and `Release`'s COMMIT-GRAPH INVARIANT comment (up to two commits before the tag) state the commit graph this task changes; §18.3 keeps such comments true in the same change.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §5.2 The keep list; §5.3 Built-in always-excludes; §6.1 When it runs; §6.5 At a terminal: one pause per suggestion; §6.6 Recording the answers; §7.1 Where the body comes from; §7.2 The AI path, in order; §13.2 The file is outside everything mint does; §15.1 `r` on chunked notes; §16.1 Pretty mode: one notes line, updated; §16.2 Plain mode: a line per event; §16.4 How it is built; §18.2 The README; §18.3 CLAUDE.md and code comments; Corrigenda (2026-10-19, §6.6)

## token-aware-diff-sizing-8-3

### Task 8-3: Record nothing unless the last pause is answered

**Problem**: Answers are recorded so that nobody is asked again, so what is recorded must be what the operator decided (§6.6). A run interrupted partway through its pauses holds only some answers. A run with nobody at the terminal has none, and a dry run takes no action (§6.7). Reading closed input as the default would record a `[release].diff_keep` glob, never raised again, that the operator never chose.

**Solution**: Answers are held until the last pause is answered, and only then written and committed:
- A Ctrl-C at any pause, after earlier pauses were answered included, cancels the run and records nothing.
- Stdin closing at a pause is never read as Keep: the run aborts before any tag, unwinds as any failure before the push does, and records nothing.
- An unattended run (`-y` or non-TTY) pauses on nothing and writes nothing.
- A dry run, at a terminal or unattended, asks nothing and writes nothing.

**Outcome**: `.mint.toml` changes, and an answers commit lands, only once an operator at a terminal has answered every pause of a real release.

**Acceptance Criteria**:
- [ ] Given an attended release of `v1.3.0` with three surviving suggestions, the first answered Exclude and the second Keep, a Ctrl-C at the third pause ends the run with no notes call and no tag. `.mint.toml` is byte-identical to before the run, and no commit carries the subject `chore(release): release-notes path answers for v1.3.0` (§6.6)
- [ ] Given the same release with a pre-tag artifact commit, and stdin closing at the third pause, the run aborts with no tag and unwinds to the starting commit, taking the artifact commit with it. `.mint.toml` is unchanged, no answers commit is made, and none of the three globs is in `[release].diff_exclude` or `[release].diff_keep` (§6.6)
- [ ] Given `mint release -y` with surviving suggestions, at a terminal or not, `.mint.toml` is unchanged after the run and no answers commit is made (§6.7)
- [ ] Given `mint release --dry-run` at a terminal without `-y`, or `mint release --dry-run -y`, with surviving suggestions, `.mint.toml` is unchanged and no commit is made (§6.7)
- [ ] Given an attended release where the AI answers "none", or the check fails, `.mint.toml` is unchanged and no answers commit is made (§6.4, §6.6, §6.8)

**Context**:
> §6.6: answers are held until the last pause, then written together; a Ctrl-C during the pauses cancels the run and records nothing. Corrigendum 2026-10-19 on §6.6: stdin closing at a pause aborts the run before any tag, unwinding as any failure before the push does, and records nothing; it is never read as the default, keep. §6.7: unattended and dry runs ask nothing and write nothing.
>
> As built:
> - At a terminal, the pretty presenter's single-keypress read returns `ErrInputClosed` for Ctrl-C and Ctrl-D alike (`readKeyChoice`, `internal/presenter/pretty.go`), and `cmd/mint` cancels the run's context on SIGINT (`signal.NotifyContext`, `cmd/mint/main.go`).
> - A real release on a non-TTY without `-y` fails at the version confirmation, before the notes stage, so the only unattended real run that reaches the check is a `-y` run.
>
> Phase 7 built the abort and unwind at a pause (task 7-7) and the unattended and dry-run output (tasks 7-10, 7-11). Task 8-1 writes the answers and task 8-2 commits them.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.4 Reading the answer; §6.6 Recording the answers; §6.7 Unattended and dry runs; §6.8 When the check fails; Corrigenda (2026-10-19, §6.6)

## token-aware-diff-sizing-8-4

### Task 8-4: A failed answers write or commit unwinds and keeps nothing

**Problem**: Writing `.mint.toml` and committing it are mutations before the push, and either can fail: the file may not be writable, or git may refuse to stage or commit it (§6.6). The exception that keeps the answers commit through an unwind covers a commit that exists, and a failed write or commit made none. A half-finished one would leave mint's edit to the operator's config behind, and the failing tool's own message is what tells the operator what to fix.

**Solution**: A failure writing `.mint.toml`, or staging or committing it, is a failure before the push. It is surfaced and routed through the unwind as a failed pre-tag artifact commit is, with the write error, or git's own stderr, verbatim beneath its line. The run aborts before any tag and unwinds as built, back to the starting commit, taking the pre-tag artifact commit with it. The unwind leaves `.mint.toml` as it was before the answers were written, so a file the answers would have created is not left behind. No answers commit was made, so none is kept or re-committed, and nothing is recorded.

**Outcome**: A run whose answers cannot be recorded stops before any tag, shows the failing tool's own message, and leaves the repo, `.mint.toml` included, as it found it.

**Acceptance Criteria**:
- [ ] Given an attended release where the operator answers every pause and `.mint.toml` cannot be written, the run fails with the write error verbatim beneath its ✗ line, makes no notes call and no tag, and exits 1 (§6.6, §15.1)
- [ ] Given staging or committing `.mint.toml` fails, the run fails with git's own stderr verbatim beneath its ✗ line, not only the runner's exit-code text, makes no notes call and no tag, and exits 1 (§6.6)
- [ ] Given a pre_tag hook that made the pre-tag artifact commit, and committing the answers fails, the unwind resets to the starting commit, taking the artifact commit with it. No commit carries the answers subject, `.mint.toml` reads as it did before the answers were written with no change to it staged, and the unwind's narration names no answers commit as kept (§6.6)
- [ ] Given a release with no pre_tag hook, where `.mint.toml` was written and staged but committing it fails, `.mint.toml` is afterwards byte-identical to before the run, with no change to it staged or unstaged (§6.6)
- [ ] Given no `.mint.toml` before the run, and committing the answers fails, no `.mint.toml` exists afterwards (§6.6)
- [ ] Given any of these failures, the next release's check lists the answered paths again, since nothing was recorded (§6.6)

**Do**:
- The failure takes the release's pre-push failure route, `surfaceAndUnwind` into `Unwind` (`internal/engine/release.go`, `internal/engine/unwind.go`), as the pre-tag artifact and bookkeeping commits' failures do (§6.6, Corrigenda 2026-10-19).

**Context**:
> Corrigenda 2026-10-19 on §6.6:
> - The answers commit is a pre-push mutation, and every other pre-push commit mint makes is surfaced and routed through the surgical unwind on failure. The keep exception covers an answers commit that exists, and a failed write or commit made none. CLAUDE.md's "Surgical unwind, then PONR" (mint removes only what it created) has the unwind take back mint's own write.
> - What travels verbatim beneath the line is the write error, or git's own stderr. The pre-tag artifact commit is the precedent for the route, surfaced and unwound, not for the output: as built its ✗ line carries the wrapped runner error with nothing beneath it. CLAUDE.md's "Error & exit idioms" puts git stderr verbatim in `StageFailure.Output`.
>
> §15.1: the answers are recorded before the first notes attempt, so a failed write or commit stops the run before any notes call.
>
> As built:
> - `surfaceAndUnwind` renders `failureMessage(cause)` on the ✗ line and `notesFailureOutput(cause)` beneath it, which is empty for any cause that is not an `ai.GenerationError`.
> - For a non-zero exit the runner's error carries only the exit code, and git's stderr stays in the returned `runner.Result` (`translateRun`, `internal/runner/exec_runner.go`). `git.Mutator.Mutate` returns that result, after retrying lock contention (`internal/git/mutator.go`); `record.CommitDirtyTree` and `CommitBookkeeping` wrap the error and drop the result.
> - `Unwind` resets with `git reset --hard {start}` only when the run made commits, so a run with no other commit resets nothing, and `git reset --hard` leaves an untracked file in place.
> - Preflight confirmed a clean tree before the run changed anything (`--autostash` stashes the work tree first), so `.mint.toml` as it was before the answers is its committed state, or absent.
>
> Task 8-5 keeps an answers commit that landed; task 8-8 covers `--autostash`.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.6 Recording the answers; §15.1 `r` on chunked notes; Corrigenda (2026-10-19, §6.6)

## token-aware-diff-sizing-8-5

### Task 8-5: An abort after answering keeps the answers commit

**Problem**: An answer is about the repo, not this release (§6.6). The surgical unwind removes every pre-push mutation mint made, so an abort after the operator answered would take the answers commit with it and bring the same questions back on the next run. A commit left at the top of the branch after an unwind also reads as an unwind that failed, unless the unwind says why it is there. And mint's docs promise an unwind that leaves the repo exactly as it found it, which stops being true.

**Solution**: After the operator has answered, every abort or failure before the push unwinds as built, back to the starting commit, taking the pre-tag artifact commit, a bookkeeping commit and a tag mint made with it. The unwind then re-commits the answers on top of the starting commit. This is the one deliberate exception to the surgical unwind, and nothing else mint made survives. It covers:
- an abort at the review gate: a decline, a failed `r`, a launched editor that fails at `e`, stdin closing at the keypress
- an abort at the floor: `a`, an editor failure, stdin closing at the choice
- a failure before the push, such as a failed bookkeeping commit, or a rejected push whose tag the unwind deletes as built
- a Ctrl-C after the last pause, such as during the notes.

The unwind's narration names the answers commit as kept. The README's unwind promises, `mint setup`'s point-of-no-return line and CLAUDE.md's "Surgical unwind, then PONR" invariant name the answers commit as the one exception, re-committed on the starting commit. Code comments that state the unwind's contract are kept true.

**Outcome**: However a release ends before the push once the operator has answered, the branch is back at its starting commit with only the answers commit on top, and the unwind says it kept it.

**Acceptance Criteria**:
- [ ] Given an attended release of `v1.3.0` whose pre_tag hook made the artifact commit, an Exclude answer on `docs/design/`, and a decline at the review gate, the branch ends at the starting commit with one commit on top, `chore(release): release-notes path answers for v1.3.0`, carrying the answer in `.mint.toml`. The artifact commit is gone, no `v1.3.0` tag exists, the unwind's narration names the answers commit as kept and reports a clean repo, and the run exits 1 (§6.6)
- [ ] Given the same release, a failed `r`, a launched editor that fails at the review gate's `e`, or stdin closing at the review gate's keypress ends in the same state, with the same narration (§6.6, §15.1)
- [ ] Given the same release whose notes fail under `on_notes_failure = abort`, `a` at the floor, a launched editor that fails at the floor's `e`, or stdin closing at the floor's choice ends in the same state, with the same narration (§6.6, §12.2)
- [ ] Given the same release whose bookkeeping commit fails, or whose push is rejected after the tag was created, the run ends in the same state, with the same narration. After the rejected push the bookkeeping commit is gone and the tag deleted (§6.6)
- [ ] Given a Ctrl-C while the notes call runs after the last pause, the run ends in the same state, with the same narration (§6.6, §11.5)
- [ ] Given a release with no pre_tag hook and a decline at the review gate after answering, the branch ends at the starting commit with the answers commit on top, and the unwind's narration names it as kept (§6.6)
- [ ] Given an abort that kept the answers commit, the next `mint release` neither lists nor pauses on the paths those answers cover (§6.6)
- [ ] The README's "One atomic point of no return" feature and its "Surgical unwind before the point of no return" bullet, `mint setup`'s point-of-no-return line, and CLAUDE.md's "Surgical unwind, then PONR" invariant each say that after an abort or failure before the push, the exclusion check's answers commit stays, re-committed on the starting commit (§18.1, §18.2, §18.3)

**Do**:
- The re-commit belongs to the surgical unwind, `Unwind` (`internal/engine/unwind.go`), which every pre-push abort and failure reaches, and its narration rides the unwind's engine-authored `Unwound` summary (`surgicalSummary`) (§6.6).
- The docs are the README's "Why Mint?" feature list and its "Safety Model" surgical-unwind bullet, the point-of-no-return line in `pipelineSection()` (`internal/setupguide/setupguide.go`), and CLAUDE.md's "Load-bearing invariants" (§18.1, §18.2, §18.3).

**Context**:
> §6.6: unwinding the answers commit would bring the same question back next time, so it stays. A failure before the push unwinds as built, back to the starting commit and taking the pre-tag artifact commit with it, and the unwind then re-commits the answers on top of the starting commit. The narration names the answers commit as kept, so the commit at the top of the branch does not read as an unwind that failed.
>
> §12.2 and §15.1 with their corrigenda: every abort at the floor and at the review gate unwinds as any failure before the push does (Phase 5's floor, Phase 6 task 6-7). §9.1 and §11.5: a Ctrl-C cancels the run and never reaches `on_notes_failure`; a Ctrl-C at a pause records nothing (task 8-3).
>
> As built:
> - `Unwind` detaches from cancellation (`context.WithoutCancel`), so its git commands still run after a Ctrl-C. It deletes a tag mint made, resets to the starting commit when the run made commits, and narrates `Unwound` only when it undid something. A rejected push marks the tag created (`release.ErrPushRejected`), so the unwind deletes it.
> - `surgicalSummary` authors the summary, `reset {n} commits` and/or `deleted tag {tag}` with a `; repo clean` tail, in ASCII so the plain presenter stays byte-pure; both presenters render it verbatim.
> - The deferred `--autostash` restore runs after the unwind (task 8-8).
> - The preflight's remote-sync gate passes a branch that is only ahead of its upstream (`CheckRemoteSync`, `internal/preflight/preflight.go`), so a kept answers commit does not stop the next release.
> - The README's feature list says the unwind leaves "the repo exactly as it found it", and its Safety Model bullet that mint removes its own commits, tag and autostash; the setup guide's pipeline section says "Everything before it is unwindable; nothing after it is"; CLAUDE.md's invariant says every pre-push mutation is tracked and unwound on failure.
> - Comments that state the unwind's contract go stale with this change: the `unwind.go` file header and `Unwind`'s doc (the repo returned to its exact clean starting state, nothing mint made surviving), the AUTO-UNWIND paragraph of `release.go`'s header, and the `autostash.go` header (the restore lands on the surgically-unwound clean state). §18.3 keeps comments true in the same change.
>
> Task 8-6 covers the reset or the re-commit failing.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.6 Recording the answers; §9.1 The trigger; §11.5 Ctrl-C mid-wave; §12.2 The choice; §15.1 `r` on chunked notes; §18.1 The setup guide; §18.2 The README; §18.3 CLAUDE.md and code comments; Corrigenda (2026-10-19, §12.2, §15.1)

## token-aware-diff-sizing-8-6

### Task 8-6: A failed reset or re-commit warns with the step that restores the answers

**Problem**: The unwind's reset and its re-commit of the answers are git mutations, and either can fail (§6.6). A failed re-commit leaves the answers off the branch. A failed reset leaves the answers commit on the branch among the commits the reset could not remove, and the manual reset the as-built warning names would take the answers with it. Either way, naming the answers commit as kept would be false, and following the as-built steps alone would bring the same questions back on the next release.

**Solution**:
- When the re-commit fails, one warning names the manual step that restores the answers, as a failed reset or tag delete during the unwind names its manual step, with git's own stderr verbatim beneath it.
- When the reset fails, no re-commit is attempted, since the answers commit is still on the branch. The as-built warning that names the reset as the manual step names after it the step that restores the answers, because that reset takes the answers commit with it.

In both cases the unwind's narration does not name the answers commit as kept, and its summary reports manual cleanup required rather than a clean repo.

**Outcome**: When the unwind cannot keep the answers itself, the operator is told how to restore them, and the narration never claims a commit that is not there.

**Acceptance Criteria**:
- [ ] Given an abort after answering where the reset succeeds and the re-commit fails, mint prints one warning in the as-built `unwind incomplete` form, naming the manual step that restores the answers, with git's own stderr from the failed re-commit verbatim beneath it. The unwind's summary does not name the answers commit as kept and ends `; manual cleanup required`, and the run exits 1 (§6.6)
- [ ] Given an abort after answering where the reset fails, no re-commit is attempted. The reset's warning names the manual `git reset --hard {starting commit}` as built and, after it, the step that restores the answers. The summary does not name the answers commit as kept and ends `; manual cleanup required` (§6.6)
- [ ] Given either warning, carrying out the steps it names, in the order it names them, leaves the branch at the starting commit with the answers committed on top (§6.6)

**Do**:
- The re-commit's warning takes the form `warnUnwindIncomplete` gives a failed reset or tag delete, the restore step joins the reset's own warning in `resetCommitsIfMade`, and the summary's tail comes from `summaryTail` (`internal/engine/unwind.go`) (§6.6, Corrigenda 2026-10-19).

**Context**:
> Corrigenda 2026-10-19 on §6.6:
> - The re-commit failing: as built, a failed reset or tag delete during the unwind warns with the exact manual step (`warnUnwindIncomplete`), and the summary drops `; repo clean` for `; manual cleanup required` (`summaryTail`), so the narration never contradicts the warning. Naming the commit kept when it is not would be the false claim that rule exists to prevent.
> - The reset failing: a re-commit on a branch the reset never moved would duplicate the answers commit or commit nothing. The as-built reset warning gives a manual `git reset --hard {start}` that drops the answers along with mint's other commits, and following it would bring the same questions back on the next release, which §6.6's exception exists to prevent.
> - What travels verbatim beneath the re-commit's warning is git's own stderr. The as-built `unwind incomplete` warning is the precedent for the manual-step warning, not for its output: it carries the wrapped runner error, whose text holds only the exit code, while git's stderr stays in the `runner.Result` the mutator returns (`translateRun`, `internal/runner/exec_runner.go`).
>
> As built:
> - `warnUnwindIncomplete` emits a `Warning` labelled `unwind incomplete`, with the message `automatic recovery failed; manually {action}` and `cause.Error()` as its `Output`.
> - `resetCommitsIfMade` names its action as resetting HEAD back to the starting commit, with the command `git reset --hard {start}`.
> - `Unwind` deletes the tag before it resets, and narrates a clean repo only when every recovery mutation succeeded.
>
> Task 8-5 built the re-commit and the narration that names the answers commit as kept.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.6 Recording the answers; Corrigenda (2026-10-19, §6.6)

## token-aware-diff-sizing-8-7

### Task 8-7: Commit-subject lists leave out the answers commit

**Problem**: The answers commit is mint's own tooling commit, and it lands inside release ranges: `{lastTag}..HEAD` on the run that made it, a later run's range when an abort kept it, and the version's own range when that version is regenerated. Every route that ends without AI notes ships the commit-subject list (§14.1), so without a filter `chore(release): release-notes path answers for v1.3.0` would land in a release's notes (§6.6).

**Solution**: Every commit-subject list leaves out subjects starting `chore(release): release-notes path answers for `, whatever tag follows: `--no-ai`'s list, the body `on_notes_failure = "fallback"` ships, the floor's `c`, and `c` at a later regenerate's floor over that version's range. Every other subject stays listed, the pre-tag artifact commit's included. A list left empty gives the maintenance-release body, as built.

**Outcome**: No commit-subject body lists mint's own answers commit.

**Acceptance Criteria**:
- [ ] Given an attended release of `v1.3.0` over `feat: add export` and `fix: handle empty input` that recorded answers, and whose notes call then fails under `on_notes_failure = "fallback"`, the body that ships lists `feat: add export` and `fix: handle empty input`, and not `chore(release): release-notes path answers for v1.3.0` (§6.6, §14.1)
- [ ] Given the same release under `on_notes_failure = abort`, `c` at the floor gives the same list (§6.6, §12.2)
- [ ] Given an earlier attempt at `v1.3.0` that aborted after answering and kept its answers commit, and a re-run as `mint release -M --no-ai`, which cuts `v2.0.0`, the list gives the range's other subjects without `chore(release): release-notes path answers for v1.3.0` (§6.6, §14.1)
- [ ] Given a range holding two answers commits, one kept by an aborted attempt and one from the run that released, neither is listed (§6.6)
- [ ] Given a released `v1.3.0` whose range holds `feat: add export`, `chore(release): pre-tag artifacts for v1.3.0`, `chore(release): release-notes path answers for v1.3.0` and its bookkeeping commit, `c` at a later `mint release regenerate v1.3.0` floor gives `feat: add export` and the pre-tag artifact subject, without the answers subject (§6.6, §12.3, §14.1)
- [ ] A list with nothing left once the answers commits, and on regenerate the version's bookkeeping commit, are left out gives the maintenance-release body (§14.1)

**Do**:
- The forward list is the as-built `notes.FallbackBody` (`internal/notes/resolve.go`); regenerate's, over the version's own range, is Phase 5's floor `c` list (task 5-10) (§14.1).

**Context**:
> §6.6: the commit-subject list leaves out subjects starting `chore(release): release-notes path answers for `, so the floor's `c` and `fallback` never list mint's own answers commit, and neither does `c` on a later regenerate. §14.1: every route that ends without AI notes gives the commit-subject list; on regenerate it also leaves out the version's bookkeeping commit; the pre-tag artifact commit stays; an empty list gives the maintenance-release body, so the tag body is never empty.
>
> §5.3: the answers commit changes only `.mint.toml`, a built-in always-exclude (task 7-3), so its diff never reaches a notes diff, and its subject is the one place it could surface. §6.1: `--no-ai` runs no check, so its list meets an answers commit only when an earlier aborted attempt kept one (task 8-5). §7.1: a range whose post-exclusion diff is empty gets the maintenance-release body before `--no-ai` is consulted.
>
> As built, `notes.FallbackBody` runs `git log --format=%s {lastTag}..HEAD`, and the shared selector floors an empty or whitespace-only list to `StubBody()` (`internal/notes/resolve.go`, `internal/notes/degenerate.go`). Phase 3 made the commit-subject list the one non-AI body (task 3-6); Phase 5 built the floor's `c` on release (task 5-1) and on regenerate (tasks 5-9, 5-10).

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §5.3 Built-in always-excludes; §6.1 When it runs; §6.6 Recording the answers; §7.1 Where the body comes from; §12.2 The choice; §12.3 Regenerate at the floor; §14.1 The commit-subject list everywhere; Corrigenda (2026-10-19, §14.1)

## token-aware-diff-sizing-8-8

### Task 8-8: `--autostash` keeps the answers in the committed `.mint.toml`

**Problem**: `--autostash` stashes the operator's uncommitted work before the clean-tree check and restores it when the run ends, after any unwind, as built. An uncommitted edit to `.mint.toml` sits in that stash while the answers are written. Merging the answers into the stashed edit would commit work the operator never chose to commit, and a restore that overwrote the answers, or dropped the edit, would lose one or the other (§6.6).

**Solution**: The answers are written to the committed `.mint.toml`, the file as the stash left it, and committed. The restore then applies the stashed edit on top, as built. A stashed edit that conflicts with the answers on restore leaves the stash entry intact, with the as-built restore warning. Both hold after a successful release and after an abort that re-commits the answers.

**Outcome**: Under `--autostash`, the answers commit carries the answers and nothing of the operator's stashed edit, and that edit is restored or kept safe in the stash.

**Acceptance Criteria**:
- [ ] Given `mint release --autostash` with an uncommitted edit to `.mint.toml` that adds `diff_exclude = ["specs/"]` under `[release]`, which the committed file lacks, and an Exclude answer on `docs/design/`, the answers commit's `[release].diff_exclude` holds `docs/design/` alone: the stashed `specs/` entry is not committed (§6.6)
- [ ] Given a stashed `.mint.toml` edit that restores cleanly over the answers, after the run the work tree's `.mint.toml` carries both the edit and the answers, and the stash entry is gone (§6.6)
- [ ] Given a stashed `.mint.toml` edit that conflicts with the answers on restore, after the run the stash entry is still in `git stash list`, mint prints the as-built warning `could not restore stashed changes cleanly — your work is preserved in git stash; resolve manually`, and the answers commit still holds the answers (§6.6)
- [ ] Given `--autostash`, a stashed `.mint.toml` edit and an abort after answering, the answers commit sits on the starting commit holding only the answers, and the stashed edit is restored over it, or, on a conflict, left in the stash with the same warning (§6.6)

**Context**:
> §6.6: under `--autostash` the answers are written to the committed `.mint.toml`; a stashed edit to the same file that conflicts on restore leaves the stash entry intact, as any conflicting restore does.
>
> As built (`internal/engine/autostash.go`, and `Release` in `internal/engine/release.go`):
> - `config.Load` runs at Stage 1, before the stash at Stage 2, so the config the run holds, the one Phase 7's check reads, can carry a stashed edit to `.mint.toml` that the committed file does not.
> - The stash is `git stash push --include-untracked`. The restore, `git stash pop`, is deferred until `Release` returns, so it lands after any unwind. A conflicting pop leaves the stash entry in place, mint never drops it, and `warnPopConflict` prints the warning above; a clean pop drops the entry itself.
> - A dry run neither stashes nor restores.
>
> Task 8-1 writes the answers, task 8-2 commits them, and task 8-5 re-commits them after an abort.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.6 Recording the answers
