TASK: A failed reset or re-commit warns with the step that restores the answers (token-aware-diff-sizing-8-6, tick-7b18f0)

ACCEPTANCE CRITERIA:
- Given an abort after answering where the reset succeeds and the re-commit fails, mint prints one warning in the as-built `unwind incomplete` form, naming the manual step that restores the answers, with git's own stderr from the failed re-commit verbatim beneath it. The unwind's summary does not name the answers commit as kept and ends `; manual cleanup required`, and the run exits 1 (§6.6)
- Given an abort after answering where the reset fails, no re-commit is attempted. The reset's warning names the manual `git reset --hard {starting commit}` as built and, after it, the step that restores the answers. The summary does not name the answers commit as kept and ends `; manual cleanup required` (§6.6)
- Given either warning, carrying out the steps it names, in the order it names them, leaves the branch at the starting commit with the answers committed on top (§6.6)

STATUS: complete

SPEC CONTEXT: §6.6 makes the exclusion answers the one exception to the surgical unwind: after the reset to the starting commit, the unwind re-commits the answers and narrates them as kept. The two bullets "The unwind's re-commit failing" and "The unwind's reset failing", plus the 2026-10-19 corrigenda (spec lines 992, 994, 996), require: one `unwind incomplete` warning naming the step that restores the answers, with git's stderr (or the write error) verbatim beneath it; no re-commit after a failed reset, the reset's warning naming the restore step after the manual reset; no "kept" claim; and the `; manual cleanup required` tail from `summaryTail`. Answers and a `--notes-file` KeepOut never co-occur: a supplied body wins before the exclusion check runs (internal/notes/select.go:184), so the soft-reset/restore form of the reset never carries the restore step.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/unwind.go:123 — `answersRestoreStep` runs before any unwind mutation, so HEAD is still mint's tip when the sha is read
  - internal/engine/unwind.go:157-170 — the restore step: reads `git rev-parse HEAD~{Commits-position}` and falls back to `ORIG_HEAD~N` if that read fails; the result is `restore the exclusion answers commit on top (\`git cherry-pick {sha}\`)`
  - internal/engine/unwind.go:197-210 — a failed reset appends `, then {restore}` to the as-built manual reset step (202-205)
  - internal/engine/unwind.go:141-150 — `recommitAnswersIfKept` skips the re-commit when the reset failed (142) and warns through `warnUnwindIncomplete` when the re-commit fails (145-147)
  - internal/engine/release.go:1138-1149 — `recommit`: a failed write returns the cherry-pick restore step with the write error; a failed stage/commit returns `git -C {root} add .mint.toml` then `git -C {root} commit -m '{subject}'` with git's stderr (via `gitOutput`, release.go:1165)
  - internal/engine/release.go:552-556 — `position` is recorded when the answers commit lands
  - internal/engine/unwind.go:132, 353-379 — `kept=false` on either failure keeps the summary from naming the answers commit as kept; `clean=false` gives `; manual cleanup required`
- Notes: The manual steps hold by reading. When staging or committing fails, the work tree holds the re-written answers on top of the reset start, so `add` and then `commit` produce the answers commit on the starting commit. When the write fails, `fsutil.WriteFile` is atomic (temp file then rename, internal/fsutil/atomicwrite.go:26-53), so `.mint.toml` is left at its starting content and the cherry-pick applies. When the reset fails, the manual `reset --hard` followed by the cherry-pick of the sha read up front restores the answers. The `ORIG_HEAD~N` fallback is also correct, because the manual reset (or the automatic reset that succeeded) sets ORIG_HEAD to mint's tip before the cherry-pick. The `-C root` form is used only where a path is relative. Nothing drifts from the task or the spec.

TESTS:
- Status: Adequate
- Coverage (internal/engine/release_answersunwindfailure_test.go):
  - :91 — the re-commit's staging or commit fails after a successful reset. Asserts exactly one warning with the exact manual add+commit message, git's stderr verbatim as Output, the summary `reset 2 commits; manual cleanup required`, exit 1, one Unwound, and the answers left in `.mint.toml` for the named step.
  - :153 — the re-commit's write fails, with the answers commit at the tip (HEAD) and below the bookkeeping commit (HEAD~1). Asserts the cherry-pick restore step with the read sha, the write error verbatim, the summary, that the sha is read immediately before the first mutation, and that no git call follows the reset.
  - :209 — the reset fails, in both positions. Asserts no git call after the failed reset (no re-commit), the warning `reset ... (\`git reset --hard {start}\`), then restore ... (\`git cherry-pick {sha}\`)` with the reset's stderr, and the summary in both positions.
  - :266 — the sha read fails. Asserts the `ORIG_HEAD~1` fallback.
  - The unwind's success path (release_answerskept_test.go) was updated to script the new sha read.
- Notes: Each test would fail if its behaviour broke. The exact warning, Output, summary and argv assertions follow the project's test idioms. Table-driven subtests cover the position arithmetic without duplicating tests. No real git runs (FakeRunner), so whether the named steps work against git itself is recorded under UNSETTLED.

CODE QUALITY:
- Project conventions: Followed. Mutations go through the Mutator and the sha read through the Mutator's pass-through `Run`, as `record.IsIgnored` does. Output goes through the presenter. Failure output is git's stderr verbatim (later task 12-2 extended this to the reset's own Output).
- SOLID principles: Good. The restore step is computed once and threaded to both consumers.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good. The doc comments on `recommit`, `recommitAnswersIfKept`, `answersRestoreStep`, `resetCommitsIfMade` and `summaryTail` hold against the code.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "Given either warning, carrying out the steps it names, in the order it names them, leaves the branch at the starting commit with the answers committed on top (§6.6)" — reading supports it (see IMPLEMENTATION Notes), but every test runs on FakeRunner. To settle it, run against a real git repo: take an attended release with an Exclude answer and a pre_tag artifact commit to an abort, force (a) the re-commit's `git commit` to fail, (b) the re-commit's write to fail, (c) the unwind's `git reset --hard` to fail (and the sha read to fail, for the ORIG_HEAD~N form), then run each warning's named commands in order and check that `git log` shows the starting commit with one answers commit on top holding the Exclude glob.
