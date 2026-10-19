# Phase 7: Per-verb exclusion lists and the exclusion check — 13 tasks

## token-aware-diff-sizing-7-1

### Task 7-1: Release notes exclude the `[release].diff_exclude` level

**Problem**: `diff_exclude` is one shared top-level list, and every verb's AI diff excludes it. The two verbs serve different readers: release notes are a public list of what a release holds, while a commit message is for the developers and agents reading that piece of work and should cover its tests and specs. With one shared list, hiding a test or spec folder from release notes also hid it from commit messages (§5.1).

**Solution**: Add `diff_exclude` under `[release]`. A release's notes diff excludes the built-in always-excludes, the shared list and the `[release]` list. The lists add, unlike `ai_command` and `timeout`, where a per-verb value overrides the shared one. Entries are git pathspec globs and git does the matching, and the Change Map is built over the same post-exclusion set. Regenerate's fresh source, single version or `--all`, applies the list as it stands. `mint commit`'s AI diff never applies it. The key takes the strict-config path, and the setup guide's advice, the README and CLAUDE.md stop treating `diff_exclude` as shared-only.

**Outcome**: An operator can keep a path out of release notes while commit messages still read it.

**Acceptance Criteria**:
- [ ] Given shared `diff_exclude = ["*.lock"]`, `[release].diff_exclude = ["specs/"]`, and a release range changing `app.lock`, `specs/export.md` and `src/export.go`, the notes call carries the diff of `src/export.go` alone, and its Change Map names neither `app.lock` nor `specs/export.md` (§5.1)
- [ ] Given `[release].diff_exclude = ["specs/"]` and no shared list, `specs/` paths stay out of the notes diff and Change Map. Given both lists, a path only the shared list matches stays out too: the `[release]` list adds to the shared list and never replaces it (§5.1)
- [ ] Given a range whose only changes match `[release].diff_exclude`, the release gets the maintenance-release body with no AI call (§5.1, §7.1)
- [ ] Given `mint release regenerate <version>` from a fresh source, or `mint release regenerate --all` from a fresh source, each version's notes diff and Change Map leave out the paths `[release].diff_exclude` matches (§5.1, §6.1)
- [ ] Given `[release].diff_exclude = ["specs/"]` and a staged change to `specs/export.md`, `mint commit`'s AI call carries that change's diff (§5.1)
- [ ] Given `[release].diff_exclude = "specs/"`, a string rather than an array, any command that loads config fails before any step runs with a friendly type error in the `invalid .mint.toml: …` shape that names the `[release]` key and its array-of-strings type (§5.1)
- [ ] `mint setup`'s config reference and the README's `[release]` table list `diff_exclude` at the `[release]` level with an empty default, described as globs that add to the shared list for release notes only (§5.1, §18.1, §18.2)
- [ ] `mint setup`'s "diff_exclude scope" etiquette section and its procedure step 4 send noise only release notes should skip, such as `.workflows/`, `.claude/`, agent directories and `docs/`, to `[release].diff_exclude`, and keep the shared list for what neither verb should read, such as lockfiles and build output (§18.1)
- [ ] Neither the README's shared engine keys section nor CLAUDE.md's config seam says `diff_exclude` is shared-only (§18.2, §18.3)

**Do**:
- The key takes the strict-config path CLAUDE.md's config seam names, in `internal/config`: a field in the `[release]` decode shape (`releaseShape`, `config.go`), a `typeErrorMessages` entry, and a `config.MetadataRows()` row (`metadata.go`). The README row goes in its `[release]` table (§5.1, §18.2).
- The release-notes diff and Change Map take their exclusions from the notes `Assembler`'s `ExcludeConfig` (`internal/notes/assemble.go`). `resolveBody` (`internal/engine/release.go`) builds it for the forward release, and `freshExcludeConfig` (`internal/engine/regenerate_fresh.go`) for regenerate's fresh source, single version and `--all` alike (§5.1).
- The setup guide's advice lives in `etiquetteSection()` and `procedure()` (`internal/setupguide/setupguide.go`); the CLAUDE.md line is "Non-negotiable seams" item 6 (§18.1, §18.3).

**Context**:
> §5.1: the shared list holds what neither verb should read, such as lockfiles and build output, and a per-verb list should not have to repeat it. As built, and still: entries at every level are git pathspec globs, git does the matching, and the Change Map is built over the same post-exclusion set as the diff. §6.1: regenerate applies the `[release]` lists as they stand, so answers given on forward releases carry into regenerated notes.
>
> As built:
> - `Assembler.excludePathspecs` turns each glob into a `:(exclude)<glob>` pathspec, after the built-in `:(exclude)CHANGELOG.md` and before the plain-mode `version_file` entry. The diff and the Change Map's two git calls (`changemap.go`) share those pathspecs.
> - Per-verb type errors read `release.ai_command must be a string`. The metadata SoT renders an empty collection as `[]`, and the drift test fails until the SoT and the decode shape biject over `(level, key)`.
> - The README tripwire (`internal/config/readme_tripwire_test.go`) does not catch a missing `[release].diff_exclude` row, because the README already names `diff_exclude` (§18.2). The `internal/initgen` template carries no per-key entries.
> - The README's shared engine keys section says "the rest are shared-only". `internal/setupguide/setupguide_test.go` pins the "diff_exclude scope" framing.
> - The comments on `Config.DiffExclude` and `Config.Commit` (`config.go`), on `ExcludeConfig`, and in `resolveBody` describe `diff_exclude` as top-level only; §18.3 keeps comments true to a changed contract in the same change.
>
> Task 7-2 adds the `[commit]` level and completes CLAUDE.md's description of the three levels.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §5.1 Three levels of `diff_exclude`; §6.1 When it runs; §7.1 Where the body comes from; §18.1 The setup guide; §18.2 The README; §18.3 CLAUDE.md and code comments

## token-aware-diff-sizing-7-2

### Task 7-2: Commit messages exclude the `[commit].diff_exclude` level

**Problem**: The two verbs serve different readers (§5.1). `mint commit`'s AI diff can exclude only what the shared list excludes, and release notes exclude that too, so a path that is noise in a commit message but belongs in release notes has no list of its own.

**Solution**: Add `diff_exclude` under `[commit]`. `mint commit`'s AI diff excludes the shared list plus the `[commit]` list, the lists adding. Release notes never apply `[commit].diff_exclude`. Commit's preflight still ignores every level, so a commit whose every file is excluded still commits, with its message written in the editor and the all-excluded note unchanged. The key takes the strict-config path, and CLAUDE.md's config seam describes the three levels, which add rather than override.

**Outcome**: Each verb's AI diff excludes the shared list plus its own level's list, and nothing from the other verb's level.

**Acceptance Criteria**:
- [ ] Given shared `diff_exclude = ["*.lock"]`, `[commit].diff_exclude = ["tmp/"]`, and staged changes to `app.lock`, `tmp/run.log` and `src/export.go`, `mint commit`'s AI call carries the diff of `src/export.go` alone, and accepting the message commits all three files (§5.1)
- [ ] Given `[commit].diff_exclude = ["tmp/"]`, `mint commit -A`, a tracked change to `src/export.go` and an untracked `tmp/new.log`, the AI call carries `src/export.go`'s change and not `tmp/new.log`, and accepting stages and commits both (§5.1)
- [ ] Given `[commit].diff_exclude = ["tmp/"]` and a release range changing `tmp/run.log` and `src/export.go`, the release's notes diff and Change Map carry both (§5.1)
- [ ] Given `[commit].diff_exclude = ["tmp/"]` and only `tmp/run.log` staged, `mint commit` makes no AI call, prints `all changes are excluded from the AI context — opening editor` with no AI output, and opens the editor; saving a message commits `tmp/run.log` (§5.1, §17.1)
- [ ] Given `[commit].diff_exclude = "tmp/"`, a string rather than an array, any command that loads config fails before any step runs with a friendly type error in the `invalid .mint.toml: …` shape that names the `[commit]` key and its array-of-strings type (§5.1)
- [ ] `mint setup`'s config reference and the README's `[commit]` table list `diff_exclude` at the `[commit]` level with an empty default, described as globs that add to the shared list for commit messages only (§5.1, §18.1, §18.2)
- [ ] CLAUDE.md's config seam says `diff_exclude` has shared, `[release]` and `[commit]` levels whose lists add rather than override (§18.3)

**Do**:
- The key takes the strict-config path, in `internal/config`: a field in the `[commit]` decode shape (`commitShape`, `config.go`), a `typeErrorMessages` entry, and a `config.MetadataRows()` row (`metadata.go`). The README row goes in its `[commit]` table (§5.1, §18.2).
- Commit's AI diff takes its exclusions through `sourceDiff` and the `excludePathspecs` tail (`internal/commit/generate.go`, `internal/commit/source.go`); the preflight probes stay exclusion-blind (`internal/commit/preflight.go`) (§5.1).

**Context**:
> §5.1: each verb's AI diff excludes the shared list plus its own level's list, and the lists add. As built, and still: commit's preflight ignores `diff_exclude` at every level, and a commit whose every file is excluded still commits, with its message written in the editor. §17.1: the all-excluded route into the editor carries no AI output, and its line is unchanged.
>
> As built:
> - The first generation and the review gate's `r` both assemble the diff through `GenerateWithContext`, which calls `sourceDiff(ctx, cfg.DiffExclude)`. Under `-A` the untracked files come from a `git ls-files --others` source carrying the same exclusion tail.
> - Commit carries no built-in excludes.
> - The comments on `Config.Commit` (`config.go`), the commit `Generator` and `excludePathspecs` describe commit as reading the shared globs only; §18.3 keeps comments true in the same change. The README tripwire does not catch a missing `[commit].diff_exclude` row (§18.2).
>
> Phase 2's commit size cap measures the composed prompt, which carries the post-exclusion diff.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §5.1 Three levels of `diff_exclude`; §17.1 The AI's output, verbatim; §18.1 The setup guide; §18.2 The README; §18.3 CLAUDE.md and code comments

## token-aware-diff-sizing-7-3

### Task 7-3: `.mint.toml` never reaches release notes

**Problem**: A change to mint's config is about how releases are made, not what a release holds (§5.3). As built, a `.mint.toml` edit in a release's range reaches its notes diff and Change Map like any source change. Phase 8 makes such edits routine: the exclusion check commits `.mint.toml` inside the release it answered for (§6.6), which would put a tooling line into that release's notes and into any later regenerate of that version.

**Solution**: `.mint.toml` joins `CHANGELOG.md` as a built-in, non-configurable always-exclude from every release-notes diff and Change Map, forward and regenerated alike, since exclusion is path-based. A range whose only change is `.mint.toml` has an empty post-exclusion diff and gets the maintenance-release body. `mint commit`'s AI diff has no built-in excludes, as built, so a config change still reaches its commit message. The plain-mode `version_file` exclusion is unchanged.

**Outcome**: Editing mint's config never shows up in release notes, and still shows up in commit messages.

**Acceptance Criteria**:
- [ ] Given no `diff_exclude` at any level and a release range changing `.mint.toml` and `src/export.go`, the notes call carries the diff of `src/export.go` alone, and the Change Map does not name `.mint.toml` (§5.3)
- [ ] Given a range whose only change is `.mint.toml`, the release gets the maintenance-release body with no AI call (§5.3, §7.1)
- [ ] Given `mint release regenerate <version>` or `mint release regenerate --all` from a fresh source, over a version whose range changed `.mint.toml`, that version's notes diff and Change Map leave it out (§5.3)
- [ ] Given a staged change to `.mint.toml`, `mint commit`'s AI call carries its diff (§5.3)
- [ ] Given `version_file = "VERSION"` with no `version_pattern`, a regenerated version whose range changed `VERSION` leaves it out of its notes diff; given a `version_pattern`, a changed version file stays in, as built (§5.3)
- [ ] Where the shared `diff_exclude` row in `mint setup`'s config reference names the built-in release-notes exclusions, it names `.mint.toml` beside `CHANGELOG.md` (§5.3, §18.1)

**Do**:
- `.mint.toml` joins the built-in always-exclude beside `changelogExcludePathspec` in `internal/notes/assemble.go`, whose pathspecs the diff and the Change Map share; git does the matching (§5.1, §5.3).
- The row is the shared `diff_exclude` entry in `config.MetadataRows()` (`internal/config/metadata.go`) (§18.1).

**Context**:
> §5.3: the exclusion check commits `.mint.toml` inside the release it answered for (§6.6, Phase 8). Without the exclude, that change would reach the release's notes and any later regenerate of that version. §6.2 and §6.5: paths the built-in always-excludes cover are never listed by the check (task 7-5) and never counted at a pause (task 7-8).
>
> As built:
> - `config.Load` reads `{root}/.mint.toml`. Commit's `excludePathspecs` (`internal/commit/source.go`) carries no built-ins.
> - `versionFileExcludePathspec` (`assemble.go`) excludes a plain-mode `version_file` and keeps an embedded-mode one.
> - The shared `diff_exclude` metadata row reads "on top of the built-in CHANGELOG.md exclusion"; §18.1 has a row whose description names behaviour this feature changes edited in the source of truth itself.
> - The comments on `changelogExcludePathspec`, `ExcludeConfig`, `AssembleDiff` and `AssembleRange` (`assemble.go`), on `BuildChangeMap` (`changemap.go`), on `Config.DiffExclude` (`config.go`) and in `resolveBody` name `CHANGELOG.md` as the one built-in exclude; §18.3 keeps comments true in the same change.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §5.1 Three levels of `diff_exclude`; §5.3 Built-in always-excludes; §6.2 The call; §6.5 At a terminal: one pause per suggestion; §6.6 Recording the answers; §7.1 Where the body comes from; §18.1 The setup guide; §18.3 CLAUDE.md and code comments

## token-aware-diff-sizing-7-4

### Task 7-4: `[release].diff_keep` config key

**Problem**: The exclusion check this phase adds asks the AI which changed paths look out of place in release notes, and the operator answers each suggestion (§6). A path the operator has decided belongs in the notes needs a place in the config that says so, or the check can raise it on every release (§5.2, §6.5).

**Solution**: Add `[release].diff_keep`: an array of globs in the same pathspec form as `diff_exclude`, default empty. A path it covers is never raised by the exclusion check. It does nothing else: it adds nothing to the diff and never overrides an exclusion, so a path both kept and excluded stays out of the notes. The key takes the strict-config path.

**Outcome**: The config can say which paths belong in release notes, and setting it changes no notes on its own.

**Acceptance Criteria**:
- [ ] Given `[release].diff_keep = ["docs/api/"]` and a release range changing `docs/api/export.md` and `src/export.go`, the notes call carries the same diff and Change Map as with no `diff_keep` set (§5.2)
- [ ] Given `[release].diff_keep = ["docs/api/"]` with `[release].diff_exclude = ["docs/"]`, or with shared `diff_exclude = ["docs/"]`, a change to `docs/api/export.md` stays out of the notes diff and Change Map (§5.2)
- [ ] Given `[release].diff_keep = "docs/api/"`, a string rather than an array, any command that loads config fails before any step runs with a friendly type error in the `invalid .mint.toml: …` shape that names the `[release]` key and its array-of-strings type (§5.2)
- [ ] Given `diff_keep` at the top level or under `[commit]`, loading fails with the as-built unknown-key message naming the key and where it sits (§5.2)
- [ ] `mint setup`'s config reference and the README's `[release]` table list `diff_keep` at the `[release]` level with an empty default, described as globs whose paths the exclusion check never raises, which add nothing to the diff and never override an exclusion (§5.2, §18.1, §18.2)

**Do**:
- The key takes the strict-config path, in `internal/config`: a field in the `[release]` decode shape (`releaseShape`, `config.go`), a `typeErrorMessages` entry, and a `config.MetadataRows()` row (`metadata.go`). The README row goes in its `[release]` table (§5.2, §18.2).

**Context**:
> §5.2: a path `diff_keep` covers is never raised by the exclusion check; the key does nothing else. Where the check reads it, later in this phase: the check is skipped when every changed path is covered by an exclusion or the keep list (§6.1, task 7-6); a kept path is not listed for the AI (§6.2, task 7-5); a suggestion survives only if it matches a changed path no exclusion or keep glob covers (§6.4, task 7-7); a pause counts and names the matched files on the keep list, because an Exclude answer takes them out too (§6.5, tasks 7-8, 7-9).
>
> Phase 8 appends Keep answers to `[release].diff_keep`. Regenerate applies the keep list as it stands but never runs the check (§6.1), so the key has no effect there. As built, the README tripwire catches a missing `diff_keep` (§18.2), and `internal/initgen`'s template carries no per-key entries.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §5.2 The keep list; §6 The Exclusion Check; §6.1 When it runs; §6.2 The call; §6.4 Reading the answer; §6.5 At a terminal: one pause per suggestion; §18.1 The setup guide; §18.2 The README

## token-aware-diff-sizing-7-5

### Task 7-5: Ask the AI which changed paths don't belong in the notes

**Problem**: Design files and test changes have shipped in release notes they were never meant for. Those slips happened on normal-sized releases, so no size rule catches them (§6). As built, nothing asks whether each changed path belongs in a release's notes before the notes are written.

**Solution**: Before a forward release's AI notes are written, make one AI call through the existing transport, with the release's `ai_command` and timeout. It carries no diff, only the release's changed-path list. Each entry is a changed path not covered by the built-in always-excludes, the plain-mode `version_file` exclusion, the shared or `[release]` `diff_exclude`, or `[release].diff_keep`, listed with its change status and the size of its diff, written in the decimal format. Big files are listed like any other path, with no separate verdict. The call asks for suggested globs, each with a short reason, or an explicit "none". It is the AI path's first step, before cache reuse and the size check. The notes line shows `Checking for files that don't belong in the notes…` while it runs. `r` at the review gate never re-runs it. The README documents the check with `mint release`.

**Outcome**: Every forward release whose notes the AI writes is first checked for changed paths that look out of place.

**Acceptance Criteria**:
- [ ] Given a forward release on the AI path whose range modifies `src/export.go` and adds `docs/design/export.md`, the AI tool receives one call before any notes call. Its prompt lists those two paths, each with its change status and the size of its diff, carries none of the diff's content, and asks for suggested globs, each with a short reason, or an explicit "none" (§6.2, §6.4, §7.2)
- [ ] Given shared `diff_exclude = ["*.lock"]`, `[release].diff_exclude = ["specs/"]`, `[release].diff_keep = ["docs/api/"]`, `version_file = "VERSION"` with no `version_pattern`, and a range that also changes `app.lock`, `specs/export.md`, `docs/api/export.md`, `.mint.toml`, `CHANGELOG.md` and `VERSION`, none of those six paths is listed (§5.2, §5.3, §6.2)
- [ ] Given changed files whose diffs are 3,100,000 bytes and 640 bytes, both are listed as ordinary entries, with sizes `3.1 MB` and `640 B` (§6.2, §16.1)
- [ ] Given `[release].ai_command = "claude -p --model opus"` and `[release].timeout = 300`, the check runs `claude` with `-p --model opus` under a 300-second per-attempt deadline. With neither key set at any level, it runs the lean shipped default under the 180-second release default (§2.2, §4.4, §6.2)
- [ ] Given the AI answers that nothing looks out of place, the release goes on to its notes, written from the same diff as with no check (§6.4)
- [ ] Given a dry run that cached the release's notes, the following real run makes the check call before it asks `Cached notes found for this diff — use them?` (§6.1, §7.2)
- [ ] Given a release that runs the check, in pretty mode the notes line shows `Checking for files that don't belong in the notes…` while the call runs, then narrates the notes as it would without the check. In plain mode the check prints as its own line before the notes' lines (§16.1, §16.2)
- [ ] Given AI notes at the review gate, choosing `r` makes no check call (§15.1)
- [ ] Given a run whose notes were written after halving, the lower-the-cap hint never suggests the check call's size (§10.3)
- [ ] The README's `release` section says that before the AI writes a release's notes, mint asks the AI which changed paths look like they don't belong in them (§18.2)

**Do**:
- The call goes through the release's AI transport as `aiTransport` (`internal/engine/release.go`) builds it: `aitransport.New` with `config.VerbRelease`, or the injected `deps.Transport` (§6.2).
- The check line is a presenter event in the existing model (`internal/presenter`): the pretty presenter renders it on the notes line, the plain presenter as its own line, and `presentertest.RecordingPresenter` records it. The engine never touches spinners or TTY state (§16.4).
- The README text goes in its `release` section (§18.2).

**Context**:
> §6: the check exists to catch slips that have already shipped. It runs on every release whose notes the AI writes, not only big ones. §7.2: the AI path runs the exclusion check (forward releases only), then cache reuse, then the size check, then one single-pass call or chunked generation; `--notes-file`, a first release, an empty post-exclusion diff and `--no-ai` take their bodies before it (§7.1). Task 7-6 covers the runs that skip the check.
>
> §2.2: the lean command serves every AI call mint makes, the check among them, and each call's own prompt carries its job. §4.1: no call is ever bigger than the cap; task 7-13 condenses the list. §15.1: `r` never re-runs the check, which asks about paths, not notes. CLAUDE.md seam 5: AI is reached behind a consumer-defined one-method interface at the point of use.
>
> As built:
> - Only the forward release uses `notes.Selector`. `SelectBodyWithReuse` (`internal/notes/select.go`) runs first release, then the post-exclusion diff assembled once, the degenerate check and `--no-ai`, then the reuse hook before the generator. Regenerate's fresh source calls the `Generator` directly (`freshGenerator`).
> - The Change Map's `git diff --name-status` call reads each path's change status (`parseNameStatus`, `internal/notes/changemap.go`).
> - The notes stage narrates as a blocking `StageStarted` with `generating release notes…`; the `Presenter.StageStarted` stream-discipline note forbids other narration between a blocking start and its completion. Plain's synthesised narration is byte-pure printable ASCII (`internal/presenter/bytepurity_test.go`), so its line cannot carry `…`; its wording is the implementer's.
> - Phase 2 built the decimal size format (task 2-8), and Phase 4 the lower-the-cap hint (task 4-6), which counts only diff-carrying calls.
>
> Task 7-7 reads the answer and pauses on each suggestion; task 7-12 handles a check that fails.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §2.2 One command for every call; §4.1 The key; §4.4 The release timeout; §5.2 The keep list; §5.3 Built-in always-excludes; §6 The Exclusion Check; §6.1 When it runs; §6.2 The call; §6.4 Reading the answer; §7.1 Where the body comes from; §7.2 The AI path, in order; §10.3 Telling the operator; §15.1 `r` on chunked notes; §16.1 Pretty mode: one notes line, updated; §16.2 Plain mode: a line per event; §16.4 How it is built; §18.2 The README; Corrigenda (2026-10-19, §6.2, §16.1)

## token-aware-diff-sizing-7-6

### Task 7-6: The check runs only on a forward release with paths to raise

**Problem**: The check costs an AI call and can pause the operator. Some runs leave it nothing to ask: no AI notes will be written, or no changed path is left undecided. On others its question does not fit: a regenerate would have it edit and commit config mid-run, and a commit message should describe the tests and specs in the work (§6.1).

**Solution**: The check runs only on `mint release`, on the AI path, with at least one changed path left to raise. It is skipped under `--no-ai`, with `--notes-file`, on a first release, on an empty post-exclusion diff, and when every changed path is already covered by an exclusion or the keep list. A skipped check makes no AI call and shows no check line. `mint release regenerate` never runs it, for any source, single version or `--all`, and `mint commit` never runs it.

**Outcome**: The check runs only where its answers can change a forward release's notes.

**Acceptance Criteria**:
- [ ] Given `mint release --no-ai`, no check call is made and no check line shows, and the body is the commit-subject list (§6.1, §14.1)
- [ ] Given `mint release --notes-file notes.md`, no check call is made and no check line shows, and the supplied body ships (§6.1, §13.3)
- [ ] Given a first release, no check call is made, and the body is `Initial release.` (§6.1, §7.1)
- [ ] Given a range whose only changes are to `CHANGELOG.md`, `.mint.toml` or paths a `diff_exclude` level matches, no check call is made, and the release gets the maintenance-release body (§6.1, §7.1)
- [ ] Given `[release].diff_keep = ["docs/api/"]`, shared `diff_exclude = ["*.lock"]`, and a range changing only `docs/api/export.md` and `app.lock`, no check call is made and no check line shows, and the notes call carries `docs/api/export.md`'s diff (§5.2, §6.1)
- [ ] Given `mint release regenerate <version>` or `mint release regenerate --all` from a fresh source, attended or under `-y`, no check call is made and no check line shows for any version (§6.1)
- [ ] Given `mint commit`, no check call is made (§6.1)

**Context**:
> §6.1, regenerate: a check there would have it edit and commit `.mint.toml` mid-run, and could pause `--all` at every version; a regenerate that writes only the provider release would otherwise need no clean tree and commit nothing. Regenerate applies the `[release]` exclusion and keep lists as they stand, so answers given on forward releases carry into regenerated notes. Accepted cost: noise in an old version that was never asked about is not flagged.
>
> §6.1, commit: a commit message should describe the tests and specs in the work, and a check there would be asking a different question, closer to "did you mean to stage this?". §13.3: a supplied body skips the exclusion check, the size cap, chunking and every AI call.
>
> As built, regenerate's fresh source builds its notes through `freshGenerator` (`internal/engine/regenerate_fresh.go`), and commit through its own generator (`internal/commit/generate.go`); neither goes through the forward release's `notes.Selector`.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §5.2 The keep list; §6.1 When it runs; §7.1 Where the body comes from; §7.2 The AI path, in order; §13.3 How a supplied body moves through the run; §14.1 The commit-subject list everywhere; §16.1 Pretty mode: one notes line, updated

## token-aware-diff-sizing-7-7

### Task 7-7: Pause on each usable suggestion

**Problem**: A suggestion helps only if the operator sees it and decides, since the AI never excludes anything itself (§6.4). Its answer can also miss: a glob can match nothing that changed, or only paths an exclusion or the keep list has already decided.

**Solution**: Read the answer as suggested globs, each with a short reason, or an explicit "none". Keep a suggestion only if it matches at least one uncovered changed path, matched the way git matches `diff_exclude`. At a terminal, give each surviving suggestion one pause through the presenter's gate seam, in the order the AI gave them. The pause shows the glob and the AI's reason, and offers two choices, each picked by its initial letter: `e` for Exclude and `k` for Keep. A bare Enter picks Keep, the default, so no file leaves the notes without the operator choosing it. The spinner suspends for the pauses. A Ctrl-C at a pause cancels the run. Stdin closing at a pause aborts the run before any tag, unwinding as any failure before the push does, and is never read as Keep. The README documents the pauses with `mint release`.

**Outcome**: At a terminal, the operator decides on each AI suggestion that could change the notes.

**Acceptance Criteria**:
- [ ] Given an attended release where the AI suggests `docs/design/`, with the reason "design documents, not shipped behaviour", then `test/fixtures/`, with the reason "test data", and both match changed paths nothing covers, mint pauses on `docs/design/` showing the glob and its reason, then on `test/fixtures/` (§6.5)
- [ ] Given a pause, it offers `e` for Exclude and `k` for Keep, and a bare Enter picks Keep (§6.5)
- [ ] Given the AI answers "none", no pause shows (§6.4)
- [ ] Given a suggestion `build/` that matches no changed path, and a suggestion that matches only paths a `diff_exclude` level or `[release].diff_keep` already covers, neither gets a pause, and the other suggestions still pause (§5.2, §6.4)
- [ ] Given a suggestion `*.md` on a release whose only uncovered Markdown change is `docs/guide/intro.md`, it gets a pause, since the git pathspec `*.md` matches that path (§6.4)
- [ ] Given every pause answered Keep, the notes are written from the same diff as with no check (§6.4, §6.5)
- [ ] Given the pauses in pretty mode, no spinner animates while a pause waits for its answer (§16.1)
- [ ] Given a Ctrl-C at a pause, the run is cancelled before any notes call, as a Ctrl-C during the notes stage is (§6.6)
- [ ] Given a pre_tag hook that left an artifact commit, and stdin closing at a pause, the run aborts with no notes call and no tag, and the unwind resets the branch to the starting commit, taking the artifact commit with it. The closed input is never taken as Keep (§6.6)
- [ ] The README's `release` section describes the pauses: one per suggestion, showing the glob and the AI's reason, with `e` to exclude its files from the release's notes and `k`, or Enter, to keep them (§18.2)

**Do**:
- Each pause goes through `Prompt(gate)` with a `presenter.Gate` (`internal/presenter/gate.go`) that declares `e` (Exclude) and `k` (Keep), with Keep its `Default` (§6.5, Corrigendum 2026-10-19).
- The spinner is suspended around the pauses as `cacheReuse` suspends it around its gate (`internal/engine/release.go`) (§16.1).
- The README text goes in its `release` section (§18.2).

**Context**:
> §6.4: an answer that is neither "none" nor yields one usable suggestion is a failed check (task 7-12). §6.5: the pause also shows how many changed files the glob matches, the total size of their diffs and any of them on the keep list, and a suggestion whose uncovered matches earlier answers already cover raises no pause (task 7-8). Exclude takes the glob's files out of this release's notes (task 7-9). Phase 8 writes the answers into `.mint.toml`; in this phase an answer governs only the release in hand. §6.6 holds answers until the last pause is answered, so closed input leaves none to record.
>
> §5.1: entries are git pathspec globs, and git does the matching. As built, each `diff_exclude` entry becomes a `:(exclude)<glob>` pathspec and mint does no Go-side glob matching (`internal/notes/assemble.go`). A git pathspec without magic matches a leading directory, and its `*` crosses `/`.
>
> As built:
> - `presenter.Choice` holds `y`, `n`, `e` and `r`. Each gate declares its own choice set and default, so the pause's `e` clashes with no other gate's `e`.
> - Under `-y`, `Prompt` returns the gate's `Default` and echoes the auto-accept; on a non-TTY without `-y` it returns `ErrNotInteractive`, already rendered; on closed input it returns `ErrInputClosed`, which is not rendered and the caller surfaces (CLAUDE.md seam 4).
> - Closed input at the cached-notes prompt (`cacheReuse`) returns through `resolveBody` to `surfaceAndUnwind`, which renders the notes stage's failure and unwinds what `made` tracks, a pre_tag artifact commit included.
> - Unattended runs get no pause (task 7-10), and a dry run asks nothing (task 7-11).

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §5.1 Three levels of `diff_exclude`; §5.2 The keep list; §6.4 Reading the answer; §6.5 At a terminal: one pause per suggestion; §6.6 Recording the answers; §16.1 Pretty mode: one notes line, updated; §18.2 The README; Corrigenda (2026-10-19, §6.5, §6.6)

## token-aware-diff-sizing-7-8

### Task 7-8: A pause counts the files the notes still carry

**Problem**: A glob and a reason do not tell the operator what Exclude would take out of the notes: how many files, how much of the release they hold, and whether any of them is a path the keep list says belongs (§6.5). Earlier answers in the same run also change what a later suggestion would take out, and a suggestion whose remaining matches are all decided needs no question.

**Solution**: Each pause shows how many changed files the glob matches and the total size of their diffs, each file measured as the check's list measures a path, the total written in the decimal format. The files counted are the changed paths this release's notes diff still carries at that pause:
- a path taken out by the built-in always-excludes, the plain-mode `version_file` exclusion, either `diff_exclude` level or an earlier Exclude answer in this run is not counted
- a path on the keep list is counted, and a glob an earlier Keep answer in this run gave counts as on the keep list.

The pause names the matched files on the keep list. A suggestion is skipped when earlier answers in this run already cover every uncovered changed path it matches, so kept files alone raise no pause.

**Outcome**: Each pause says exactly what an Exclude answer would take out of this release's notes, kept files included, and no pause asks about paths already decided.

**Acceptance Criteria**:
- [ ] Given a release changing `docs/design/a.md` (a 12,000-byte diff), `docs/design/b.md` (3,000 bytes), `docs/api.md` (2,000 bytes), `docs/old/c.md` and `docs/VERSION`, with `[release].diff_keep = ["docs/api.md"]`, `[release].diff_exclude = ["docs/old/"]` and `version_file = "docs/VERSION"` with no `version_pattern`, the pause on `docs/` shows 3 files totalling `17 KB` and names `docs/api.md` as on the keep list. Neither `docs/old/c.md` nor `docs/VERSION` is counted (§6.5)
- [ ] Given that release also changes `README.md`, and the suggestions are `docs/design/`, answered Exclude, then `*.md`, the pause on `*.md` counts `README.md` and `docs/api.md`, and neither `docs/design/` file (§6.5)
- [ ] Given that release also changes `docs/guide.md`, and the suggestions are `docs/design/`, answered Keep, then `docs/`, the pause on `docs/` counts `docs/design/a.md`, `docs/design/b.md`, `docs/guide.md` and `docs/api.md`, and names the two `docs/design/` files and `docs/api.md` as on the keep list (§6.5)
- [ ] Given the suggestions `docs/`, answered Exclude or Keep, then `docs/design/`, no pause shows for `docs/design/` (§6.5)
- [ ] Given the suggestions `docs/design/`, answered Keep, then `docs/design/a.md`, no pause shows for `docs/design/a.md`: its only match is now on the keep list (§5.2, §6.5)

**Context**:
> Corrigendum 2026-10-19 on §6.5, the count: the size is the total of the matched files' diff sizes, each measured as §6.2 measures a path; §6.3's condensed line in the same list, `vendor/: 4,312 files, 38 MB`, uses that measure. Exclude can take out only a file the notes still carry, so counting a file already out would misstate what the answer removes. §5.2 has keep never override an exclusion, which is why the pause shows kept files: an Exclude answer takes them out too (task 7-9).
>
> Corrigendum 2026-10-19 on §6.5, the skip: the kept files a pause shows are the warning that comes with uncovered matches before Exclude takes a kept file out, not a reason to ask about kept files alone. Corrigendum 2026-10-19 on §6.2: the notes diff never carries a plain-mode `version_file`, so it is not an uncovered changed path.
>
> §6.4: a suggestion matches the way git matches `diff_exclude` (task 7-7). §16.1: sizes are written in decimal units, so 17,000 bytes reads `17 KB` (Phase 2's format, task 2-8). Unattended runs and dry runs give no answers, so what they show of a suggestion's matches reflects the config alone (tasks 7-10, 7-11).

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §5.2 The keep list; §5.3 Built-in always-excludes; §6.2 The call; §6.4 Reading the answer; §6.5 At a terminal: one pause per suggestion; §16.1 Pretty mode: one notes line, updated; Corrigenda (2026-10-19, §6.2, §6.5)

## token-aware-diff-sizing-7-9

### Task 7-9: An Exclude answer leaves its files out of this release's notes

**Problem**: An Exclude answer matters only if every step that writes the release's notes then works without those files. A release over the cap only because of a committed 1.9 MB transcript should get an ordinary single-pass call once the operator excludes the transcript, not a chunked run that still reads it (§7.2).

**Solution**: An Exclude answer takes every file its glob matches out of this release's notes diff and Change Map, kept files included, since keep never overrides an exclusion. Every later step sees the diff the answers leave: the size check measures it, cache reuse keys on it, every notes call carries it, and an `r` at the review gate writes from it. When the answers leave no path in the diff, the release gets the maintenance-release body. A Keep answer leaves its files in the notes and never brings back a file an Exclude answer took out. In this phase the answers govern only the release in hand.

**Outcome**: The operator's Exclude answers shape this release's notes and every call that writes them.

**Acceptance Criteria**:
- [ ] Given an Exclude answer on `docs/design/`, no notes call carries a `docs/design/` diff and the Change Map names no `docs/design/` path, in a single-pass and a chunked run alike (§6.5)
- [ ] Given `[release].diff_keep = ["docs/design/spec.md"]` and an Exclude answer on `docs/design/`, `docs/design/spec.md` leaves the notes too (§5.2, §6.5)
- [ ] Given a release whose composed single-pass prompt is over the cap only because of a committed 1.9 MB transcript, an Exclude answer on the transcript's path brings the size check's measure under the cap, and the notes come from one single-pass call (§7.2)
- [ ] Given Exclude answers that leave no path in the diff, the release gets the maintenance-release body with no notes call (§6.6, §7.1)
- [ ] Given a dry run that cached its notes, a real run whose operator answers Exclude at a pause does not offer the cached notes, and writes fresh notes without those files. Given every pause answered Keep instead, the real run offers the cached notes as built (§7.2, §15.2)
- [ ] Given an Exclude answer on `docs/design/` and an `r` at the review gate, the `r`'s calls carry no `docs/design/` diff (§15.1)
- [ ] Given an Exclude answer on `docs/design/`, then a Keep answer on `*.md`, which matches `README.md` and `docs/design/a.md`, `README.md` stays in the notes and `docs/design/a.md` stays out (§5.2, §6.5)

**Do**:
- An answer's glob excludes as a `diff_exclude` glob does, as a git pathspec exclude through the notes `Assembler` (`internal/notes/assemble.go`), so the diff, the Change Map and every step after them see one post-answer set (§5.1, §6.5, §7.2).

**Context**:
> §6.5: on Exclude, the glob joins `[release].diff_exclude`, and this release's notes are written without those files. §7.2: exclusions the check records change the diff the later steps see; if they leave the diff empty, the maintenance-release body applies; the size check comes after the check, so it measures the diff the exclusions leave.
>
> §15.2: the notes cache keys on inputs: the post-exclusion diff, the version and the resolved instructions. §15.1: `r` re-runs the AI path from the size check onward and never re-runs the check, whose answers were recorded before the first attempt.
>
> As built:
> - `SelectBodyWithReuse` (`internal/notes/select.go`) assembles the post-exclusion diff once and hands it to both the reuse hook and the generator; the Change Map comes from the same `Assembler`.
> - `perRunRegenerator` (`internal/engine/release.go`) re-runs `generator.GenerateWithContext`, which re-assembles the diff through the generator's own `Assembler` on every `r`.
>
> Phase 8 writes Exclude answers into `[release].diff_exclude` and Keep answers into `[release].diff_keep`; until then an answer has no effect on a later release.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §5.1 Three levels of `diff_exclude`; §5.2 The keep list; §6.5 At a terminal: one pause per suggestion; §6.6 Recording the answers; §7.1 Where the body comes from; §7.2 The AI path, in order; §15.1 `r` on chunked notes; §15.2 The dry-run cache

## token-aware-diff-sizing-7-10

### Task 7-10: An unattended release prints the suggestions after the notes

**Problem**: An unattended run (`-y`, a non-TTY, an agent run) has nobody to pause for (§6.7). Excluding on the AI's word would let the AI take files out of the notes, which it never does (§6.4). Dropping the suggestions would hide them from whoever reads the output, often an agent about to act on it.

**Solution**: Unattended, the check pauses on nothing and excludes nothing: a flagged file stays in the diff and is read like any other. Nothing is written. After the notes, each surviving suggestion prints with its reason; what it matches, as the pause would show it (how many changed files, the total size of their diffs, and any of those files on the keep list); and the `[release].diff_exclude` line that would exclude it. That line is the key as an Exclude answer would leave it: the entries it holds now with the glob appended, or the glob alone where the key is unset, and never another suggestion's glob. They print whatever the notes' outcome: after AI notes, after the commit-subject list when `on_notes_failure = "fallback"` ships it, and with the next moves when the run stops at the floor. They are a follow-up after the notes, beside the lower-the-cap hint. The README documents the unattended output.

**Outcome**: An unattended run's output carries each suggestion with the config line that acts on it, and the run excludes nothing on the AI's word.

**Acceptance Criteria**:
- [ ] Given `mint release -y` where the AI suggests `docs/design/`, with the reason "design documents", matching 2 changed files whose diffs total 15,000 bytes, no pause and no `-y` auto-accept echo shows for it, and the notes calls carry the `docs/design/` diffs (§6.7)
- [ ] Given that run's notes are written, after the notes mint prints the suggestion's glob and reason, its 2 files totalling `15 KB`, and the `[release].diff_exclude` line that would exclude `docs/design/` (§6.5, §6.7)
- [ ] Given `.mint.toml` sets `[release].diff_exclude = ["specs/"]`, the printed line sets `diff_exclude` under `[release]` to `specs/` followed by `docs/design/`. Given the key unset, it sets `diff_exclude` to `docs/design/` alone. Given a second surviving suggestion, `tmp/`, in the same run, its line holds `specs/` followed by `tmp/` and never `docs/design/` (§6.6, §6.7)
- [ ] Given one of the matched files is on `[release].diff_keep`, the printed suggestion names it as on the keep list (§6.5, §6.7)
- [ ] Given `mint release -y`, the output is the same whether stdin is a terminal or not (§6.7)
- [ ] Given `on_notes_failure = "fallback"` and a notes call that fails, the suggestions print after the commit-subject list that ships (§6.7)
- [ ] Given `on_notes_failure = abort` and a notes call that fails, the suggestions print with the floor's next moves, and the run exits 1 (§6.7, §12.4, §16.3)
- [ ] Given a run whose notes were written after halving, the suggestions and the lower-the-cap hint both print after the notes (§16.3)
- [ ] Given any of these runs, `.mint.toml` is unchanged (§6.7)
- [ ] The README's `release` section says an unattended run pauses on nothing and excludes nothing, and prints each suggestion after the notes with the `[release].diff_exclude` line that would exclude it (§18.2)

**Do**:
- The suggestions reach the output through the presenter as payload-driven events (CLAUDE.md seam 3).
- The README text goes in its `release` section (§18.2).

**Context**:
> §6.7 and its corrigendum: what a printed suggestion shows of its matches is what the pause shows (§6.5), not a path list, which would run to thousands of lines for a folder glob such as `vendor/`. The suggestions print with the next moves at the floor because there the reader is about to act. §16.3: the follow-ups print after the notes, the lower-the-cap hint and these suggestions; on an unattended run that stops at the floor, the suggestions print with the floor's next moves instead. The failed-check warning (task 7-12) is not a follow-up.
>
> Corrigendum 2026-10-19 on §6.7: the printed line is the key as an Exclude answer would leave it, because §6.6 has an Exclude answer append to an existing key and never overwrite. TOML rejects a key defined twice in one table and the config loads strictly (`config.Load`), so a line holding the glob alone, pasted beside an existing key, would break the next run's load, while the full line replaces the existing one and keeps the earlier exclusions. Each suggestion's line is worked out against the config as loaded.
>
> CLAUDE.md seam 4: business logic always calls `Prompt(gate)` at a gate, and the `-y` auto-accept happens inside the presenter. §6.7 gives the unattended check no pause at all, and nothing is written, so an unattended run is not a pause auto-accepted as Keep.
>
> As built:
> - A real release's version confirmation fails a non-TTY run without `-y` with `ErrNotInteractive` before the notes stage, so a real release reaches the check unattended only under `-y`; a dry run reaches it without `-y` (task 7-11).
> - The release engine receives neither the `-y` decision nor the stdin-is-a-TTY signal; the presenter holds both. `mint commit` receives both, as `Yes` and `StdinInteractive` in its deps (`cmd/mint/main.go`, from `presenter.DetectStartupSignals`).
>
> Phase 4 built the lower-the-cap hint (task 4-6), Phase 5 the floor's next moves (task 5-3) and the `fallback` warning (task 5-6), and Phase 3 the commit-subject list as the one non-AI body (task 3-6).

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.4 Reading the answer; §6.5 At a terminal: one pause per suggestion; §6.7 Unattended and dry runs; §10.3 Telling the operator; §12.4 The ✗ line and the next moves; §16.3 Follow-ups; §18.2 The README; Corrigenda (2026-10-19, §6.7)

## token-aware-diff-sizing-7-11

### Task 7-11: A dry run shows the suggestions in its own mode, without asking

**Problem**: A dry run previews the real run and takes no action, so it must not pause or write (§6.7). A preview that hid the suggestions would not show what the real run will raise, and an agent previewing with `mint release --dry-run -y` acts on the `[release].diff_exclude` line the real run will print.

**Solution**: A dry run runs the check and asks nothing. Its suggestions follow the dry run's own mode, as the real run it previews would show them:
- at a terminal without `-y`, each surviving suggestion shows as its pause would show it, with no choice offered and no input read
- unattended (`-y` or non-TTY), they print after the notes as an unattended run's do, the `[release].diff_exclude` line included: after the previewed commit-subject list under `fallback`, and with the next moves when the dry run stops at the floor.

No answer is given, so no file leaves the dry run's notes. Nothing is written in either mode.

**Outcome**: A dry run shows the check's suggestions as the real run would present them, and changes nothing.

**Acceptance Criteria**:
- [ ] Given `mint release --dry-run` at a terminal without `-y` where the AI suggests `docs/design/`, mint shows the glob, its reason, the matched files' count and total diff size, and any of them on the keep list, as the pause would. It shows them before the notes are written, where the real run would pause, offers no choice and reads no input (§6.7, §7.2)
- [ ] Given that dry run, its notes carry the `docs/design/` diff (§6.4, §6.7)
- [ ] Given `mint release --dry-run -y`, or a dry run with stdin not a terminal, the suggestions print after the notes as an unattended run prints them, each with the `[release].diff_exclude` line that would exclude it (§6.7)
- [ ] Given an unattended dry run whose notes call fails, under `on_notes_failure = "fallback"` the suggestions print after the previewed commit-subject list, and under `abort` they print with the next moves (§6.7, §12.1)
- [ ] Given either kind of dry run, `.mint.toml` is unchanged (§6.7)

**Context**:
> §6.7 and its corrigendum: a dry run previews the real run, and a real `-y` or non-TTY run never pauses. The §12.1 corrigendum settles the dry run at the floor the same way, by the dry run's own mode. §7.2: the check is the AI path's first step, before the notes are written, so the real run an attended dry run previews raises its pauses there.
>
> As built, a dry run asks for nothing: it skips the version confirmation, reuses cached notes silently, and stops before the review gate (`finishDryRun`, `internal/engine/release.go`). A non-TTY dry run without `-y` therefore reaches the notes stage, where a gate would fail it with `ErrNotInteractive`.
>
> A dry run caches notes over a diff no answer has touched; the real run that follows re-runs the check, and its answers decide whether the cached notes still match (task 7-9). Task 7-10 built the unattended output, and Phase 5 the dry run's floor (tasks 5-7, 5-8).

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.4 Reading the answer; §6.5 At a terminal: one pause per suggestion; §6.7 Unattended and dry runs; §7.2 The AI path, in order; §12.1 What applies, by mode; Corrigenda (2026-10-19, §6.7, §12.1)

## token-aware-diff-sizing-7-12

### Task 7-12: A failed check warns and the release carries on

**Problem**: The check can fail: its call fails, times out or finds no binary, or its answer cannot be read. Without a word, "nothing looked out of place" and "the check never ran" would look the same, and that is how a design file slips into the notes unnoticed (§6.8). A failed check is still no reason to stop a release, since the notes call that follows runs the same command and meets any real AI problem through its own failure path.

**Solution**: The check counts as failed on a generation failure, a timeout, a missing binary, or an answer that is neither "none" nor yields one usable suggestion; task 7-13 adds the fifth case, a list that will not fit. The release then carries on with no suggestions, after one warning line, `couldn't check for files that don't belong in the notes`, with the AI's captured output verbatim beneath it where there is any. The warning prints when the check fails, before the notes are written; it is not a follow-up. The failure classifier never runs for the check. A Ctrl-C during the check cancels the run.

**Outcome**: A release whose check failed says so once and goes on to its notes.

**Acceptance Criteria**:
- [ ] Given the check's call exits non-zero after the transport's retry, with `Not logged in · Please run /login` on stdout, mint warns `couldn't check for files that don't belong in the notes` with that output verbatim beneath it, before the notes are written, and makes no classify call for the check (§6.8, §9.1)
- [ ] Given the check's call times out, or the AI tool is not installed, the same warning shows and the release carries on to its notes (§6.8)
- [ ] Given an answer that is neither "none" nor yields one usable suggestion, such as one whose every glob matches no uncovered changed path, the same warning shows with the AI's answer verbatim beneath it; no pause shows and no second check call is made (§6.4, §6.8)
- [ ] Given a failed check, the release carries on to its notes with no pause, and no suggestion prints after the notes, under `-y` included (§6.8)
- [ ] Given a check that failed because the AI tool is not installed, the notes call that follows fails the same way through the release's own notes failure path, with its own ✗ line, `AI tool not installed` (§6.8, §9.1)
- [ ] Given a Ctrl-C during the check's call, the run is cancelled before any notes call, and no warning about the check shows (§6.8)

**Do**:
- The warning goes through the presenter's `Warn`, with the AI's output in `Warning.Output`, where CLAUDE.md's error idioms carry subprocess diagnostics verbatim (§6.8).

**Context**:
> §6.8: any real AI problem surfaces at the notes call, which runs the same command through the normal failure path (§9); the failure classifier never runs for the check (§9.1). §16.3: the failed-check warning prints when the check fails, before the notes are written.
>
> CLAUDE.md seam 5: the transport owns validation and the single bad-content retry, and consumers never re-retry; `context.Canceled` propagates unchanged, since a cancel is not an AI failure.
>
> As built:
> - The transport's failures are `ai.ErrGenerationFailed`, whose `ai.GenerationError` carries the captured output, `ai.ErrTimeout` and `ai.ErrCommandMissing` (`internal/ai/transport.go`).
> - The `Presenter.StageStarted` stream-discipline note forbids a `Warn` between a blocking start and its completion unless wrapped in `SuspendSpinner`/`ResumeSpinner`.
> - Release's notes failures render their cause phrase and output through `notes.CauseText` and `notesFailureOutput` (`internal/engine/release.go`), with Phase 4's phrases.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.4 Reading the answer; §6.8 When the check fails; §9.1 The trigger; §12.4 The ✗ line and the next moves; §16.3 Follow-ups

## token-aware-diff-sizing-7-13

### Task 7-13: Condense the check's path list by folder

**Problem**: A release that adds a vendored tree or moves thousands of files has a changed-path list long enough to crowd the check's call, or to push it past the cap no call may exceed (§4.1, §6.2). Skipping the check would lose it on exactly the releases where flagging a folder matters most, and splitting the list across calls would split the judgment.

**Solution**: When the list would take more than a tenth of the cap (150,000 bytes at the default), condense it by folder, by the same rule as the Change Map: a long run of entries under one folder becomes one summary line, such as `vendor/: 4,312 files, 38 MB`, the largest runs first, until the list is under its share. The same input condenses identically on every run. A list under its share is sent exactly as before. A list that cannot get the check's call under the cap even condensed sends no call and is a failed check.

**Outcome**: The check runs on releases of any size, with a list the AI can read whole.

**Acceptance Criteria**:
- [ ] Given a list that takes at most a tenth of the cap, the check's call lists every path on its own line (§6.3)
- [ ] Given a release adding 4,312 files under `vendor/` whose diffs total 38,000,000 bytes, so that its list would take more than a tenth of the cap, the check's call carries `vendor/: 4,312 files, 38 MB` in place of those 4,312 entries (§6.2, §6.3)
- [ ] Given a 4,312-file run under `vendor/` and a 12-file run under `docs/`, where collapsing `vendor/` alone brings the list under its share, `vendor/` collapses and the `docs/` files stay listed one per line (§6.3)
- [ ] Given the same release on two runs, the check's list is byte-identical both times (§6.3)
- [ ] Given `max_prompt_bytes` set, the share is a tenth of the configured cap: a list sent whole under the default condenses under a cap small enough that the list takes more than a tenth of it (§6.3)
- [ ] Given the AI suggests `vendor/` from that condensed line, the suggestion survives, and its pause counts the 4,312 files and their `38 MB` (§6.2, §6.4, §6.5)
- [ ] Given a list that, condensed as far as it goes, still takes more than a tenth of the cap but leaves the check's call within the cap, the call is sent with the condensed list (§6.2)
- [ ] Given a cap so small that the check's call cannot fit under it even with the list condensed, no check call is sent; mint warns `couldn't check for files that don't belong in the notes` with no output beneath it, and the release carries on with no suggestions (§4.1, §6.2, §6.8)

**Context**:
> §6.2: the AI suggests folder-level globs in the main, so a folder-grouped list keeps what it needs. A list that cannot get under the cap even condensed counts as a failed check (§6.8), and no call mint makes is ever bigger than the cap (§4.1).
>
> §6.3, the rule both texts share:
> - A text condenses once it would take more than a tenth of the size cap, which is 150,000 bytes at the default.
> - Condensing replaces a long run of entries under one folder with one summary line for that folder, until the text is under its share.
> - A text under its share renders exactly as it does today.
> - The result must be deterministic for the same input and must collapse the largest runs first. Which folders collapse, and in what order, is otherwise the implementer's.
>
> Phase 2 built the rule for the Change Map (task 2-3, `internal/notes/changemap.go`); §6.3 sets one rule for both texts. §6.4: a suggestion survives when it matches an uncovered changed path, so a folder glob suggested from a summary line is matched against every path under that folder, not against the line. The summary line's size is written in the decimal format (task 2-8, §16.1). Task 7-12 built the failed-check warning.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §4.1 The key; §6.2 The call; §6.3 Condensing by folder; §6.4 Reading the answer; §6.5 At a terminal: one pause per suggestion; §6.8 When the check fails; §16.1 Pretty mode: one notes line, updated
