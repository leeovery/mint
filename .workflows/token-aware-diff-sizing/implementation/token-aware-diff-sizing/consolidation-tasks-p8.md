# Consolidation Tasks: Token Aware Diff Sizing (Phase 8)

## Task 1: Write the answers without committing them when git ignores `.mint.toml`
placement: phase 8
severity: behaviour

**Problem**: `record.CommitFile` stages the answers with `git -C {root} add .mint.toml` and no `-f` (`internal/record/commit.go:108`). git refuses to stage an untracked path that a `.gitignore` rule matches. That covers a config kept local on purpose, and an absent `.mint.toml` under a broad rule such as `.*`, which the answers write creates and the stage then refuses. The clean-tree check leaves ignored files out (`internal/preflight/preflight.go:96-103`), so such a repo reaches the pauses. The run then aborts once the operator has answered every pause, and the unwind reverts the write (`internal/engine/release.go:558-560`, `internal/engine/unwind.go:240-247`). Nothing is recorded, so the next attended release raises the same pauses and aborts the same way. An attended release whose check raises a pause can never succeed in that repo.
**Solution**: Follow §6.6 as corrected on 2026-10-21. Before staging, the answers commit asks git, read-only and at the repo root, whether `.mint.toml` is untracked and ignored. `git check-ignore` reports nothing for a tracked path, so a tracked `.mint.toml` stays on today's route. When it is untracked and ignored:
- the answers are written as built, but not staged or committed;
- the recording line says the answers were written to `.mint.toml`'s `[release]` table and not committed because git ignores the file;
- the release goes on;
- an abort or failure after answering keeps the write, as it keeps an answers commit, so the unwind's revert of an uncommitted answers write does not apply to it.

A failed write still unwinds as built. The README's description of the recorded answers says what happens to an ignored `.mint.toml`. Derivation: §6.6's purpose that nobody is asked again; the `--autostash` rule that mint never commits work the operator never chose to commit, which rules out `git add -f`; and the clean-tree check already treating ignored files as the operator's business.
**Outcome**: An attended release in a repo that ignores `.mint.toml` records its answers locally, says they were not committed, and succeeds. The next release on that clone asks nothing about those paths.

**Acceptance Criteria**:
- [ ] An attended release in a repo whose `.mint.toml` is untracked and matched by a `.gitignore` rule, whether the file exists or is absent under a broad rule such as `.*`: once the last pause is answered, the answers sit in `.mint.toml`'s `[release]` table as §6.6 writes them (an absent file is created holding only that table), no `git add` of `.mint.toml` runs, no commit carries the answers subject, and the release goes on through its notes, review gate, tag and push (§6.6).
- [ ] On that route the recording line, shown after the last pause and before the notes line resumes, says the answers were written to `.mint.toml`'s `[release]` table and not committed because git ignores the file; plain mode prints it as its own line. Neither mode says the answers were committed (§6.6, §16.1).
- [ ] A tracked `.mint.toml`, one a `.gitignore` rule matches included, and an untracked or absent `.mint.toml` that no rule matches, take today's route: `.mint.toml` is staged by path and committed as `chore(release): release-notes path answers for {tag}`, and the recording line says the answers were written and committed (§6.6).
- [ ] Having answered on the ignored route, an abort or failure before the push (`n` at the review gate, or a later stage such as the bookkeeping commit failing) leaves `.mint.toml` holding the answers: the unwind runs no unstage of `.mint.toml`, neither restores its earlier content nor removes a file the write created, and undoes the run's other commits and tag as built. Where the unwind narrates, its summary names no answers commit as kept (§6.6).
- [ ] The next attended release on that clone reads the answers from `.mint.toml`: the paths they answered are covered, so the check raises no pause about them (§6.6).
- [ ] On the ignored route a write that fails, such as to an unwritable `.mint.toml`, fails as it does today: the ✗ line `recording the exclusion answers` with the write error verbatim beneath it, and the run aborts before any tag, unwinding as built (§6.6).
- [ ] The README's account of the recorded answers says that when git ignores an untracked `.mint.toml`, the answers are written but not committed, the release goes on, and the write survives an abort or failure before the push (§18.2).

**Do**:
- The probe is `git check-ignore` on `.mint.toml`, run at the repo root before the answers are staged. It is a read: the plain runner, or the Mutator's `Run` pass-through that `record.CommitDirtyTree`'s status probe uses (`internal/record/commit.go:34-36`), never `Mutate`. git exits 0 when the path is ignored and 1 when it is not, a tracked path never counting as ignored; the runner returns a non-zero exit as an error with `Result.ExitCode` readable (`internal/runner/runner.go:23-31`).
- The ignored route never stages `.mint.toml`: no `git add -f`.
- The answers commit is `answersCommit.record` and `commit` (`internal/engine/release.go:1075-1120`). The revert the ignored route must not reach is `revertEditIfMade` (`internal/engine/unwind.go:237-247`), fed by `made.answersEdit = answers.edit` (`internal/engine/release.go:557`).
- The recording line is the `ExclusionAnswersRecorded` event: payload `presenter.ExclusionAnswers`, text `exclusionAnswersText` (`internal/presenter/presenter.go:576-586`), rendered by `PrettyPresenter` (`internal/presenter/pretty.go:615-618`) and `PlainPresenter` (`internal/presenter/plain.go:232-235`), recorded by `presentertest.RecordingPresenter`. The engine's payload carries which route the answers took; the presenter derives nothing.
- The README's recorded-answers account is the paragraph at `README.md:117`.
- The answers commit's git calls are scripted in sequence by `answersCommitCalls`/`withAnswersCommit` (`internal/engine/release_answerscommit_test.go:32-39`), `stageFails`/`commitFails` (`internal/engine/release_answersfailure_test.go:39-41`) and `internal/engine/release_answerscommit_test.go:174`; the probe's answer joins every sequence that reaches it. An exit-0 `ScriptedOut("")` answers the probe as ignored; not ignored is an exit-1 call.

## Task 2: Quote the repo root and path in the unstage manual step
placement: phase 8
severity: behaviour

**Problem**: When the answers commit fails and the unwind's unstage of `.mint.toml` fails too, the `unwind incomplete` warning tells the operator to run `git -C {root} reset -q {sha} -- .mint.toml`. `unstageEdit` builds that command from `edit.root` and `edit.path` raw (`internal/engine/unwind.go:258`). In a repo root containing a space or a shell metacharacter, such as `~/Code/My App` or an iCloud `Mobile Documents` path, the pasted step splits at the space and git stops with `fatal: cannot change to '/Users/x/Code/My'`. The operator finds out mid-recovery that the printed step does not finish it. The re-commit step in the same unwind already quotes the same root with `cmdline.Quote` (`internal/engine/release.go:1130-1132`). No test sees the gap, because `t.TempDir` roots never need quoting.
**Solution**: Build `unstageEdit`'s manual step from `cmdline.Quote(edit.root)` and `cmdline.Quote(edit.path)`, as `recommit` does, so every copy-paste step the answers unwind prints is quoted the same way. Add a test whose root contains a space and that pins the quoted step. `resetTo`'s hand-quoted restore line (`internal/engine/unwind.go:303`) is not in scope: it predates the phase, and its `--notes-file` keep-out route never runs alongside answers (`internal/notes/select.go:184`).
**Outcome**: The unstage step the answers unwind prints, pasted into a POSIX shell, runs the command mint ran whatever the repo root holds, and every copy-paste step that unwind prints writes the root the same way.

**Acceptance Criteria**:
- [ ] In a repo whose root contains a space (`…/My App`), the answers commit fails and the unwind's unstage of `.mint.toml` fails too: the `unwind incomplete` warning reads ``automatic recovery failed; manually unstage .mint.toml (`git -C '…/My App' reset -q {sha} -- .mint.toml`)``, the root single-quoted as `cmdline.Quote` writes it, with git's stderr verbatim beneath it (§6.6).
- [ ] In a root made only of `[A-Za-z0-9_@%+=:,./-]`, as every `t.TempDir` root is, the step reads as today with the root bare: the warnings pinned at `internal/engine/release_answersfailure_test.go:278-282` and `:309-313` hold unchanged (§6.6).

**Do**:
- The step is built in `unstageEdit` (`internal/engine/unwind.go:249-260`); `cmdline.Quote` is `internal/cmdline/quote.go:10`; `recommit` (`internal/engine/release.go:1125-1136`) is the pattern it follows.
- The measured set: ``rg -n '\(`git ' internal/engine -g '!*_test.go'`` gives 10 hits. Five are comments (`unwind.go:50`, `autostash.go:5`, `autostash.go:6`, `release.go:734`, `release.go:1348`); five are printed steps: `unwind.go:167` (a sha), `unwind.go:182` (a tag), `unwind.go:258` (the unstage step, root and path raw), `unwind.go:296` (a sha) and `release.go:1132` (the re-commit step, root quoted). ``rg -n '`git -C ' internal -g '!*_test.go'`` confirms only `unwind.go:258` and `release.go:1132` print a repo root, its other hits being comments, so `unwind.go:258` is the whole set to convert. `resetTo`'s `restoreLine` (`unwind.go:303`) stays out of scope.
