# Phase 8: Recording exclusion answers — 8 tasks

## token-aware-diff-sizing-8-1

### Task 8-1: Write exclusion answers into `.mint.toml`'s `[release]` table

**Problem**: Phase 7's pauses let the operator answer Exclude or Keep on each suggestion, but an answer governs only the release in hand. Unrecorded, the same question comes back on every release and on every machine: a teammate, CI or an agent session elsewhere is asked again (§6.6). The answer has to land in `.mint.toml`, the operator's own config, without disturbing anything else they wrote there.

**Solution**: Once the last pause is answered, write every answer together into `.mint.toml`'s `[release]` table. An Exclude answer's glob joins `[release].diff_exclude`, and a Keep answer's joins `[release].diff_keep`. As needed:
- the glob is appended to a key that already exists, after its entries
- the key is added to an existing `[release]` table
- the `[release]` table is added to a file that has none
- with no `.mint.toml`, the file is created holding only that table.

Everything else in the file is left as written: other keys, comments and formatting. Answers go to `[release]` only, never to the shared top-level `diff_exclude`, so commit messages still see those files.

**Outcome**: After the pauses, `.mint.toml` holds every answer in its `[release]` table and otherwise reads exactly as the operator wrote it.

**Acceptance Criteria**:
- [ ] Given a `.mint.toml` whose `[release]` table sets `diff_exclude = ["specs/"]` and `diff_keep = ["docs/api/"]`, and an attended release where the operator answers Exclude on `docs/design/` and Keep on `test/fixtures/`, after the last pause the file loads with `[release].diff_exclude` holding `specs/` then `docs/design/`, and `[release].diff_keep` holding `docs/api/` then `test/fixtures/` (§6.5, §6.6)
- [ ] Given Exclude answers on `docs/design/` and then `tmp/`, `[release].diff_exclude` holds its existing entries followed by `docs/design/` and `tmp/`, in the order the pauses asked (§6.6)
- [ ] Given a `[release]` table that sets only `tag_prefix = "v"`, and an Exclude answer on `docs/design/`, `diff_exclude` is added to that table. The file still has one `[release]` table, and no `diff_keep` key is added (§6.6)
- [ ] Given a `.mint.toml` with only top-level keys, such as shared `diff_exclude = ["*.lock"]`, and an Exclude answer on `docs/design/`, a `[release]` table is added whose `diff_exclude` holds `docs/design/`. The shared `diff_exclude` still holds only `*.lock` (§6.6)
- [ ] Given no `.mint.toml` and a Keep answer on `test/fixtures/`, the file is created holding only a `[release]` table whose `diff_keep` holds `test/fixtures/` (§6.6)
- [ ] Given a `.mint.toml` with comments on their own lines and after values, blank lines, other keys in `[release]`, a `[release.hooks]` table, and comments among the entries of `[release].diff_exclude`, the write changes only what the answers add. Every comment, blank line, other key and table reads byte-identical and in its place, and the existing entries keep their comments (§6.6)

**Do**:
- The file is `.mint.toml` at the repo root, the one `config.Load` reads (`internal/config/config.go`) (§6.6).

**Context**:
> §6.6: answers live in `.mint.toml` rather than in state on one machine, so a teammate, CI or an agent session elsewhere is not asked the same question again. Answers go to `[release]` only, never the shared list, so commit messages still see those files (§5.1). §6.5: on Exclude the glob joins `[release].diff_exclude`; on Keep it joins `[release].diff_keep` and is never raised again (§5.2).
>
> Corrigendum 2026-10-19 on §6.7: an Exclude answer appends to an existing key and never overwrites it. TOML rejects a key defined twice in one table and the config loads strictly, so a second `diff_exclude` line in `[release]` would break the next run's load. The unattended `[release].diff_exclude` line Phase 7 prints (task 7-10) is the key as an Exclude answer would leave it, so the two agree on its entries.
>
> As built:
> - `config.Load` reads `{root}/.mint.toml`, returns the defaults when the file is absent, and decodes strictly through `github.com/pelletier/go-toml/v2` (`internal/config/config.go`). A decode-then-encode round trip through that library does not carry the file's comments or formatting.
> - `[release].diff_exclude` and `[release].diff_keep` are Phase 7's keys (tasks 7-1, 7-4). Phase 7's pauses produce the answers (tasks 7-7, 7-8), each a `presenter.Gate` offering `e` and `k` with Keep its default.
> - `internal/initgen`'s template is a header and an empty body; a file the answers create holds only the `[release]` table (§6.6).
>
> Task 8-2 commits the file; task 8-3 keeps an unfinished, unattended or dry run from writing it; task 8-8 covers a `.mint.toml` edit stashed by `--autostash`.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §5.1 Three levels of `diff_exclude`; §5.2 The keep list; §6.5 At a terminal: one pause per suggestion; §6.6 Recording the answers; Corrigenda (2026-10-19, §6.7)

## token-aware-diff-sizing-8-2

### Task 8-2: Commit the answers after the last pause

**Problem**: An answer written only into the work tree reaches nobody else. Until it is committed and pushed, a teammate, CI or an agent session in another clone is asked the same question (§6.6). Left uncommitted, it would also leave the operator's tree dirty after a release that otherwise commits everything it changes.

**Solution**: Once the answers are written, stage `.mint.toml` by path and commit it alone through the mutator, with the fixed subject `chore(release): release-notes path answers for {tag}`. The prefix is constant, like the pre-tag artifact commit's, never `commit_prefix`. One commit carries every answer of the run. It sits on the release branch and goes out with the release's atomic push.

The answers are recorded before the first notes attempt, so answers that leave no path in the diff are still committed, and the release gets the maintenance-release body. A later release, from this clone or from another after the push, reads the recorded answers from config: a kept glob's paths are never raised, and an excluded glob's paths never reach the notes. The README's description of the pauses says the answers are recorded in `.mint.toml`'s `[release]` table and committed with the release.

**Outcome**: The operator's answers ship with the release as one commit of `.mint.toml`, so no clone is asked the same question again.

**Acceptance Criteria**:
- [ ] Given an attended release of `v1.3.0` whose pre_tag hook changed `dist/bundle.js`, an Exclude answer on `docs/design/` and a Keep answer on `test/fixtures/`, the branch carries the pre-tag artifact commit, then one commit with the subject `chore(release): release-notes path answers for v1.3.0`, then the release bookkeeping commit. The answers commit's only change is `.mint.toml`, carrying both answers, and the atomic push `git push --atomic origin HEAD v1.3.0` sends all three (§6.6)
- [ ] Given `commit_prefix = "🚀"`, the answers commit's subject is still `chore(release): release-notes path answers for v1.3.0`, while the bookkeeping commit reads `🚀 Release v1.3.0` (§6.6)
- [ ] Given a release whose only changes are under `docs/design/`, and an Exclude answer on `docs/design/`, the answers commit is made and the release's body is the maintenance-release body, with no notes call (§6.6, §7.1)
- [ ] Given a released `v1.3.0` that recorded an Exclude answer on `docs/design/` and a Keep answer on `test/fixtures/`, the next release, whose range changes `docs/design/next.md`, `test/fixtures/case.json` and `src/export.go`, lists neither recorded path in its check call, pauses on neither, and carries no `docs/design/` diff in its notes. The same holds for that release run from a fresh clone of the pushed branch (§5.2, §6.5, §6.6)
- [ ] The README's `release` section says the pauses' answers are recorded in `.mint.toml`'s `[release]` table and committed with the release (§6.6, §18.2)

**Do**:
- `.mint.toml` is staged by path and committed through `git.Mutator`, as `record.CommitBookkeeping` stages and commits its paths (`internal/record/commit.go`) (§6.6).
- The subject's prefix is a constant, as `pretagArtifactSubject` builds the pre-tag artifact commit's (`internal/engine/release.go`) (§6.6).
- The commit is a pre-push mutation, tracked for the surgical unwind with the run's other commits (CLAUDE.md "Surgical unwind, then PONR").
- The README text goes in its `release` section, beside Phase 7's description of the pauses (§18.2).

**Context**:
> §6.6: the commit sits on the release branch and goes out with the release's push. §7.2 and §15.1: the check's answers are recorded before the first notes attempt, and the exclusions they record change the diff every later step sees; when they leave it empty, the maintenance-release body applies. §5.3: `.mint.toml` is a built-in always-exclude from every release-notes diff and Change Map (task 7-3), so the answers commit never reaches this release's notes or a later regenerate's; task 8-7 keeps its subject out of every commit-subject list.
>
> §6.1: a `--notes-file` run never runs the check, so it never makes an answers commit, and the notes file never meets one (§13.2).
>
> As built:
> - `Release` (`internal/engine/release.go`) runs the pre_tag hook and its artifact commit before the notes stage, and the bookkeeping commit after the review gate. The tag is created on HEAD, and `git push --atomic origin HEAD {tag}` sends whatever commits HEAD carries.
> - The run's pre-push commits are tracked in `MadeState` (`internal/engine/unwind.go`) as they land. The comments on `MadeState.Commits` (zero to two commits) and `Release`'s COMMIT-GRAPH INVARIANT (up to two commits before the tag) state the commit graph this task changes; §18.3 keeps such comments true in the same change.
> - Preflight confirmed a clean tree before any change, and the pre_tag hook's changes are committed before the notes stage, so nothing else is staged when the answers commit is made.
>
> Task 8-1 writes the file. Task 8-4 covers the write or the commit failing; tasks 8-5 and 8-6 cover an abort after the commit lands.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §5.2 The keep list; §5.3 Built-in always-excludes; §6.1 When it runs; §6.5 At a terminal: one pause per suggestion; §6.6 Recording the answers; §7.1 Where the body comes from; §7.2 The AI path, in order; §13.2 The file is outside everything mint does; §15.1 `r` on chunked notes; §18.2 The README; §18.3 CLAUDE.md and code comments

## token-aware-diff-sizing-8-3

### Task 8-3: Record nothing unless the last pause is answered

**Problem**: Answers are recorded so that nobody is asked again, so what is recorded must be what the operator decided (§6.6). A run interrupted partway through its pauses holds only some answers. A run with nobody at the terminal has none, and a dry run takes no action (§6.7). Reading closed input as the default would record a `[release].diff_keep` glob, never raised again, that the operator never chose.

**Solution**: Answers are held until the last pause is answered, and only then written and committed:
- A Ctrl-C at any pause, after earlier pauses were answered included, cancels the run and records nothing.
- Stdin closing at a pause is never read as Keep: the run aborts before any tag, unwinds as any failure before the push does, and records nothing.
- An unattended run (`-y` or non-TTY) pauses on nothing and writes nothing.
- A dry run, at a terminal or unattended, asks nothing and writes nothing.
- A run that raises no pause has no answers, and records nothing.

**Outcome**: `.mint.toml` changes only once an operator at a terminal has answered every pause of a real release.

**Acceptance Criteria**:
- [ ] Given an attended release of `v1.3.0` with three surviving suggestions, the first answered Exclude and the second Keep, a Ctrl-C at the third pause cancels the run with no notes call and no tag. `.mint.toml` is byte-identical to before the run, and no commit carries the subject `chore(release): release-notes path answers for v1.3.0` (§6.6)
- [ ] Given the same release with a pre-tag artifact commit, and stdin closing at the third pause, the run aborts with no tag and unwinds to the starting commit, taking the artifact commit with it. `.mint.toml` is unchanged, no answers commit is made, and none of the three globs is in `[release].diff_exclude` or `[release].diff_keep` (§6.6)
- [ ] Given `mint release -y` with surviving suggestions, whether stdin is a terminal or not, `.mint.toml` is unchanged after the run and no answers commit is made (§6.7)
- [ ] Given `mint release --dry-run` at a terminal without `-y`, or `mint release --dry-run -y`, with surviving suggestions, `.mint.toml` is unchanged and no commit is made (§6.7)
- [ ] Given an attended release where the AI answers "none", or the check fails, `.mint.toml` is unchanged and no answers commit is made (§6.4, §6.6, §6.8)

**Context**:
> §6.6: answers are held until the last pause, then written together; a Ctrl-C during the pauses cancels the run and records nothing. Corrigendum 2026-10-19 on §6.6: stdin closing at a pause aborts the run before any tag, unwinding as any failure before the push does, and records nothing. It is never read as the default, keep, which would record a `[release].diff_keep` glob the operator never chose. §6.7: unattended and dry runs ask nothing and write nothing.
>
> As built:
> - At a terminal, the pretty presenter's single-keypress gate read returns `ErrInputClosed` for Ctrl-C and Ctrl-D alike (`readKeyChoice`, `internal/presenter/pretty.go`), and `cmd/mint` cancels the run's context on SIGINT (`signal.NotifyContext`, `cmd/mint/main.go`).
> - A real release on a non-TTY without `-y` fails at the version confirmation, before the notes stage; a dry run asks for nothing.
>
> Phase 7 built the abort and unwind at a pause (task 7-7) and the unattended and dry-run output (tasks 7-10, 7-11). Task 8-1 writes the answers and task 8-2 commits them.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.4 Reading the answer; §6.6 Recording the answers; §6.7 Unattended and dry runs; §6.8 When the check fails; Corrigenda (2026-10-19, §6.6)

## token-aware-diff-sizing-8-4

### Task 8-4: A failed answers write or commit unwinds and keeps nothing

**Problem**: Writing `.mint.toml` and committing it are mutations before the push, and either can fail: the file may not be writable, or git may refuse to stage or commit it (§6.6). The exception that keeps the answers commit through an unwind covers a commit that exists. A failed write or commit made none, and a half-finished one would leave mint's edit to the operator's config behind.

**Solution**: A failure writing `.mint.toml`, or staging or committing it, is a failure before the push. It is surfaced with the write error or git's error verbatim beneath its line. The run aborts before any tag and unwinds as built, back to the starting commit, taking the pre-tag artifact commit with it. The unwind leaves `.mint.toml` as it was before the answers were written, so a file the answers would have created is not left behind. No answers commit was made, so none is kept or re-committed, and nothing is recorded.

**Outcome**: A run whose answers cannot be recorded stops before any tag and leaves the repo, `.mint.toml` included, as it found it.

**Acceptance Criteria**:
- [ ] Given an attended release where the operator answers every pause and `.mint.toml` cannot be written, the run fails with the write error verbatim beneath its ✗ line, makes no notes call and no tag, and exits 1 (§6.6, §15.1)
- [ ] Given staging or committing `.mint.toml` fails, the run fails with git's error verbatim beneath its ✗ line, makes no notes call and no tag, and exits 1 (§6.6)
- [ ] Given a pre_tag hook that made the pre-tag artifact commit, and committing the answers fails, the unwind resets to the starting commit, taking the artifact commit with it. No commit carries the answers subject, `.mint.toml` reads as it did before the answers were written with no change to it staged, and the unwind's narration names no answers commit as kept (§6.6)
- [ ] Given a release with no pre_tag hook, where `.mint.toml` was written but committing it fails, `.mint.toml` is afterwards byte-identical to before the run, with no change to it staged or unstaged (§6.6)
- [ ] Given no `.mint.toml` before the run, and committing the answers fails, no `.mint.toml` exists afterwards (§6.6)
- [ ] Given any of these failures, the next release's check lists the answered paths again, since nothing was recorded (§6.6)

**Do**:
- The failure takes the release's pre-push failure route through the surgical unwind, `surfaceAndUnwind` and `Unwind` (`internal/engine/release.go`, `internal/engine/unwind.go`), as the pre-tag artifact and bookkeeping commits' failures do (§6.6, Corrigendum 2026-10-19).

**Context**:
> Corrigendum 2026-10-19 on §6.6: the answers commit is a pre-push mutation, and every other pre-push commit mint makes is surfaced and routed through the surgical unwind on failure. The keep exception covers an answers commit that exists, and a failed write or commit made none. CLAUDE.md's "Surgical unwind, then PONR" (mint removes only what it created) has the unwind take back mint's own write. §15.1: the answers are recorded before the first notes attempt.
>
> As built:
> - `Unwind` resets with `git reset --hard {start}` only when the run made commits, so a run with no other commit resets nothing, and `git reset --hard` leaves an untracked file in place.
> - Preflight confirmed a clean tree before the run changed anything (`--autostash` stashes the work tree first), so `.mint.toml` as it was before the answers is its committed state, or absent.
> - `surfaceAndUnwind` renders `failureMessage(cause)` on the ✗ line and `notesFailureOutput(cause)` beneath it, which is empty for any cause that is not an `ai.GenerationError`. For a non-zero exit, the runner's error carries the exit code, and git's stderr stays in the returned `runner.Result` (`internal/runner/exec_runner.go`); `record.CommitDirtyTree` and `CommitBookkeeping` wrap that error.
> - `git.Mutator` retries lock contention before a failure surfaces (`internal/git/mutator.go`).
>
> Task 8-5 keeps an answers commit that landed; task 8-8 covers `--autostash`.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.6 Recording the answers; §15.1 `r` on chunked notes; Corrigenda (2026-10-19, §6.6)

## token-aware-diff-sizing-8-5

### Task 8-5: An abort after answering keeps the answers commit

**Problem**: An answer is about the repo, not this release (§6.6). The surgical unwind removes every pre-push mutation mint made, so an abort after the operator answered would take the answers commit with it and bring the same questions back on the next run. A commit left at the top of the branch after an unwind also reads as an unwind that failed, unless the unwind says why it is there.

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
- [ ] Given the same release whose bookkeeping commit fails, or whose push is rejected after the tag was created, the run ends in the same state, with the same narration. After the rejected push, the bookkeeping commit is gone and the tag deleted (§6.6)
- [ ] Given a Ctrl-C while the notes call runs after the last pause, the run ends in the same state, with the same narration (§6.6, §11.5)
- [ ] Given a release with no pre_tag hook and a decline at the review gate after answering, the branch ends at the starting commit with the answers commit on top, and the unwind's narration names it as kept (§6.6)
- [ ] Given an abort that kept the answers commit, the next `mint release` neither lists nor pauses on the paths those answers cover (§6.6)
- [ ] The README's "One atomic point of no return" feature and its surgical-unwind bullet, `mint setup`'s point-of-no-return line, and CLAUDE.md's "Surgical unwind, then PONR" invariant each say that after an abort or failure before the push, the exclusion check's answers commit stays, re-committed on the starting commit (§18.1, §18.2, §18.3)

**Do**:
- The re-commit belongs to the surgical unwind, `Unwind` (`internal/engine/unwind.go`), which every pre-push abort and failure reaches; its narration rides the engine-authored `Unwound` summary (`surgicalSummary`) (§6.6).
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
> - The README's feature list says the unwind leaves "the repo exactly as it found it"; the setup guide's pipeline section says "Everything before it is unwindable; nothing after it is"; CLAUDE.md's invariant says every pre-push mutation is tracked and unwound on failure.
> - The comments that state the unwind's contract go stale with this change: the `unwind.go` file header and `Unwind`'s doc (the repo returned to its exact clean starting state), and the `autostash.go` header (the restore lands on the surgically-unwound clean state). §18.3 keeps comments true in the same change.
>
> Task 8-6 covers the reset or the re-commit failing.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.6 Recording the answers; §9.1 The trigger; §11.5 Ctrl-C mid-wave; §12.2 The choice; §15.1 `r` on chunked notes; §18.1 The setup guide; §18.2 The README; §18.3 CLAUDE.md and code comments; Corrigenda (2026-10-19, §12.2, §15.1)

## token-aware-diff-sizing-8-6

### Task 8-6: A failed reset or re-commit warns with the step that restores the answers

**Problem**: The unwind's reset and its re-commit of the answers are git mutations, and either can fail (§6.6). A failed re-commit leaves the answers off the branch. A failed reset leaves the answers commit on the branch among the commits the reset could not remove, and the manual reset the as-built warning names would then take the answers with it. Either way, naming the answers commit as kept would be false, and following the as-built steps alone would bring the same questions back on the next release.

**Solution**:
- When the re-commit fails, one warning names the manual step that restores the answers, with git's error verbatim beneath it, as a failed reset or tag delete during the unwind warns.
- When the reset fails, no re-commit is attempted, since the answers commit is still on the branch. The as-built warning that names the reset as the manual step names after it the step that restores the answers, because that reset takes the answers commit with it.

In both cases the unwind's narration does not name the answers commit as kept, and its summary reports manual cleanup required rather than a clean repo.

**Outcome**: When the unwind cannot keep the answers itself, the operator is told how to restore them, and the narration never claims a commit that is not there.

**Acceptance Criteria**:
- [ ] Given an abort after answering where the reset succeeds and the re-commit fails, mint prints one warning in the shape of the as-built `unwind incomplete` warning, naming the manual step that restores the answers, with git's error verbatim beneath it. The unwind's summary does not name the answers commit as kept and ends `; manual cleanup required`, and the run exits 1 (§6.6)
- [ ] Given an abort after answering where the reset fails, no re-commit is attempted. The reset's warning names the manual `git reset --hard {starting commit}` as built and, after it, the step that restores the answers, with git's error verbatim beneath it. The summary does not name the answers commit as kept and ends `; manual cleanup required` (§6.6)
- [ ] Given either warning, carrying out the steps it names, in the order it names them, leaves the branch at the starting commit with the answers' `.mint.toml` change committed on top (§6.6)

**Do**:
- The re-commit's warning takes the shape `warnUnwindIncomplete` gives a failed reset or tag delete, the restore step joins the reset's own warning in `resetCommitsIfMade`, and the summary's tail comes from `summaryTail` (`internal/engine/unwind.go`) (§6.6, Corrigenda 2026-10-19).

**Context**:
> Corrigenda 2026-10-19 on §6.6:
> - The re-commit failing: as built, a failed reset or tag delete during the unwind warns with the exact manual step and git's error verbatim (`warnUnwindIncomplete`), and the summary drops `; repo clean` for `; manual cleanup required` (`summaryTail`), so the narration never contradicts the warning. Naming the commit kept when it is not would be the false claim that rule exists to prevent.
> - The reset failing: a re-commit on a branch the reset never moved would duplicate the answers commit or commit nothing. The as-built reset warning gives a manual `git reset --hard {start}` that drops the answers along with mint's other commits, and following it would bring the same questions back on the next release, which §6.6's exception exists to prevent.
>
> As built:
> - `warnUnwindIncomplete` emits a `Warning` labelled `unwind incomplete`, with the message `automatic recovery failed; manually {action}` and the failing mutation's error (`cause.Error()`) as its `Output`.
> - `resetCommitsIfMade` names its action as resetting HEAD back to the starting commit, with the command `git reset --hard {start}`.
> - `Unwind` deletes the tag before it resets, and its summary reports a clean repo only when every recovery mutation succeeded.
>
> Task 8-5 built the re-commit and the narration that names the answers commit as kept.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.6 Recording the answers; Corrigenda (2026-10-19, §6.6)

## token-aware-diff-sizing-8-7

### Task 8-7: Commit-subject lists leave out the answers commit

**Problem**: The answers commit is mint's own tooling commit, and it lands inside release ranges: `{lastTag}..HEAD` on the run that made it, the next run's range when an abort kept it, and the version's own range when that version is regenerated. Every route that ends without AI notes ships the commit-subject list (§14.1), so without a filter `chore(release): release-notes path answers for v1.3.0` would land in a release's notes (§6.6).

**Solution**: Every commit-subject list leaves out subjects starting `chore(release): release-notes path answers for `, whatever tag follows: `--no-ai`'s list, the body `on_notes_failure = "fallback"` ships, the floor's `c`, and `c` at a later regenerate's floor over that version's range. Every other subject stays listed, the pre-tag artifact commit's included. A list left empty gives the maintenance-release body, as built.

**Outcome**: No commit-subject body lists mint's own answers commit.

**Acceptance Criteria**:
- [ ] Given an attended release of `v1.3.0` over `feat: add export` and `fix: handle empty input` that recorded answers, and whose notes call then fails under `on_notes_failure = "fallback"`, the body that ships lists `feat: add export` and `fix: handle empty input`, and not `chore(release): release-notes path answers for v1.3.0` (§6.6, §14.1)
- [ ] Given the same release under `on_notes_failure = abort`, `c` at the floor gives the same list (§6.6, §12.2)
- [ ] Given an earlier attempt at `v1.3.0` that aborted after answering and kept its answers commit on the branch, `mint release --no-ai` lists the range's other subjects without it (§6.6, §14.1)
- [ ] Given a released `v1.3.0` whose range holds `feat: add export`, `chore(release): pre-tag artifacts for v1.3.0`, `chore(release): release-notes path answers for v1.3.0` and its bookkeeping commit, `c` at a later `mint release regenerate v1.3.0` floor gives `feat: add export` and the pre-tag artifact subject, without the answers subject (§6.6, §12.3, §14.1)
- [ ] Given a range holding two answers commits, one kept by an aborted attempt and one from the run that released, both are left out (§6.6)
- [ ] A list with nothing left once the answers commits, and on regenerate the version's bookkeeping commit, are left out gives the maintenance-release body (§14.1)

**Do**:
- The forward range's list is built by `notes.FallbackBody` (`internal/notes/resolve.go`); regenerate's, over the version's own range, by Phase 5's floor `c` (task 5-10) (§14.1).

**Context**:
> §6.6: the commit-subject list leaves out subjects starting `chore(release): release-notes path answers for `, so the floor's `c` and `fallback` never list mint's own answers commit, and neither does `c` on a later regenerate. §14.1: every route that ends without AI notes gives the commit-subject list; on regenerate it also leaves out the version's bookkeeping commit; the pre-tag artifact commit stays; an empty list gives the maintenance-release body, so the tag body is never empty.
>
> §5.3: the answers commit changes only `.mint.toml`, a built-in always-exclude (task 7-3), so its diff never reaches a notes diff, and its subject is the one place it could surface. §7.1: a range whose post-exclusion diff is empty gets the maintenance-release body before `--no-ai` is consulted.
>
> As built, `notes.FallbackBody` runs `git log --format=%s {lastTag}..HEAD`, and the shared selector floors an empty or whitespace-only list to `StubBody()` (`internal/notes/resolve.go`, `internal/notes/degenerate.go`). Phase 3 made the commit-subject list the one non-AI body (task 3-6); Phase 5 built the floor's `c` on release (task 5-1) and on regenerate (tasks 5-9, 5-10).

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §5.3 Built-in always-excludes; §6.6 Recording the answers; §7.1 Where the body comes from; §12.2 The choice; §12.3 Regenerate at the floor; §14.1 The commit-subject list everywhere; Corrigenda (2026-10-19, §14.1)

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
