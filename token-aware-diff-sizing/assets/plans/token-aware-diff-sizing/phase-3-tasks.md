# Phase 3: `--notes-file` and one non-AI body — 6 tasks

## token-aware-diff-sizing-3-1

### Task 3-1: Release with a supplied notes body

**Problem**: An operator or agent holding finished notes has no direct way to release with them. The workaround is to write the notes into `[release].fallback`, release with `--no-ai`, then remove them again. That costs an extra clean-up commit on main for every such release (§13.1). Later phases produce bodies meant to be released the same way, such as the saved chunk notes a failed chunked run leaves in the user cache dir (§11.4). The floor's guidance tells the operator to release those with `mint release --notes-file <path>` (§12.4).

**Solution**: Add `--notes-file <path>` to `mint release`:
- mint reads the file once at startup, before any stash, check or gate, and holds the body in memory
- the path resolves against the directory mint was run from, and there is no stdin form
- the supplied body is the first match in the release's notes routing (§7.1): it wins over the first-release and empty-diff bodies
- it skips the exclusion check, the size cap, chunking and every AI call, and `on_notes_failure` never applies.

An attended run shows the body at the review gate a non-AI body gets: accept, edit and abort, with no `r`. Under `-y` that gate auto-accepts inside the presenter. The body reaches the tag message, changelog entry and provider release byte-identical. The usage text and the README document the flag.

**Outcome**: `mint release --notes-file P` releases with P's contents as the notes and makes no AI call. On an attended run the operator sees the body at the review gate first.

**Acceptance Criteria**:
- [ ] Given a release with a prior tag, a non-empty diff and a notes file outside the repo, `mint release -y --notes-file <path>` makes no AI call of any kind. It ships the file's contents byte-identical, as an AI body ships, in the tag message, the changelog entry and the provider release (§13.3)
- [ ] Given `--notes-file`, the supplied body ships with no AI call in each of these cases (§7.1, §7.2, §13.3):
  - a first release, where it ships instead of `Initial release.`
  - an empty post-exclusion diff, where it ships instead of the maintenance-release body
  - `on_notes_failure = "fallback"`
  - a live dry-run cache entry for this release's diff, where no `Cached notes found for this diff — use them?` question is asked
- [ ] Given an attended run, the body shows at the review gate `Release with these notes?`, which offers accept, abort and edit, with no `r` (§13.3). Choosing edit opens the editor on the supplied body, and the saved text is what ships, while the notes file on disk is unchanged (§13.2). Choosing abort ends the run before any tag
- [ ] Given `-y`, the review gate auto-accepts inside the presenter with its as-built `notes: accepted (-y)` echo, and the supplied body ships (§13.3)
- [ ] Given mint run from `<repo>/sub` with `--notes-file notes.md`, where committed files `<repo>/sub/notes.md` and `<repo>/notes.md` both exist, the body is the contents of `<repo>/sub/notes.md` (§13.1)
- [ ] Given `--notes-file -` and a file named `-` in the directory mint was run from, the body is that file's contents, and stdin is never read for the body (§13.1)
- [ ] Given a notes file outside the repo that a pre_tag hook rewrites during the run, the body shipped is the contents mint read at startup (§13.2)
- [ ] `mint release --help` lists `--notes-file P` with the description `release with the finished notes in file P (no AI)` (§13.1, §18.3)
- [ ] The README's `release` section documents `--notes-file` (§18.2):
  - a row in its flag table
  - that it releases a finished body with no AI
  - that an attended run shows the body at the review gate first

**Do**:
- The flag joins `mint release`'s flag set (`parseReleaseFlags` in `cmd/mint/flags.go`) and its curated usage text (`releaseUsage` in `cmd/mint/usage.go`), where a test pins coverage (§13.1, §18.3).
- The README row goes in the `release` section's flag table (§18.2).

**Context**:
> §7.1, where a release's body comes from, first match wins:
> 1. `--notes-file`
> 2. a first release (`Initial release.`)
> 3. an empty post-exclusion diff (the maintenance-release body)
> 4. `--no-ai` (the commit-subject list)
> 5. the AI path.
>
> The exclusion check (Phase 7) and cache reuse are steps 1 and 2 of the AI path (§7.2). A supplied body never reaches either.
>
> §13.3: the first-release and empty-diff bodies exist to produce an honest body when there is nothing for an AI to summarise, and a supplied body does not need one. `on_notes_failure` governs only an AI failure. §13.1: there is no stdin form because the review gate reads its keypresses from stdin, and an agent can always write a file.
>
> As built:
> - The notes-path precedence runs in `notes.Selector.SelectBodyWithReuse` (`internal/notes/select.go`), reached from `resolveBody` in `internal/engine/release.go`. `ReleaseOptions.NotesBody` is a test-injection override that bypasses the selector.
> - `gateForKind` (`internal/engine/release.go`) gives `notes.KindNormalAI` `presenter.ReleaseReviewGate()` (y/n/e/r). Every other Kind gets `FirstReleaseReviewGate()` (y/n/e, `internal/engine/engine.go`).
> - The version confirmation is the release's first gate. It comes before `--autostash` and preflight.
> - `mint release` loads config inside the engine; the cmd layer only parses flags.
>
> CLAUDE.md seam 4: business logic always calls `Prompt(gate)`, and the `-y` auto-accept happens inside the presenter.
>
> Task 3-2 keeps a notes file inside the repo out of the clean-tree check, `--autostash` and every commit. Until it lands, such a file counts against the clean-tree check as built. Task 3-4 keeps the file as it was through an unwind. Task 3-5 refuses unusable invocations, an empty path among them.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §7.1 Where the body comes from; §7.2 The AI path, in order; §11.4 The saved chunk-notes file; §12.4 The ✗ line and the next moves; §13.1 What it is; §13.2 The file is outside everything mint does; §13.3 How a supplied body moves through the run; §18.2 The README; §18.3 CLAUDE.md and code comments

## token-aware-diff-sizing-3-2

### Task 3-2: Keep the notes file out of the release's git steps

**Problem**: The obvious first try is an agent writing `notes.md` at the repo root and running `mint release -y --notes-file notes.md` (§13.2). As built, three release steps break it:
- the clean-tree check refuses the untracked file
- `--autostash` sweeps it into the stash
- the pre-tag artifact commit's `git add -A` stages it into a release commit.

A tracked file with uncommitted edits, such as a `NEXT_RELEASE.md` drafted afresh each cycle, fares the same. Any of its edits already staged would also ride the bookkeeping commit.

**Solution**: Keep the notes file outside every git step the release runs. This holds whether the file is untracked or tracked with uncommitted edits, staged or not:
- the clean-tree check does not count it
- `--autostash` leaves it in place instead of stashing it
- no commit the release makes contains it, neither the pre-tag artifact commit nor the release bookkeeping commit.

Every other path is treated as built. Afterwards the file sits in the work tree untouched: still untracked, or with its edits intact, staged ones still staged and unstaged ones unstaged. The README says so.

**Outcome**: An agent can write the notes into the repo and release with them in one command. The release's commits carry none of the file, and the file is left exactly as it was.

**Acceptance Criteria**:
- [ ] Given a tree that is clean apart from an untracked `notes.md` at the repo root, `mint release -y --notes-file notes.md` passes the clean-tree check and releases. Afterwards `notes.md` is still untracked with its contents unchanged, and no commit the release made contains it (§13.2)
- [ ] Given `--notes-file NEXT_RELEASE.md` naming a tracked file with uncommitted edits, whether those edits are staged or unstaged (§13.2):
  - the clean-tree check passes
  - neither the pre-tag artifact commit nor the release bookkeeping commit contains those edits
  - afterwards the file carries the same edits, staged if they were staged and unstaged if not
- [ ] Given the notes file and one other untracked or modified path, the clean-tree check still fails as built (§13.2)
- [ ] Given `--autostash`, the notes file (untracked, or tracked with uncommitted edits) and other uncommitted changes, the other changes are stashed and restored as built. The notes file stays in the work tree through the run with its contents and staged state unchanged. Given the notes file is the only uncommitted change, nothing is stashed and no `git stash pop` runs (§13.2)
- [ ] Given a pre_tag hook that changes files, the artifact commit holds the hook's changes and not the notes file. Given a pre_tag hook that changes nothing, no artifact commit is made, even though the notes file is untracked or carries uncommitted edits (§13.2)
- [ ] Given mint run from a subdirectory with a relative `--notes-file` path inside the repo, the file that path names is the one left out of the clean-tree check, the stash and the commits (§13.1, §13.2)
- [ ] The README's `--notes-file` documentation with `mint release` says the file may sit in the repo, untracked or with uncommitted edits. It is kept out of the clean-tree check, `--autostash` and every commit the release makes, and left as it was (§13.2, §18.2)

**Context**:
> §13.2, the file is outside everything mint does:
> - mint reads the file once at startup, before any stash, check or gate, and holds the body in memory.
> - The file does not count against the clean-tree check.
> - It is left out of `--autostash`'s stash.
> - It is never staged or committed by any git step mint runs, including the pre-tag artifact commit's `git add -A`.
> - Afterwards it sits in the work tree untouched. Cleaning it up is the caller's business.
> - All of this holds whether the file is untracked or a tracked file with uncommitted edits. After a successful run, edits the operator had staged are still staged, and unstaged ones unstaged (Corrigendum 2026-10-19 on §13.2).
>
> Keeping the file out of git is mint's job, not only the caller's (§13.2).
>
> As built:
> - `preflight.CheckCleanTree` (`internal/preflight/preflight.go`) fails on any `git status --porcelain` entry. The forward release reaches it through `RunLocalGates`. Regenerate reaches it through `RegeneratePreflight`, which task 3-3 covers.
> - `autostashPush` (`internal/engine/autostash.go`) runs `git stash push --include-untracked`. It reads git's `No local changes to save` as nothing stashed, so no pop is owed.
> - `record.CommitDirtyTree` (`internal/record/commit.go`) probes porcelain, then runs `git add -A` and commits. Any dirty path, the notes file included, therefore triggers an artifact commit.
> - `record.CommitBookkeeping` stages the changelog and version file by path, then runs `git commit -m`, which also commits whatever else is already staged.
> - Porcelain paths are relative to the repo root, while the flag's path resolves against the directory mint was run from (§13.1).
>
> CLAUDE.md seam 1: read-side git that uses cwd-relative pathspecs runs at the repo root via `RunInDir`. Seam 2: every git mutation goes through `git.Mutator`.
>
> Task 3-4 keeps the file as it was through the unwind's reset.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §13.1 What it is; §13.2 The file is outside everything mint does; §18.2 The README; Corrigenda (2026-10-19, §13.2)

## token-aware-diff-sizing-3-3

### Task 3-3: Regenerate one version from a notes file

**Problem**: A version already out can take new notes only from a fresh AI run, its tag annotation or its provider release (§13.1). There is no way to put a finished body on it. Later phases leave such bodies behind. The saved chunk notes of a failed chunked regenerate, or of a forward release that shipped under `fallback`, go onto the version with `mint release regenerate <version> --notes-file <path>` (§11.4, §12.4). An operator with hand-written notes for an old version has the same need.

**Solution**: Add `--notes-file <path>` to `mint release regenerate <version>` as a notes source beside fresh, tag and release:
- giving it decides the source, so the source question is not asked
- mint reads the file once at startup, before any check or gate, holds the body, and makes no AI call
- the body shows at regenerate's own review gate with accept, edit and abort and no `r`, not the simple confirm the tag and release sources show; under `-y` that gate auto-accepts
- the body reaches the changelog section and the provider release byte-identical, for whichever target
- on a changelog or both target, the notes file is kept out of regenerate's clean-tree check and its changelog commit, and is left as it was afterwards.

The usage text and the README document the flag.

**Outcome**: `mint release regenerate <version> --notes-file P` puts P's contents on the version's chosen surfaces with no AI call. On an attended run the operator sees the body at regenerate's review gate first.

**Acceptance Criteria**:
- [ ] Given a released version, `mint release regenerate <version> --notes-file notes.md --target both -y` asks no source question and makes no AI call. The changelog section and the provider release receive the file's contents byte-identical. With `--target release` only the provider release is written, and with `--target changelog` only the changelog (§13.1, §13.3)
- [ ] Given the oldest version, which has no prior tag, the supplied body is written, not `Initial release.` (§13.1, §13.3)
- [ ] Given an attended run, the plan shown before the gate does not name the source as `fresh`, `tag` or `release` (§13.1)
- [ ] Given an attended run, the body shows at regenerate's review gate, offering accept, edit and abort with no `r`, not the accept/abort confirm the `tag` and `release` sources show. Choosing edit changes the body written, not the file on disk, and choosing abort writes nothing. Under `-y` the gate auto-accepts inside the presenter (§13.2, §13.3)
- [ ] Given a `changelog` or `both` target and either an untracked notes file in the repo or a tracked one with uncommitted edits, staged or unstaged (§13.2):
  - regenerate's clean-tree check passes
  - the changelog commit contains only `CHANGELOG.md`
  - afterwards the file is as it was: still untracked, or carrying the same edits, staged if they were staged and unstaged if not
  - any other dirty path still fails the check as built
- [ ] Given a relative path, it resolves against the directory mint was run from. The body written is the file's contents as read at startup (§13.1, §13.2)
- [ ] `mint release regenerate --help` lists `--notes-file P` with the description `notes source: the finished notes in file P (one version only)` (§13.1, §18.3)
- [ ] The README's `release regenerate` section documents `--notes-file` (§18.2):
  - a row in its flag table
  - that it is a notes source beside fresh, tag and release that runs no AI
  - that it serves one version only

**Do**:
- The flag joins regenerate's flag set (`parseRegenerateFlags` in `cmd/mint/regenerate_flags.go`) and its curated usage text (`regenerateUsage` in `cmd/mint/usage.go`), where a test pins coverage (§13.1, §18.3).
- The README row goes in the `release regenerate` section's flag table (§18.2).

**Context**:
> §13.1: one file answers one version. §13.3 and its corrigendum: the review gate shows on both verbs so the operator sees what will ship. On regenerate it is regenerate's own review gate without `r`. Phase 5 sends regenerate's floor `c` and `e` bodies through this same gate (§12.3).
>
> As built:
> - Regenerate's source axis is `RegenerateSource` (`internal/engine/regenerate_write.go`). `gateRegenerate` picks the gate through `usesReviewGate`:
>   - a fresh source runs `presenter.NotesReviewGate()` (`Use these notes?`, y/n/e/r)
>   - the tag and release sources run `presenter.ReuseConfirmGate()` (y/n).
> - The source question is `resolveSource` (`internal/engine/regenerate_interactive.go`). The plan's source step comes from `sourceLabel`, which labels any source it does not recognise `fresh`.
> - Body production per source is dispatched in `newBatchBodyProducer` (`cmd/mint/regenerate_all.go`). The cmd-to-engine source mapping is `regenerateSourceAxis` (`cmd/mint/regenerate_run.go`).
> - A fresh source on the oldest version returns `Initial release.` with no AI (`RegenerateFreshBody`, `internal/engine/regenerate_fresh.go`).
> - The target axis is orthogonal to the source. An unset `--target` is asked at a terminal. `-y` with no `--target` is the usage error `--target is required with -y` for every source (`validateRegenerateRequest`, `cmd/mint/regenerate_validate.go`).
> - Regenerate runs the clean-tree check only for a changelog or both target (`RegeneratePreflight`, `internal/engine/regenerate.go`). Its changelog commit, `stageAndCommitChangelog`, stages `CHANGELOG.md` by path, then runs `git commit -m`, which also commits whatever else is already staged.
> - `splitRegeneratePositional` lifts the `<version>` positional out before flag parsing. It passes a separate-token value through only for the flags `isValueFlag` names (`--source`, `--target`). As built, the path in `--notes-file <path>` would therefore be read as a second positional.
> - Regenerate loads config in the cmd layer (`runRegenerate`, `cmd/mint/main.go`).
>
> Task 3-2 built the release's exclusions of the notes file from the clean-tree check and from commits. Task 3-4 keeps the file as it was through regenerate's reset after its changelog commit. Task 3-5 refuses `--all`, `--source` and unusable files.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §11.4 The saved chunk-notes file; §12.3 Regenerate at the floor; §12.4 The ✗ line and the next moves; §13.1 What it is; §13.2 The file is outside everything mint does; §13.3 How a supplied body moves through the run; §18.2 The README; §18.3 CLAUDE.md and code comments; Corrigenda (2026-10-19, §13.2 and §13.3)

## token-aware-diff-sizing-3-4

### Task 3-4: A failed run leaves the notes file as it was

**Problem**: A run that fails after mint has made a commit recovers with `git reset --hard`. That is the forward release's surgical unwind back to the starting commit, and regenerate's reset after its changelog commit (§13.2). The reset discards a tracked file's uncommitted edits and resets the index. A `NEXT_RELEASE.md` draft passed as `--notes-file` would be wiped by the very run that failed, and any edits staged in it unstaged, although mint removes only what it created.

**Solution**: Both resets leave the notes file as it was. A tracked notes file keeps its content and whether its edits were staged, and an untracked one sits untouched. Otherwise the recovery removes what it does as built: the commits and the tag mint made, and the work-tree changes those commits carried.

**Outcome**: However a run ends, the operator's notes draft survives with its content and staged state. The repo is otherwise back where mint found it.

**Acceptance Criteria**:
- [ ] Given `mint release --notes-file NEXT_RELEASE.md` on a tracked file with uncommitted edits, and a pre_tag hook whose artifact commit lands, the operator aborts at the review gate. The unwind resets the artifact commit, and the file carries the same edits it had, staged if they were staged and unstaged if not (§13.2)
- [ ] Given that file and a failure before the push after the bookkeeping commit, such as a rejected atomic push, the unwind resets mint's commits and deletes the tag mint created, as built. The file carries the same edits, with the same staged state (§13.2)
- [ ] Given that file and a Ctrl-C after mint has made a commit, the unwind runs as built, and the file carries the same edits, with the same staged state (§13.2)
- [ ] Given that file, `--autostash` with other uncommitted changes, and a failure before the push, the other changes are restored after the unwind as built. The notes file carries the same edits, with the same staged state (§13.2)
- [ ] Given `mint release regenerate <version> --notes-file NEXT_RELEASE.md --target changelog` on that file, the push of the changelog commit fails. Regenerate resets the changelog commit as built, and the file carries the same edits, with the same staged state (§13.2)
- [ ] Given an untracked notes file inside the repo, it sits untouched with its contents after either reset (§13.2)
- [ ] In each case HEAD is back at the starting commit, and the rest of the work tree is as the as-built reset leaves it: the changelog write, any version-file projection and the hook's artifacts are gone (§13.2)

**Context**:
> Corrigenda 2026-10-19 on §13.2: a reset during the forward release's surgical unwind or regenerate's reset after its changelog commit leaves the file as it was, tracked or untracked: its content, and whether its edits were staged. The grounds are CLAUDE.md's "Surgical unwind, then PONR" invariant, that mint removes only what it created, and that the operator's index state is not mint's.
>
> As built:
> - `Unwind` (`internal/engine/unwind.go`) deletes the tag mint created with `git tag -d`. When mint made commits it resets with `git reset --hard {start.HEAD}`; with none made it issues no reset. It runs detached from cancellation (`context.WithoutCancel`), so a Ctrl-C before the push still unwinds.
> - `autostashPop` is deferred, so it runs after the unwind has returned the repo to its starting state.
> - Regenerate's `resetAndAbort` (`internal/engine/regenerate_write.go`) resets with `git reset --hard {startingHEAD}` when a changelog commit landed, also detached from cancellation. A gate abort on regenerate lands before the changelog commit, so nothing is reset.
> - `git reset --hard` resets tracked paths in both the index and the work tree. An untracked file mint never staged survives it.
> - Both resets go through `git.Mutator` (CLAUDE.md seam 2).
>
> Tasks 3-2 and 3-3 keep the file out of every commit mint makes, so no commit a reset drops ever holds it. Phase 8 later adds the one deliberate exception to the unwind, the exclusion check's answers commit. That commit never arises under `--notes-file`, since the check does not run with it (§6.1).

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §13.2 The file is outside everything mint does; §6.1 When it runs; Corrigenda (2026-10-19, §13.2)

## token-aware-diff-sizing-3-5

### Task 3-5: Refuse a `--notes-file` that cannot be used

**Problem**: Some `--notes-file` invocations cannot be acted on (§13.4):
- With `--no-ai`, the two flags give two answers to where the notes come from.
- On regenerate, one file answers one version, so `--all` or `--source` contradicts it (§13.1).
- An empty file would ship an empty tag message, changelog entry and provider release, since git accepts an empty tag message (§12.2).
- A missing or unreadable file has nothing to ship.
- An empty path, such as `--notes-file ""` from an unset variable, read as the flag left off would send the run down the AI path the caller meant to skip, unreviewed under `-y`.

Each has to stop before the run has done anything.

**Solution**: Refuse each case before any step runs:
- contradictory flags are usage errors, exit `2`
- an empty or whitespace-only file fails with `--notes-file P is empty`, exit `1`
- a missing or unreadable file fails with `reading --notes-file P: <cause>`, exit `1`
- an empty path names no file and fails as a missing file does, exit `1`.

The file and path refusals hold on both verbs.

**Outcome**: A bad `--notes-file` invocation stops at once with one clear line and the right exit code, having stashed, asked, called and changed nothing.

**Acceptance Criteria**:
- [ ] Given `mint release --notes-file notes.md --no-ai`, mint exits 2 with a usage error (§13.4)
- [ ] Given `mint release regenerate --all --notes-file notes.md`, or `mint release regenerate <version> --notes-file notes.md --source <value>` for any value, `fresh` included, mint exits 2 with a usage error (§13.1, §13.4)
- [ ] Given a notes file that is empty or holds only whitespace, `mint release --notes-file P` and `mint release regenerate <version> --notes-file P` each fail with `--notes-file P is empty`, P being the path as given, and exit 1 (§13.4)
- [ ] Given a path that does not exist, or a file mint cannot read, each verb fails with `reading --notes-file P: <cause>`, `<cause>` being the read error, and exits 1 (§13.4)
- [ ] Given `--notes-file ""` on either verb, the run fails as a missing file does, with the `reading --notes-file P: <cause>` refusal and exit 1. It never runs as if the flag were left off, so no AI call is made (§13.4)
- [ ] Given any of these refusals, the run stops before any step: no stash, no gate, no AI call and no git mutation. On release not even the version confirmation shows, and on regenerate no source or target question is asked (§13.2, §13.4)

**Context**:
> §13.4: `--no-ai` with `--notes-file` is a usage error because the two flags give two answers to where the notes come from. Corrigendum 2026-10-19 on §13.4: an empty path names no file, fails as a missing file does, and never counts as the flag left off. §13.1: one file answers one version. P is the flag's argument, as the §13.1 usage lines name it.
>
> CLAUDE.md exit idioms: `0` success, `1` runtime failure or user abort, `2` usage errors only. The cmd layer prints a usage error as `mint: <message>` on stderr. §13.4 fixes the exit code of the two usage errors, not their wording.
>
> As built:
> - Release's usage errors come from `parseReleaseFlags` (`cmd/mint/flags.go`), for example `can't combine --set-version with a bump flag`. Its string flags register through `fs.StringVar`, under which an empty value and an absent flag both read as `""`.
> - Regenerate's usage errors come from `parseRegenerateFlags` (`cmd/mint/regenerate_flags.go`), for example `cannot combine a version with --all`, and from `validateRegenerateRequest` (`cmd/mint/regenerate_validate.go`). `regenerateRequest.SourceSet` tells an explicit `--source fresh` apart from no `--source`.
> - `runRegenerate` (`cmd/mint/main.go`) returns the usage code 2 for its own root-resolution and config-load failures, so an exit-1 refusal on regenerate cannot ride those paths.
> - Release's first gate is the version confirmation, before `--autostash` and preflight. Regenerate's source and target questions come before its preflight.
>
> CLAUDE.md seam 3: every byte of user output goes through the presenter, and the cmd layer's usage and error lines are the one exception.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §12.2 The choice; §13.1 What it is; §13.2 The file is outside everything mint does; §13.4 Refusals; Corrigenda (2026-10-19, §13.4)

## token-aware-diff-sizing-3-6

### Task 3-6: One non-AI body: remove `[release].fallback`

**Problem**: `[release].fallback` is a fixed notes body shared by `--no-ai` and `on_notes_failure = fallback`. Its one observed use was hand-written notes for a release the AI could not take, which is now `--notes-file`'s job (§14.1). Keeping it leaves two non-AI bodies. In a repo that sets the string, the floor's `c` (Phase 5) would differ from `--no-ai`.

**Solution**: Drop the key. Every route that ends without AI notes gives this release's commit-subject list, `git log --format=%s` over `{lastTag}..HEAD`, or the maintenance-release body when that list is empty. A `.mint.toml` still setting `[release].fallback` fails at load with a removed-key message that names `--notes-file`. `on_notes_failure` keeps both of its values. The key's config-metadata row and README row go. `on_notes_failure`'s descriptions and the setup guide's AI-transport paragraph name the commit-subject list as the one non-AI body.

**Outcome**: The commit-subject list is the one non-AI body. A repo still carrying the old key learns at load what replaced it, and nothing mint prints or documents mentions a fixed fallback string.

**Acceptance Criteria**:
- [ ] Given a `.mint.toml` setting `fallback` under `[release]`, each of `mint release`, `mint release regenerate <version>` and `mint commit` fails before any step runs with `invalid .mint.toml: release.fallback has been removed — every non-AI body is the commit-subject list; supply hand-written notes with --notes-file` (§14.2)
- [ ] Given `--no-ai` on a release with commits since the last tag, the body is the subject list `git log --format=%s {lastTag}..HEAD` returns. Given `on_notes_failure = "fallback"` and a failed AI call, the same list ships (§14.1)
- [ ] Given either route and a commit-subject list that comes back empty or whitespace-only, the maintenance-release body ships instead, so the tag body is never empty (§14.1)
- [ ] Given `on_notes_failure` set to `abort` or `fallback`, config loads as before. Any other value is still rejected at load as built (§14.1)
- [ ] `mint setup`'s config reference and the README's `[release]` table carry no row for `fallback` (§14.2, §18.1, §18.2)
- [ ] `on_notes_failure`'s description, as `mint setup`'s config reference renders it, and its README row both say `fallback` ships the commit-subject list, naming no fixed string (§14.2, §18.1)
- [ ] `mint setup`'s AI-transport paragraph names the commit-subject list as the one non-AI body, in place of "fall back to the commit-subject / fallback body" (§18.1)

**Do**:
- The key leaves the `[release]` decode shape (`releaseShape` in `internal/config/config.go`) and `config.MetadataRows()` (`internal/config/metadata.go`). Its removed-key message sits beside the `[hooks]` placement message in `unknownKeyMessage` (`internal/config/config.go`) (§14.2).
- The README row goes from its `[release]` table. The setup guide paragraph is the "The AI transport" text in `overview()` (`internal/setupguide/setupguide.go`) (§14.2, §18.1).

**Context**:
> §14.1: with one non-AI body, `c` can no longer differ from `--no-ai` in a repo with a fixed fallback string. `on_notes_failure` keeps its two values, and `fallback` still reads as falling back to a non-AI body. §14.2: no repo of the user's sets the key, so none needs a change before upgrading.
>
> As built:
> - `resolveFallbackBody` (`internal/notes/resolve.go`) is the shared selector that both `NoAIBody` (`internal/notes/noai.go`) and `ResolveFailure` call. It returns the first of these that applies:
>   1. the fixed `rel.Fallback` string, when set
>   2. `FallbackBody`'s `git log --format=%s {lastTag}..HEAD`
>   3. `StubBody()`, the maintenance-release body `Maintenance release — no notable source changes` (`internal/notes/degenerate.go`), when that list is empty or whitespace-only.
> - `validateOnNotesFailure` (`internal/config/config.go`) rejects any value outside abort and fallback.
> - Regenerate never reads `on_notes_failure` (§12.3).
> - The `on_notes_failure` metadata row reads `fallback uses the commit-subject / fixed-string body`, and its README row adds "(or `fallback` string)".
> - Phases 1 and 2 rewrite other parts of the setup guide's AI-transport paragraph: the lean default, the timeouts, the cap and chunking.
>
> CLAUDE.md config seam: the drift test fails until the metadata SoT and the decode shape biject over `(level, key)`. The README tripwire test checks only that every schema key name appears in the README.
>
> §18.3: code comments that state a contract this feature changes, the fixed fallback string among them, are rewritten in the same change. As built, the comments that state it sit in `internal/notes/noai.go` and `resolve.go`, and in `internal/config/config.go` (`onNotesFailureValues`, the `OnNotesFailure` and `Fallback` field docs) and `metadata.go`.
>
> Later phases extend the list. Phase 5 gives it as the floor's `c`, and computes regenerate's list over the version's own range without its bookkeeping commit. Phase 8 leaves out the subjects of the answers commit.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §14.1 The commit-subject list everywhere; §14.2 Removing the key; §12.3 Regenerate at the floor; §18.1 The setup guide; §18.2 The README; §18.3 CLAUDE.md and code comments
