# Consolidation Findings: token-aware-diff-sizing (Phase 8)

## Findings

### F1: The unstage manual step prints the repo root unquoted, while the re-commit step in the same unwind quotes it
- **Class**: behaviour
- **Failure**: Take an operator whose repo root contains a space or a shell metacharacter (`~/Code/My App`, an iCloud `Mobile Documents` path). The answers commit fails and the unwind's unstage of `.mint.toml` fails too. The warning tells them to run `` `git -C /Users/x/Code/My App reset -q <sha> -- .mint.toml` ``. Pasted into a shell, this splits at the space and git stops with `fatal: cannot change to '/Users/x/Code/My': No such file or directory`, so the operator finds out mid-recovery that the printed step does not finish it. Two tasks wrote the copy-paste steps for this one unwind differently: the 8-6 re-commit step quotes the same root through `cmdline.Quote`, and the 8-4 unstage step does not. No test sees the gap, because `t.TempDir` roots never need quoting.
- **Evidence**:
  - `internal/engine/unwind.go:258`: `unstageEdit` concatenates `edit.root` and `edit.path` raw into the `git -C … reset -q … -- …` step.
  - `internal/engine/release.go:1130-1132`: `recommit` builds its `git -C … add` / `git -C … commit -m …` step from `cmdline.Quote(a.root)` and `cmdline.Quote(record.AnswersSubject(a.tag))`.
  - `internal/cmdline/quote.go:10`: `Quote`.
  - `internal/engine/release_answersfailure_test.go:280,311`: these tests assert the unstage message with the raw root and stay green after the fix, since `Quote` leaves TempDir roots unchanged. `internal/engine/release_answersunwindfailure_test.go:113-114` already relies on this for the re-commit step.
- **Proposed shape**: Build `unstageEdit`'s manual step from `cmdline.Quote(edit.root)` and `cmdline.Quote(edit.path)`, the same way `recommit` does, so every copy-paste step the answers unwind prints is quoted one way. `resetTo`'s hand-quoted `restoreLine` (`unwind.go:303`) is not part of this finding: it is unchanged since before the phase, and its `--notes-file` keep-out route never runs alongside answers, because a supplied body returns before the exclusion check (`internal/notes/select.go:184`).
- **Bank**: "Manual steps in unwind warnings quote shell words inconsistently: the re-commit step uses cmdline.Quote, two earlier steps do not" (8-6 reviewer). Confirmed for the unstage step only; the `resetTo` half is pre-existing and dropped. "Task 8-4 unstage warning builds a copy-paste command from an unquoted repo root; 8-6 quotes its own" (8-6 reviewer). Confirmed.

## Comment Corrections

- internal/engine/release.go:634-637: an answers commit landing after the artifact commit falsifies this. With no bookkeeping commit the tag sits on the answers commit, not on "any pre_tag artifact commit". The phase fixed the twin claim in the Stage 6 comment (`:729`) but not this one.
  OLD: // When changelog=false WriteChangelog no-ops (Changed:false). When the version file
	// also nets no change, CommitBookkeeping makes NO commit — the tag then points at the
	// existing HEAD (or any pre_tag artifact commit) and STILL carries the full body via
	// TagAndPush. Nothing durable is lost.
  NEW: // When changelog=false WriteChangelog no-ops (Changed:false). When the version file
	// also nets no change, CommitBookkeeping makes NO commit — the tag then points at
	// HEAD as it stands and STILL carries the full body via TagAndPush. Nothing durable
	// is lost.

## Spec Defects

### S1: Answers cannot be committed to a `.mint.toml` that git ignores, so every attended release that pauses fails
- **Claim**: §6.6: "mint then stages `.mint.toml` by path and commits it through the mutator" and "Answers live in `.mint.toml` rather than in state on one machine, so a teammate, CI or an agent session elsewhere is not asked the same question again".
- **Observed**: `record.CommitFile` runs `git -C {root} add .mint.toml` with no `-f` (`internal/record/commit.go:108`). git refuses to add an untracked path that a `.gitignore` matches: it exits 1 with "The following paths are ignored by one of your .gitignore files". This covers a config kept local on purpose, and also an absent `.mint.toml` under a broad ignore such as `.*`, which the write creates and the add then refuses. The clean-tree gate's `git status --porcelain` leaves ignored files out (`internal/preflight/preflight.go:96-103`), so such a repo gets through to the pauses. Following §6.6's "answers write or commit failing" bullet, the run aborts once the operator has answered every pause (`internal/engine/release.go:558-560`), and the unwind reverts the write (`internal/engine/unwind.go:240-247`). Nothing is recorded, so the next attended release raises the same pauses and aborts the same way. An attended release whose check raises a pause can never succeed in that repo. Only `-y`, a non-TTY run, `--no-ai` or `--notes-file` gets through.
- **Read**: genuinely open. The spec decided to commit the answers but never covers a config git will not track, and §6.6's failure bullet turns that into a permanent dead end rather than a one-off failure. Fixing it is a design choice: write the answers without committing them and say so; skip recording with a warning; or detect the ignored path (`git check-ignore`) before the pauses and handle it up front. Using `git add -f` would commit a config the operator deliberately kept out of the repo.
