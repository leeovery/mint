---
phase: 3
phase_name: Project Prep — Hooks, Version-File Projection & Diff Exclusion
total: 11
---

## mint-release-tool-3-1 | approved

### Task mint-release-tool-3-1: Hook runner foundation (sh -c, repo root, MINT_* env, string|array)

**Problem**: Hooks are mint's escape valve for project-specific steps mint cannot know generically. They are configured as a table of shell commands keyed by lifecycle point (`[release.hooks]`), where each value is a **string or an array of strings**, run sequentially through a shell from the repo root, with a set of injected `MINT_*` environment variables. None of the three hook points (`preflight`/`pre_tag`/`post_release`) can run without a single shared mechanism that resolves a hook value to a sequence of commands, executes each via `sh -c` from the repo root with the env injected, and reports the first non-zero exit. Without this foundation each hook point would re-implement the same invocation logic.

**Solution**: A hook-runner component that takes a hook value (string or `[]string`) and a context (the resolved `MINT_*` env vars), normalises the value to a sequence of command entries, and runs each entry as `sh -c "<entry>"` through the Phase 1 `CommandRunner` from the repo root with the env injected. It returns success when all entries exit zero, or the failing entry's result on the first non-zero exit (stopping the sequence). This foundation is mechanism-only; the three hook *points* and their pre/post-PONR failure semantics are tasks 3-2/3-3/3-4.

**Solution note**: This task builds the shared invocation mechanism and the `MINT_*` env assembly only. It does NOT decide abort-vs-warn (that is per-point: 3-2/3-3 abort, 3-4 warns) and does NOT wire any specific lifecycle point into the spine. `MINT_DRY_RUN` is part of the injected set, but the dry-run skip-and-report behaviour is task 3-11.

**Outcome**: Given a hook value, the runner executes a single string as one `sh -c` command, or an array of strings as a sequence run in order, from the repo root, each with `MINT_NEW_VERSION`/`MINT_PREVIOUS_VERSION`/`MINT_VERSION_TAG`/`MINT_BUMP`/`MINT_DRY_RUN` injected. The first non-zero exit stops the sequence and returns that entry's failure (later entries do not run). An empty or absent hook value is a no-op (success, nothing run). `MINT_BUMP` is `patch`/`minor`/`major` for the bump flags and `explicit` when `--set-version` was used.

**Do**:
- In `internal/hooks` (new package), implement a runner, e.g. `Run(ctx, value any, env HookEnv) error` where `value` is the parsed TOML value (string or `[]string`) and `HookEnv` carries the injected variables.
- **Normalise the value** to a `[]string` of command entries: a single string → one entry; an array → entries in declared order. An absent/empty value (nil, empty string, empty array) → no entries → no-op success.
- **Execute each entry** through the Phase 1 `CommandRunner` as `sh -c "<entry>"` (command name `sh`, args `["-c", entry]`), so `&&`, pipes, env vars, and `./script.sh` invocations all work.
- **Run from the repo root** — set the working directory to the resolved repo root (`git rev-parse --show-toplevel`, task 1-4). If the `CommandRunner` seam needs a working-directory affordance, thread the repo root through it (do not `chdir` the process globally).
- **Inject the `MINT_*` env vars** for each entry, layered on top of the inherited process environment:
  - `MINT_NEW_VERSION` — the version being released (e.g. `1.4.0`).
  - `MINT_PREVIOUS_VERSION` — the prior latest version (e.g. `1.3.2`).
  - `MINT_VERSION_TAG` — the full tag with prefix (e.g. `v1.4.0`).
  - `MINT_BUMP` — `patch` / `minor` / `major` / `explicit` (`explicit` when `--set-version` was used).
  - `MINT_DRY_RUN` — `0` / `1`.
- **Sequencing / first-failure**: run entries in order; on the first non-zero exit, **stop** and return that entry's failure (carrying the failing command and its stderr/exit). Remaining entries are not run. This is the pre-PONR sequencing rule; whether the *caller* aborts or warns is decided by the hook point (3-2/3-3 abort, 3-4 warns).
- Assemble `HookEnv` from the run's computed version state (current/next version, tag, bump kind, dry-run flag) so each hook point reuses the same builder.
- All execution goes through the `CommandRunner`; tests use `FakeRunner` to script per-entry exit codes and assert the `sh -c` argv, the working directory, the injected env, and invocation order; `RecordingPresenter` is not required by this mechanism-only task (reporting is per-point).

**Acceptance Criteria**:
- [ ] A single string hook value runs as exactly one `sh -c "<string>"` command.
- [ ] An array hook value runs each entry as `sh -c "<entry>"` in declared order.
- [ ] All entries run via the `CommandRunner` with the working directory set to the repo root.
- [ ] Each entry is invoked with `MINT_NEW_VERSION`, `MINT_PREVIOUS_VERSION`, `MINT_VERSION_TAG`, `MINT_BUMP`, and `MINT_DRY_RUN` injected on top of the inherited environment.
- [ ] `MINT_BUMP` is `patch`/`minor`/`major` for the bump flags and `explicit` when `--set-version` was used.
- [ ] The first non-zero exit stops the sequence; later entries do not run; the failing entry's result is returned.
- [ ] An empty or absent hook value is a no-op (success, nothing run).
- [ ] All command execution goes through the `CommandRunner`/`FakeRunner`.

**Tests**:
- `"a single string hook runs as one sh -c command"`
- `"an array hook runs each entry via sh -c in order"`
- `"hooks run with the working directory set to the repo root"`
- `"each entry is invoked with the MINT_* env vars injected"`
- `"MINT_BUMP is explicit when --set-version was used"`
- `"the first non-zero exit stops the sequence and returns the failure"`
- `"an absent or empty hook value is a no-op"`

**Edge Cases**:
- String single command vs array run in order.
- First non-zero exit stops the sequence.
- `MINT_BUMP=explicit` for `--set-version`.
- Empty/absent hook → no-op.

**Context**:
> Mechanism: "Hooks are a config table of shell commands keyed by lifecycle point (`[release.hooks]` in `.mint.toml`)… Value is a string or an array of strings. Array entries run sequentially; the first non-zero exit aborts (for pre-PONR hooks)… Executed through a shell (`sh -c "<entry>"`) so `&&`, pipes, env vars, and `./script.sh` invocations all work. Run from the repo root." Invocation & context: "Each hook entry runs via `sh -c` from the repo root. mint injects: `MINT_NEW_VERSION` (the version being released), `MINT_PREVIOUS_VERSION` (the prior latest version), `MINT_VERSION_TAG` (the full tag with prefix), `MINT_BUMP` (`patch`/`minor`/`major`/`explicit` — `explicit` when `--set-version` was used), `MINT_DRY_RUN` (`0`/`1`). The set may grow as later stages need it." Config: hooks nest under `[release.hooks]`. Phase 3 reads only the config keys it needs with defaults; full schema validation is Phase 6.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Hooks → Mechanism, Invocation & context (injected env vars)".

## mint-release-tool-3-2 | approved

### Task mint-release-tool-3-2: preflight hook (runs after built-in gates, aborts on non-zero)

**Problem**: A project may need its own gates/validation beyond mint's built-in preflight checks. The optional `preflight` hook must run **after** mint's built-in preflight checks pass (Stage 2) and **before any mutation**, so project-specific validation gates the release in the same pre-mutation, fully-recoverable window. A non-zero exit must abort the whole release cleanly (nothing tagged, no damage). Without this wiring there is no project-extensible preflight.

**Solution**: Wire the `preflight` hook into the spine immediately after the built-in preflight gates (task 1-5/1-6) and before any mutation (before Stage 3 prep / Stage 5 record). Resolve the `[release.hooks].preflight` value and run it through the hook runner (task 3-1) with the `MINT_*` env. A non-zero exit aborts the release cleanly via the Presenter; an absent hook is skipped silently.

**Solution note**: This task wires the `preflight` point only. The runner mechanism, env assembly, string/array handling, and first-failure sequencing already exist (task 3-1). `pre_tag` (3-3) and `post_release` (3-4) are separate points. Because `preflight` runs before any mutation, a clean abort here has nothing to unwind (no commits/tag yet) — the surgical auto-unwind hardening is Phase 4 and not exercised by this point.

**Outcome**: After the built-in gates pass, mint runs the `preflight` hook (string or array) from the repo root with `MINT_*` injected. A zero exit lets the release proceed to project prep / notes. A non-zero exit (or, for an array, the first non-zero entry) aborts the release cleanly — no mutation has occurred, so the repo is untouched — and the abort is surfaced via the Presenter naming the failing hook. An absent `preflight` hook is skipped.

**Do**:
- In the release orchestrator (extending the Phase 1 spine), invoke the `preflight` hook **after** the built-in preflight gates pass (task 1-5 local gates + task 1-6 network gates) and **before** any mutation (before the `pre_tag` hook / Record).
- Read `[release.hooks].preflight` from config (string/array/absent); pass it to the hook runner (task 3-1) with the assembled `MINT_*` env.
- **Absent hook → skip** (no-op): the release proceeds as if no hook were configured.
- **Non-zero exit → clean abort**: stop the release before any mutation, surface the failure via the Presenter (e.g. "preflight hook failed: <command> (exit N)"), and exit non-zero. For an array value, the first non-zero entry aborts (task 3-1's first-failure rule).
- Confirm ordering against the spine: built-in gates → `preflight` hook → (later) `pre_tag` prep → notes → record. The `preflight` hook never runs before the built-in gates and never runs after a mutation.
- Test via `FakeRunner` + `RecordingPresenter`: assert the hook runs only after the built-in gates pass, that a non-zero exit aborts before any commit/tag, and that an absent hook is skipped.

**Acceptance Criteria**:
- [ ] The `preflight` hook runs after the built-in preflight gates pass and before any mutation.
- [ ] A zero exit lets the release proceed.
- [ ] A non-zero exit aborts the release cleanly (no mutation, repo untouched) and surfaces the failing hook via the Presenter.
- [ ] For an array value, the first non-zero entry aborts (no later entries run).
- [ ] An absent `preflight` hook is skipped silently.
- [ ] The hook runs from the repo root with the `MINT_*` env injected (via the task 3-1 runner).
- [ ] All execution goes through the `CommandRunner`/`FakeRunner`; the Presenter records the abort.

**Tests**:
- `"the preflight hook runs after the built-in gates pass"`
- `"a non-zero preflight hook aborts the release cleanly before any mutation"`
- `"an array preflight hook aborts on its first non-zero entry"`
- `"an absent preflight hook is skipped"`
- `"the preflight hook does not run if a built-in gate fails first"`

**Edge Cases**:
- Runs after built-in gates pass.
- Non-zero → clean abort pre-mutation.
- Absent hook → skipped.
- Array first failure aborts.

**Context**:
> Project preflight hook: "After mint's built-in preflight checks pass, the project's optional `preflight` hook runs (for project-specific gates/validation) — before any mutation. A non-zero exit aborts the release cleanly." Hook points: "`preflight` — runs after mint's built-in preflight checks (Stage 2), for project-specific gates/validation. Before any mutation." Failure behaviour: "`preflight` / `pre_tag` run before the tag is pushed → a non-zero exit aborts the whole release cleanly (no tag, no damage; mint auto-unwinds any local mutations)." Because `preflight` precedes all mutation, the abort has nothing to unwind; the surgical auto-unwind is Phase 4.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 2 → Project preflight hook", "Hooks → Hook points, Failure behaviour".

## mint-release-tool-3-3 | approved

### Task mint-release-tool-3-3: pre_tag hook execution & artifact commit (commit-interplay rule)

**Problem**: Projects often need to build/generate artifacts (e.g. a knowledge bundle) before tagging. The `pre_tag` hook runs at Stage 3 project prep and may dirty the tree; mint must then commit **whatever the hook left dirty** as its **own** commit (`chore(release): pre-tag artifacts for {tag}`), kept separate from the release-bookkeeping commit because it is project content, not bookkeeping. The interplay rule has subtle cases: a clean tree → no commit; a hook that makes its own commit and hands back a clean tree → mint commits nothing; gitignored outputs don't count as dirty. A non-zero exit aborts cleanly. Without this, hook artifacts never reach the tag, or mint tags a dirty tree.

**Solution**: Wire the `pre_tag` hook into the spine at Stage 3 (after the `preflight` hook, before notes). Run it through the hook runner (task 3-1). After it returns successfully, check whether the tree is dirty (tracked changes and/or non-ignored untracked files, per the Phase 1 porcelain convention); if dirty, stage and commit as `chore(release): pre-tag artifacts for {tag}` — a commit distinct from the bookkeeping commit. If the tree is clean (hook built nothing, or made its own commit), mint commits nothing. A non-zero hook exit aborts the release cleanly.

**Solution note**: This task lands the `pre_tag` point and its artifact commit (commit graph step 1). The release-bookkeeping commit (changelog + version-file fold) is the separate step 2 — extended in tasks 3-5/3-6/3-7. The up-to-two-commit assembly and tag-points-at-bookkeeping ordering is task 3-8. Notes generate at the post-hook HEAD (Phase 2's diff base already targets `last_tag..HEAD`); `diff_exclude` filtering of hook artifacts is tasks 3-9/3-10.

**Outcome**: When a `pre_tag` hook dirties the tree, mint creates exactly one **own** commit `chore(release): pre-tag artifacts for {tag}` capturing the dirtied tracked + non-ignored-untracked changes. When the hook leaves the tree clean — because it built nothing, or because it made its own commit and handed back a clean tree — mint creates no pre-tag commit. Gitignored build outputs do not count as dirt. A non-zero hook exit aborts the release cleanly before any tag/push.

**Do**:
- In the orchestrator, invoke the `pre_tag` hook at **Stage 3** — after the `preflight` hook (task 3-2), before notes generation (Stage 4). Read `[release.hooks].pre_tag` (string/array/absent) and run via the hook runner (task 3-1) with the `MINT_*` env.
- **Absent hook → skip** (no prep, no commit).
- **Non-zero exit → clean abort**: stop before any tag/push, surface via the Presenter. (For an array, the first non-zero entry aborts — task 3-1.) Any local mutation is recoverable (surgical unwind is Phase 4); at this point only a possible hook-artifact commit could exist, and on a hook *failure* mint has not yet committed.
- **After a successful hook**, evaluate tree dirtiness using the Phase 1 clean-tree convention (`git status --porcelain`): tracked changes and/or **non-ignored** untracked files count as dirty; gitignored files are exempt (build outputs don't trip it).
  - **Dirty → commit**: `git add` the dirtied paths (the artifacts) and `git commit` with subject `chore(release): pre-tag artifacts for {tag}` via the `CommandRunner`. This is the hook-artifact commit — its **own** commit, semantically distinct from the bookkeeping commit.
  - **Clean → no commit**: the hook built nothing, or made its own commit and handed back a clean tree → mint commits nothing (the "commit only if changed" behaviour falls out for free, and a custom-commit hook is respected).
- Keep this commit **separate** from the release-bookkeeping commit (`{commit_prefix} Release {tag}`): the artifact commit is project content; the bookkeeping commit is release bookkeeping. Do not fold them.
- Test via `FakeRunner` + `RecordingPresenter`: a hook that dirties the tree → one `chore(release): pre-tag artifacts for {tag}` commit; a clean tree → no commit; a hook that makes its own commit + clean tree → no mint commit; gitignored-only changes → no commit; a non-zero hook → clean abort.

**Acceptance Criteria**:
- [ ] The `pre_tag` hook runs at Stage 3, after the `preflight` hook and before notes generation.
- [ ] A hook that dirties the tree → mint creates one commit `chore(release): pre-tag artifacts for {tag}` (its own commit, separate from bookkeeping).
- [ ] A clean tree after the hook → mint creates no pre-tag commit.
- [ ] A hook that makes its own commit and hands back a clean tree → mint commits nothing.
- [ ] Gitignored outputs do not count as dirty (no commit triggered by them).
- [ ] A non-zero hook exit aborts the release cleanly before any tag/push.
- [ ] The artifact commit is distinct from (not folded into) the bookkeeping commit.
- [ ] All git operations go through the `CommandRunner`/`FakeRunner`.

**Tests**:
- `"a pre_tag hook that dirties the tree produces its own chore(release) artifact commit"`
- `"a clean tree after the hook produces no pre-tag commit"`
- `"a hook that makes its own commit and leaves a clean tree produces no mint commit"`
- `"gitignored outputs do not count as dirty"`
- `"a non-zero pre_tag hook aborts the release cleanly"`
- `"the artifact commit is separate from the bookkeeping commit"`

**Edge Cases**:
- Hook dirties tree → own commit.
- Clean tree → no commit.
- Hook makes its own commit + hands back clean → nothing committed.
- Non-zero exit → clean abort.
- Gitignored outputs don't count as dirty.

**Context**:
> Hook points: "`pre_tag` — Stage 3 project prep (build/generate artifacts, e.g. a knowledge bundle). Dirties the tree → mint commits per the interplay rule." Commit interplay: "After a `pre_tag` hook runs, mint commits whatever it left dirty (message `chore(release): pre-tag artifacts for {tag}`). Consequences: Simple hooks never touch git — they just build; mint handles the commit. 'Commit only if the bundle changed' falls out for free: changed → tree dirty → mint commits; unchanged → tree clean → nothing committed. A hook that wants a custom commit can do its own and hand mint back a clean tree — mint then sees nothing to commit. Either way, mint never tags a dirty tree." Commit graph: "Hook artifacts (only if a `pre_tag` hook dirtied the tree) → their own commit: `chore(release): pre-tag artifacts for {tag}`. Kept separate because it's project content … semantically distinct from release bookkeeping." Clean-tree convention (Phase 1): `git status --porcelain`; gitignored files exempt; non-ignored untracked files count. Notes generate at the post-hook HEAD (Stage 4 diff base).

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Hooks → Hook points, Commit interplay (`pre_tag`)", "Stage 5 → Commit graph".

## mint-release-tool-3-4 | approved

### Task mint-release-tool-3-4: post_release hook (warn-only on failure)

**Problem**: After the provider release is created (Stage 7), a project may need follow-ups (notifications, tap `repository_dispatch`, etc.). The `post_release` hook runs after publish, **past the point of no return**, so unlike the pre-PONR hooks it **cannot abort** — a non-zero exit must **warn only** ("post_release hook failed; tag is already published"), the same principle as a failed `gh release create`. Without this, there is no post-publish project extensibility, or a failed follow-up would wrongly try to unwind a published release.

**Solution**: Wire the `post_release` hook into the spine at Stage 7, after the provider release is created. Run it through the hook runner (task 3-1) with the `MINT_*` env. On any non-zero exit, **warn via the Presenter and continue** — never abort, never unwind (the tag is already public). An absent hook is skipped.

**Solution note**: This task lands the `post_release` point and its warn-only failure semantics. The publish step itself (Stage 7 `Publisher.CreateRelease`) already exists (Phase 1, task 1-8/1-11). `post_release` runs after publish. Note the array semantics nuance: post-PONR, the runner's first-non-zero-stops-the-sequence rule still stops the array, but the *outcome* is a warning, not an abort.

**Outcome**: After a successful publish, mint runs the `post_release` hook (string or array) from the repo root with `MINT_*` injected. A zero exit completes the run normally. A non-zero exit (or first non-zero array entry) produces a **warning** via the Presenter ("post_release hook failed; tag is already published") and the run still completes successfully (the release is already live; nothing is unwound). An absent hook is skipped.

**Do**:
- In the orchestrator, invoke the `post_release` hook at **Stage 7**, **after** the provider release create (task 1-8's `Publisher.CreateRelease`), i.e. after the point of no return. Read `[release.hooks].post_release` (string/array/absent) and run via the hook runner (task 3-1) with the `MINT_*` env.
- **Absent hook → skip**.
- **Non-zero exit → warn only**: surface a Presenter **warning** ("post_release hook failed; tag is already published") and **do not abort, do not unwind**. The run still ends successfully — the release is live. This mirrors the warn-only behaviour of a failed `gh release create` after a successful push.
- **Array semantics post-PONR**: the runner stops at the first non-zero entry (task 3-1); post-PONR, mint reports that failure as a warning rather than aborting. Document that the stop-on-first-failure sequencing is the same; only the consequence (warn vs abort) differs by point.
- Ensure `post_release` is the only hook point whose failure is non-fatal; `preflight`/`pre_tag` (3-2/3-3) abort.
- Test via `FakeRunner` + `RecordingPresenter`: a non-zero `post_release` hook → a recorded warning and a successful run; the hook runs after publish; an absent hook is skipped.

**Acceptance Criteria**:
- [ ] The `post_release` hook runs at Stage 7, after the provider release is created.
- [ ] A zero exit completes the run normally.
- [ ] A non-zero exit warns ("post_release hook failed; tag is already published") and does NOT abort or unwind; the run still completes.
- [ ] For an array value, the first non-zero entry stops the sequence and is reported as a warning (not an abort).
- [ ] An absent `post_release` hook is skipped.
- [ ] The hook runs from the repo root with the `MINT_*` env injected.
- [ ] All execution goes through the `CommandRunner`/`FakeRunner`; the warning is recorded via the Presenter.

**Tests**:
- `"the post_release hook runs after the provider release is created"`
- `"a non-zero post_release hook warns only and the run still completes"`
- `"a non-zero post_release hook does not unwind the published release"`
- `"an array post_release hook stops at the first failure and warns"`
- `"an absent post_release hook is skipped"`

**Edge Cases**:
- Non-zero → warn only.
- Runs after publish.
- Absent → skipped.
- Array continues semantics post-PONR (stop-on-first-failure, but warn not abort).

**Context**:
> Hook points: "`post_release` — Stage 7 follow-ups after the provider release (notifications, tap `repository_dispatch`, etc.)." Failure behaviour: "`post_release` runs after the tag is live → it cannot abort; a non-zero exit just warns ('post_release hook failed; tag is already published'). Same principle as a failed `gh release create`." Failure model: "`post_release` hook fails → Warn only — after the point of no return, the tag is already published." Post-release tap/formula update is itself a `post_release` hook (`repository_dispatch`), already supported by the hook system.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Hooks → Hook points, Failure behaviour", "Stages 6–7 → Failure model, Post-release: tap / formula update".

## mint-release-tool-3-5 | approved

### Task mint-release-tool-3-5: Version-file projection — plain mode (whole file is the version)

**Problem**: Some projects need the new version written *into the repo* as a write-only mirror (e.g. `release.txt` whose whole contents are the version). In **plain mode** (`version_file` set, no `version_pattern`), mint must write the new version as the entire file contents during the Record stage (Stage 5). The file is always a derived mirror, never a source of truth. It must create the file if absent, and be a **no-op** when the file already holds the target version (no empty commit). Without this, the old legacy `file` strategy has no replacement.

**Solution**: A plain-mode version-file writer in Record: when `version_file` is configured and `version_pattern` is **not**, write the new version (e.g. `1.4.0`) as the whole file contents through the filesystem, creating the file if absent. If the file already holds exactly the target version, do nothing (so the bookkeeping commit can no-op). This is one half of the projection; embedded mode is task 3-6, and folding into the bookkeeping commit is task 3-7.

**Solution note**: This task implements plain mode only (whole file = the version). Embedded mode (`version_pattern`) is task 3-6. The version-file change is **folded into the single bookkeeping commit** with the changelog (task 3-7) — this task produces the file write + no-op detection, not the commit. Decide and document whether the projected value carries the `tag_prefix` — default to the bare semver `X.Y.Z` (the version mirror, not the tag) unless a clear reason to include the prefix; note the choice so embedded mode (task 3-6) and tests align.

**Outcome**: With `version_file` set and no `version_pattern`, Record writes the new version as the whole contents of the file (creating it if absent). When the file already contains exactly the target version, the write is a no-op (no net change → no empty commit downstream). The file is treated strictly as a write-only mirror — mint never reads it as a version source (the tag is truth, Stage 1).

**Do**:
- In `internal/record` (extending the Phase 1 Record area), implement the plain-mode projection: when `[release].version_file` is set and `[release].version_pattern` is **absent**, write the new version as the entire file contents.
- **Create if absent**: if `version_file` does not exist, create it at the configured path relative to the repo root.
- **Value written**: the new version string (bare `X.Y.Z` by default — document the decision). Decide trailing-newline handling explicitly and consistently (recommend a single trailing newline; document and test it).
- **No-op detection**: if the file already holds exactly the target contents (same version, same newline convention), make **no change** so the downstream bookkeeping commit (task 3-7) sees nothing to stage for the version file. This is the no-op-safety contributor for plain mode.
- Do **not** create the commit here — the version-file change is staged and folded into the single bookkeeping commit in task 3-7. This task is the file projection + no-op detection.
- The file is a write-only mirror; add a code note that it is never read as a version source (Stage 1: tag-as-truth).
- Test the writer directly (filesystem + the projection function): file absent → created with the target version; file holding an old version → overwritten with the new version; file already at target → no change (no-op); trailing-newline handling is consistent.

**Acceptance Criteria**:
- [ ] With `version_file` set and no `version_pattern`, the whole file is written as the new version.
- [ ] An absent `version_file` is created with the target version.
- [ ] A file holding an older version is overwritten with the new version.
- [ ] A file already holding exactly the target version is a no-op (no net change).
- [ ] Trailing-newline handling is explicit and consistent (documented + tested).
- [ ] The version file is treated as a write-only mirror (never read as a version source).
- [ ] The commit itself is NOT created here (folded into the bookkeeping commit, task 3-7).

**Tests**:
- `"plain mode writes the whole file as the new version"`
- `"an absent version_file is created with the target version"`
- `"a file holding an old version is overwritten"`
- `"a file already at the target version is a no-op"`
- `"trailing newline handling is consistent"`

**Edge Cases**:
- File absent → created.
- File already holds target version → no-op (no empty commit).
- Trailing-newline handling.

**Context**:
> Optional version-file projection: "When a project needs the version written into the repo, mint mirrors the new version into a file during the Record stage (Stage 5). The file is always a write-only mirror kept in sync — never a source of truth. `version_file` — path to write; omit = tag-only (no projection). `version_pattern` — e.g. `RELEASE_VERSION="{version}"`; omit = the whole file is the version (plain mode)." Legacy strategy mapping: "old `file` (plain `release.txt`) → `version_file = "release.txt"`, no pattern." Stage 5 Version-file projection: "When `version_file` is configured, mint writes the new version into it (per `version_pattern`, or the whole file in plain mode)." No-op safety: "No empty commits … if the version file already holds the target version, mint skips that commit." Source of truth (Stage 1): the version file is derived state; the tag is truth. The bookkeeping commit folds changelog + version-file (task 3-7).

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 1 → Optional version-file projection, Legacy strategy mapping", "Stage 5 → Version-file projection, No-op safety".

## mint-release-tool-3-6 | approved

### Task mint-release-tool-3-6: Version-file projection — embedded mode (version_pattern)

**Problem**: Other projects embed the version inside a real source file (e.g. `RELEASE_VERSION="1.4.0"` in `main.go`). In **embedded mode** (`version_file` + `version_pattern`), mint must substitute the new version into the file by replacing the configured pattern. Critically: a pattern that matches **nothing** must **abort during Record, before the tag** (fail-loud — never silently skip the version write), and a pattern matching **multiple** times must replace **all** occurrences (legacy `sed`-replace semantics) to keep the file internally consistent. Without this, the legacy `embedded` strategy has no replacement and stale versions can linger.

**Solution**: An embedded-mode version-file writer in Record: when `version_file` and `version_pattern` are both set, expand the pattern's `{version}` placeholder to derive a match expression, find all occurrences in the file, and replace each with the new-version rendering. Zero matches → abort the release before the tag (fail-loud, same family as a notes failure). Multiple matches → replace all. No change needed (file already at target) → no-op. Folding into the bookkeeping commit is task 3-7.

**Solution note**: This task implements embedded mode only. Plain mode is task 3-5. The version-file change folds into the single bookkeeping commit (task 3-7) — this task produces the substitution + mismatch-abort + multi-match-replace + no-op behaviour, not the commit. The `{version}` placeholder in `version_pattern` is the configured marker (e.g. `RELEASE_VERSION="{version}"`); mint matches the pattern with the placeholder treated as "the version slot" and writes the new version into it. Align the `{version}` rendering (bare `X.Y.Z`) with task 3-5's choice.

**Outcome**: With `version_file` + `version_pattern`, Record replaces every occurrence of the pattern (the old version slot) with the new version. A pattern that matches nothing aborts the release **before the tag**, fail-loud, with a clear message (the version write is never silently skipped). Multiple matches are all replaced (no stale copies left). A file already at the target version (pattern present, value already the new version) is a no-op (no net change). The bookkeeping commit (task 3-7) then folds this change in.

**Do**:
- In `internal/record`, implement embedded-mode projection: when `[release].version_file` and `[release].version_pattern` are both set, perform pattern substitution in the file.
- **Pattern semantics**: `version_pattern` contains a `{version}` placeholder (e.g. `RELEASE_VERSION="{version}"`). Build a matcher from the pattern where `{version}` stands for the version slot (matching any current version value there), and a replacement where `{version}` is the new version. Treat the non-`{version}` parts of the pattern as literal text to anchor the match. Render `{version}` as bare `X.Y.Z` (consistent with task 3-5).
- **Mismatch (zero matches) → abort before the tag**: if the pattern matches nothing in the file, **fail loud during Record** — abort the release before any tag is created, surfacing a clear message (e.g. "version_pattern matched nothing in `<file>`"). Never silently skip the version write. This is the same fail-loud family as a notes failure (Record precedes the tag).
- **Multiple matches → replace all**: replace **every** occurrence of the version slot with the new version (carrying forward legacy `sed`-replace semantics), so the file is internally consistent with no stale old-version copies.
- **No-op**: if the file already contains the pattern with the new version in every slot (substitution yields identical contents), make no change (contributes to no-op safety; the commit no-ops downstream).
- Do **not** create the commit here — the change is staged and folded into the single bookkeeping commit (task 3-7).
- Test the substitution directly: single match replaced; multiple matches all replaced; zero matches → abort-before-tag error; already-at-target → no-op; `{version}` placeholder correctly substituted into the replacement.

**Acceptance Criteria**:
- [ ] With `version_file` + `version_pattern`, the pattern's version slot is replaced with the new version.
- [ ] A pattern matching nothing aborts the release before the tag (fail-loud), never silently skipping.
- [ ] Multiple matches are all replaced (no stale old-version copies remain).
- [ ] A file already at the target version (every slot already the new version) is a no-op.
- [ ] The `{version}` placeholder is correctly substituted in both the match and the replacement.
- [ ] The commit itself is NOT created here (folded into the bookkeeping commit, task 3-7).

**Tests**:
- `"embedded mode replaces the version_pattern slot with the new version"`
- `"a version_pattern matching nothing aborts before the tag"`
- `"multiple matches are all replaced"`
- `"a file already at the target version is a no-op"`
- `"the {version} placeholder is substituted in the replacement"`

**Edge Cases**:
- Pattern matches nothing → abort before tag.
- Multiple matches → all replaced.
- Already at target version → no-op.
- `{version}` placeholder substitution.

**Context**:
> Optional version-file projection: "`version_pattern` — e.g. `RELEASE_VERSION="{version}"`; omit = the whole file is the version (plain mode)." Legacy strategy mapping: "old `embedded` (sed-replace into a source file) → `version_file` + `version_pattern = 'RELEASE_VERSION="{version}"'`." Stage 5 Version-file projection: "`version_pattern` mismatch (configured pattern matches nothing in the file) → abort during Record, before the tag (fail-loud, same family as a notes failure). Never silently skip the version write. `version_pattern` multiple matches → mint replaces all occurrences (carrying forward the legacy `sed`-replace semantics), keeping the file internally consistent rather than leaving stale copies of the old version." The change folds into the single bookkeeping commit (task 3-7); no-op when already at target (No-op safety).

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 1 → Optional version-file projection, Legacy strategy mapping", "Stage 5 → Version-file projection".

## mint-release-tool-3-7 | approved

### Task mint-release-tool-3-7: Bookkeeping commit folds changelog + version-file projection

**Problem**: The legacy script made three commits per release — needlessly noisy. mint folds the CHANGELOG entry **and** the version-file projection into **one** release-bookkeeping commit (`{commit_prefix} Release {tag}`). Phase 1 built this commit for the changelog alone; now the version-file projection (tasks 3-5/3-6) must be staged into the **same** commit, with no-op safety across both inputs: if neither the changelog nor the version file nets a change, no commit is made; if either changes, exactly one commit is made. Without this, the version file either gets its own extra commit or is never committed.

**Solution**: Extend the Phase 1 bookkeeping-commit step so that, before committing, it runs the version-file projection (plain mode task 3-5 or embedded mode task 3-6, per config) **and** writes the changelog (Phase 1), stages **both** changes, and creates the single `{commit_prefix} Release {tag}` commit. No-op safety spans both: commit only if at least one of the two produced a net change; otherwise skip the commit entirely.

**Solution note**: This task folds the version-file write into the existing bookkeeping commit. It depends on tasks 3-5 and 3-6 for the file writes (and on Phase 1 task 1-9 for the changelog write + commit mechanics). The embedded-mode pattern-mismatch abort (task 3-6) fires **before** this commit (during the version-file write) — so by the time this step commits, the version write either succeeded or the run already aborted. The separate hook-artifact commit (task 3-3) is distinct and precedes this commit in the graph (task 3-8).

**Outcome**: When both the changelog and the version file change, mint produces **one** commit `{commit_prefix} Release {tag}` containing both. When the version file is unchanged but the changelog changes, mint still produces the one commit (with just the changelog). When nothing nets a change (changelog no-op and version file already at target, or no `version_file` configured and a no-op changelog), mint makes **no** commit (no empty commit). The version file is never given its own separate commit.

**Do**:
- Extend the Phase 1 Record step (task 1-9 bookkeeping commit) so that, in order:
  1. Write the changelog entry (Phase 1 writer; skipped when `changelog = false`).
  2. Run the version-file projection if `version_file` is configured: plain mode (task 3-5) or embedded mode (task 3-6) per whether `version_pattern` is set. (Embedded mismatch aborts here, before any commit — task 3-6.)
  3. **Stage both** the changelog change and the version-file change.
  4. Create **one** commit `{commit_prefix} Release {tag}` containing both.
- **No-op safety across both inputs**: commit **only if** at least one of {changelog, version file} produced a net change. If both are no-ops (changelog yields no net change AND the version file already holds the target, or there is no `version_file`), make **no** commit. Reuse Phase 1's no-op-skip mechanism, extended to consider the version-file change too.
- Do **not** give the version file its own commit — it is always folded into the single bookkeeping commit.
- Keep this commit distinct from the `pre_tag` hook-artifact commit (task 3-3): that one is project content and precedes this one; this one is release bookkeeping.
- Test via `FakeRunner`: both changelog + version file change → one `{commit_prefix} Release {tag}` commit staging both; version unchanged but changelog changes → still one commit; nothing net-changed → no commit; `version_file` absent → behaves exactly as Phase 1 (changelog-only bookkeeping commit).

**Acceptance Criteria**:
- [ ] When both the changelog and the version file change, exactly one `{commit_prefix} Release {tag}` commit is made containing both.
- [ ] When the version file is unchanged but the changelog changes, one commit is still made.
- [ ] When neither nets a change, no commit is made (no empty commit).
- [ ] The version file is never given its own separate commit (always folded in).
- [ ] With no `version_file` configured, behaviour matches Phase 1 (changelog-only bookkeeping commit).
- [ ] The bookkeeping commit is distinct from the `pre_tag` hook-artifact commit.
- [ ] All git operations go through the `CommandRunner`/`FakeRunner`.

**Tests**:
- `"changelog + version file change fold into one bookkeeping commit"`
- `"version unchanged but changelog changed still commits once"`
- `"nothing net-changed makes no commit (no empty commit)"`
- `"the version file is never given its own commit"`
- `"with no version_file, behaviour matches the Phase 1 bookkeeping commit"`

**Edge Cases**:
- Both changelog and version file change → one commit.
- Version unchanged but changelog changes → still commits.
- Nothing net-changed → no empty commit.

**Context**:
> Commit graph: "Release bookkeeping — the CHANGELOG entry and the version-file projection folded into one commit: `{commit_prefix} Release {tag}` (subject uses the configurable `commit_prefix`, default 🌿). (The legacy script made three commits per release — needlessly noisy.)" No-op safety: "No empty commits — if the changelog yields no net change, or the version file already holds the target version, mint skips that commit." Hook artifacts get their own separate commit (task 3-3); the bookkeeping commit folds changelog + version-file. Phase 1 (task 1-9) built the changelog-only bookkeeping commit with no-op skip; this extends it to fold the version file.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 5 → Commit graph, No-op safety".

## mint-release-tool-3-8 | approved

### Task mint-release-tool-3-8: Up-to-two-commit graph (hook-artifact then bookkeeping)

**Problem**: A release now produces **up to two** commits before the tag: an optional `pre_tag` hook-artifact commit (Stage 3, task 3-3) then the release-bookkeeping commit (Stage 5, task 3-7). The tag must point at the **bookkeeping** commit, and no-op safety must hold throughout (no empty commits anywhere). The combinations — both commits, only the bookkeeping commit, only the hook commit, or neither — must all resolve correctly, and the tag must always sit at HEAD after Record. Without assembling this graph, the ordering and the tag target are undefined when hooks and bookkeeping interact.

**Solution**: Assemble the commit graph in the spine: Stage 3 may produce the hook-artifact commit (task 3-3), Stage 5 may produce the bookkeeping commit (task 3-7); the annotated tag (Phase 1 task 1-10) is then created at the **resulting HEAD** (the bookkeeping commit when one was made, else the last commit present). The atomic push (Phase 1) sends both commits + tag together. No-op safety from tasks 3-3 and 3-7 means zero, one, or two commits can result; the tag always points at the bookkeeping commit / HEAD.

**Solution note**: This task wires the ordering and the tag target; the individual commits are tasks 3-3 (hook artifact) and 3-7 (bookkeeping). The atomic push and PONR are Phase 1 (task 1-10). The "neither dirty → zero commits" case means the bookkeeping commit no-ops too (e.g. changelog and version file both unchanged) — the tag then points at the pre-existing HEAD; whether that is a desirable release at all is the degenerate/no-op path already handled upstream (Phase 2), not re-decided here.

**Outcome**: A release with a dirtying `pre_tag` hook and a changed changelog/version produces **two** commits (hook-artifact then bookkeeping), with the tag at the bookkeeping commit. A release with no hook dirt but a changelog/version change produces **one** commit (bookkeeping), tag at it. A release with hook dirt but no bookkeeping change produces **one** (hook-artifact) commit, tag at HEAD. A release where neither nets a change produces **zero** new commits; the tag points at the existing HEAD. In every case the tag points at the bookkeeping commit when one exists, else at HEAD, and the atomic push sends commits + tag together.

**Do**:
- In the orchestrator, assemble the commit graph in spine order:
  1. **Stage 3**: run the `pre_tag` hook and, if it dirtied the tree, create the hook-artifact commit `chore(release): pre-tag artifacts for {tag}` (task 3-3). May be zero or one commit.
  2. **Stage 5 (Record)**: write changelog + version-file projection and, if either netted a change, create the bookkeeping commit `{commit_prefix} Release {tag}` (task 3-7). May be zero or one commit.
  3. **Tag**: create the annotated tag (Phase 1 task 1-10) at the **bookkeeping commit** when one was made; if the bookkeeping commit no-opped, the tag points at the current HEAD (which may be the hook-artifact commit, or the pre-existing HEAD if neither commit was made).
  4. **Push**: `git push --atomic origin HEAD {tag}` (Phase 1) sends the up-to-two commits + tag together.
- Ensure the **tag target** is always correct: the bookkeeping commit when present, else HEAD. Document that the tag conceptually points at "the release-bookkeeping commit" (Stage 5 commit graph step 3) and that HEAD equals that commit when one was made.
- Preserve **no-op safety** end to end: zero, one, or two commits are all valid outcomes; never create an empty commit at either step (tasks 3-3/3-7 already guard this — this task must not reintroduce empties when combining them).
- Test via `FakeRunner`: hook dirt + bookkeeping change → two commits, tag at bookkeeping; no hook dirt + bookkeeping change → one commit, tag at it; hook dirt + no bookkeeping change → one (hook-artifact) commit, tag at HEAD; neither → zero commits, tag at existing HEAD; atomic push includes all commits + the tag.

**Acceptance Criteria**:
- [ ] A dirtying hook + a changelog/version change → two commits (hook-artifact then bookkeeping).
- [ ] No hook dirt + a changelog/version change → one (bookkeeping) commit.
- [ ] Hook dirt + no bookkeeping change → one (hook-artifact) commit; tag points at HEAD.
- [ ] Neither dirty → zero new commits; tag points at the existing HEAD.
- [ ] The tag points at the bookkeeping commit when one exists, else at HEAD.
- [ ] No empty commit is ever created when combining the two steps.
- [ ] The atomic push sends all resulting commits + the tag together.
- [ ] All git operations go through the `CommandRunner`/`FakeRunner`.

**Tests**:
- `"a dirtying hook plus a bookkeeping change produces two commits with the tag at the bookkeeping commit"`
- `"no hook dirt plus a bookkeeping change produces one commit"`
- `"hook dirt with no bookkeeping change produces one hook-artifact commit and the tag at HEAD"`
- `"neither dirty produces zero commits and the tag at the existing HEAD"`
- `"the atomic push includes all resulting commits and the tag"`

**Edge Cases**:
- Hook commit + bookkeeping commit (two).
- No hook dirt → one commit.
- Neither dirty → zero commits.
- Tag always points at bookkeeping/HEAD.

**Context**:
> Commit graph (up to two commits, then tag): "1. Hook artifacts (only if a `pre_tag` hook dirtied the tree) → their own commit: `chore(release): pre-tag artifacts for {tag}`. 2. Release bookkeeping — the CHANGELOG entry and the version-file projection folded into one commit: `{commit_prefix} Release {tag}`. 3. Annotated tag points at the release-bookkeeping commit. 4. `git push --atomic` sends both commits + tag together — the single point of no return." No-op safety: "No empty commits — if the changelog yields no net change, or the version file already holds the target version, mint skips that commit." Phase 1 built the tag + atomic push (task 1-10); tasks 3-3/3-7 build the individual commits.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 5 → Commit graph, No-op safety", "Stages 6–7 → Point of no return".

## mint-release-tool-3-9 | approved

### Task mint-release-tool-3-9: Configurable diff_exclude globs (on top of built-in CHANGELOG.md)

**Problem**: Projects carry **tracked, committed** generated files (knowledge bundles, minified output, lockfiles, generated code) that are deliberately not in `.gitignore` but should not pollute the AI's view of the release. The `diff_exclude` glob array must be applied via git's `:(exclude)` pathspec **on top of** the built-in `CHANGELOG.md` exclusion (Phase 2), and excluded paths must not count toward `max_diff_lines`. Without configurable exclusion, generated-file churn dominates the diff and degrades notes.

**Solution**: Extend the Phase 2 diff context assembly (task 2-2) and Change Map (task 2-4) so the assembled git pathspec includes one `:(exclude)<glob>` entry per `diff_exclude` glob, **in addition to** the always-excluded `CHANGELOG.md`. Because git does the filtering and the `max_diff_lines` count (task 2-3) runs on the already-excluded diff, excluded paths inherently don't count. Exclusion remains path-based, never commit-based.

**Solution note**: This adds the `diff_exclude` tier to the existing exclusion machinery from Phase 2 — it does not rebuild diff assembly. The strategy-aware `version_file` exclusion is the separate task 3-10. `diff_exclude` was already read into config in Phase 2; this task *applies* it to the pathspec. Globs are passed to git as pathspec exclude patterns — let git interpret them (don't reimplement glob matching in Go).

**Outcome**: With `diff_exclude = ["skills/**/knowledge.cjs", "*.min.js"]` (for example), the assembled diff and Change Map exclude every matching path **and** `CHANGELOG.md`. Multiple globs all apply. A glob that matches nothing is harmless (no effect). A force-added gitignored (tracked) file that matches a glob is still excluded (exclusion is by path, and a tracked file can appear in the diff — the glob removes it). Excluded-path lines never count toward `max_diff_lines` (the count runs post-exclusion).

**Do**:
- Extend the Phase 2 pathspec construction (task 2-2's `AssembleDiff` and task 2-4's Change Map git commands) to append one `':(exclude)<glob>'` pathspec entry for **each** `diff_exclude` glob, alongside the built-in `':(exclude)CHANGELOG.md'`.
- Read the top-level shared `diff_exclude` key as an array of globs; **absent → empty array** (only `CHANGELOG.md` excluded). Full schema validation is Phase 6; Phase 3 reads the key with a default.
- **Let git interpret the globs** as pathspec patterns (`:(exclude)` semantics) — do not reimplement glob matching in Go. Multiple globs → multiple `:(exclude)` entries.
- Confirm **path-based, never commit-based** exclusion (consistent with Phase 2): the globs remove matching paths from the diff/Change Map regardless of which commit introduced them.
- **Force-added gitignored files**: a gitignored-yet-force-added file is tracked and can appear in the diff; a `diff_exclude` glob matching it still excludes it (the exclusion is by path). This is the intended interaction — not special-cased beyond the glob applying.
- **`max_diff_lines` interaction**: because the count (task 2-3) runs on the post-exclusion diff, the newly-excluded `diff_exclude` paths inherently don't count — verify this holds (no separate code needed beyond ordering exclusion before counting).
- Combine cleanly with the built-in `CHANGELOG.md` exclusion: both tiers apply together.
- Test via `FakeRunner`: assert the assembled git pathspec carries `:(exclude)CHANGELOG.md` plus one `:(exclude)<glob>` per configured glob; multiple globs; a non-matching glob is harmless; a force-added tracked file matching a glob is excluded; excluded paths don't count toward `max_diff_lines`.

**Acceptance Criteria**:
- [ ] Each `diff_exclude` glob becomes a `:(exclude)<glob>` pathspec entry, in addition to `:(exclude)CHANGELOG.md`.
- [ ] Multiple globs all apply.
- [ ] A glob matching nothing has no effect (harmless).
- [ ] A force-added gitignored (tracked) file matching a glob is still excluded.
- [ ] Excluded paths do not count toward `max_diff_lines` (count runs post-exclusion).
- [ ] Exclusion is path-based, never commit-based.
- [ ] `diff_exclude` absent → only `CHANGELOG.md` excluded.
- [ ] Git performs the glob matching (not reimplemented in Go); all calls via the `CommandRunner`/`FakeRunner`.

**Tests**:
- `"each diff_exclude glob is applied as a git exclude pathspec on top of CHANGELOG.md"`
- `"multiple diff_exclude globs all apply"`
- `"a diff_exclude glob matching nothing is harmless"`
- `"a force-added gitignored tracked file is excluded by a matching glob"`
- `"diff_exclude-excluded paths do not count toward max_diff_lines"`
- `"absent diff_exclude excludes only CHANGELOG.md"`

**Edge Cases**:
- Multiple globs.
- Glob matches nothing.
- Force-added gitignored file still excluded by glob.
- Excluded paths not counted toward `max_diff_lines`.
- Combined with `CHANGELOG.md` exclusion.

**Context**:
> Diff exclusion: "`diff_exclude` (project artifacts) — configurable array of globs, on top of the above (knowledge bundle, minified output, lockfiles, generated code). These are tracked, committed generated files (deliberately not in `.gitignore`), which is why explicit exclusion is needed. A release diff is commit-to-commit so it can only contain tracked files; gitignored files never appear. A file that is gitignored yet force-added is nonetheless tracked, so it can still appear in the diff — this edge is deliberate and not special-cased. Kept in config (not a `.mintignore` file)." "Exclusion is path-based, never commit-based." `max_diff_lines` guard: "Excluded paths don't count toward it." Config: `diff_exclude = ["skills/**/knowledge.cjs", "*.min.js"]` is a shared top-level engine key. Built-in `CHANGELOG.md` exclusion is Phase 2 (task 2-2); the strategy-aware `version_file` exclusion is task 3-10.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 4 → Diff exclusion (`diff_exclude`), max_diff_lines guard".

## mint-release-tool-3-10 | approved

### Task mint-release-tool-3-10: Strategy-aware version_file diff exclusion (plain excludes, embedded doesn't)

**Problem**: The `version_file` is **not** blanket-excluded from the notes diff — its handling is strategy-aware. In **plain mode** (whole file is the version, e.g. `release.txt`) the file is pure bookkeeping and should be **excluded**. In **embedded mode** (`version_pattern` in a real source file like `main.go`) it should **not** be excluded — it is source we want in notes, and the lone version-line bump is negligible and neutralised by the default prompt's "ignore version bumps" rule rather than by hiding real code. On the **forward path** there is nothing to exclude (the version write happens at Stage 5, after notes at Stage 4), so this rule is regenerate-only *in effect* — but the diff-assembly **decision** lands here and must be tested directly (the assembled pathspec / decision), not via the regenerate command (Phase 5).

**Solution**: Add the strategy-aware `version_file` rule to the diff-assembly pathspec decision (extending tasks 2-2 / 3-9): given the config, decide whether to add a `:(exclude)<version_file>` entry — **yes** in plain mode (no `version_pattern`), **no** in embedded mode (`version_pattern` set), and **nothing added** when `version_file` is unset. Test the **decision / assembled pathspec** directly. On the forward path the version file is inherently unchanged at notes time, so the rule has no effect in practice there; the rule exists for the regenerate path (Phase 5) to consume.

**Solution note**: Author this task to test the diff-assembly **rule** (the assembled pathspec / the exclude-or-not decision), NOT via the regenerate command (that is Phase 5). This task implements and unit-tests the decision function that produces the pathspec; it does not build regenerate. It composes with the built-in `CHANGELOG.md` exclusion (Phase 2) and the `diff_exclude` globs (task 3-9).

**Outcome**: The diff-assembly pathspec decision adds `:(exclude)<version_file>` **only** in plain mode; in embedded mode the `version_file` is **not** excluded (it stays in the diff, where the version-line bump is neutralised by the default prompt). With no `version_file`, nothing is added. On the forward path the version file is unchanged at notes time, so excluding-or-not is a no-op in effect — but the decision is correct and unit-tested so the regenerate path (Phase 5) inherits a correct rule. A `version_file` that is *also* listed in `diff_exclude` is excluded regardless (the explicit glob wins) — exclusion is the union.

**Do**:
- Extend the diff-assembly pathspec construction (tasks 2-2 / 3-9) with a strategy-aware `version_file` decision:
  - **Plain mode** (`version_file` set, **no** `version_pattern`) → add `':(exclude)<version_file>'` (pure bookkeeping).
  - **Embedded mode** (`version_file` + `version_pattern`) → **do not** exclude the `version_file` (it is source we want in notes; the version-line bump is neutralised by the default prompt's ignore-version-bumps rule, task 2-5).
  - **No `version_file`** → add nothing for this rule.
- Implement this as a **decision** the assembler consults (e.g. `versionFileExcludePathspec(cfg) (string, bool)`), and **unit-test the decision / assembled pathspec directly** — assert the presence/absence of the `:(exclude)<version_file>` entry for each mode. Do NOT exercise it through a regenerate command (Phase 5).
- **Forward-path note**: on the forward path notes generate (Stage 4) before the version write (Stage 5), so the file is inherently unchanged at notes time and excluding-or-not has no effect in practice. Document that the rule is regenerate-only *in effect* but the assembly rule lives here; a forward-path test asserts the decision is computed (and is inert because the file is unchanged), not that it changes forward output.
- **Composition / union**: this rule composes with `CHANGELOG.md` (Phase 2) and `diff_exclude` (task 3-9). If `version_file` is **also** in `diff_exclude`, it is excluded regardless of mode — exclusion is the union of all tiers; the explicit glob takes effect even in embedded mode.
- Test the decision directly: plain mode → `version_file` excluded; embedded mode → `version_file` NOT excluded; no `version_file` → nothing added; forward path → rule computed but inert (file unchanged); `version_file` also in `diff_exclude` → excluded.

**Acceptance Criteria**:
- [ ] Plain mode (`version_file`, no `version_pattern`) → `:(exclude)<version_file>` is added.
- [ ] Embedded mode (`version_file` + `version_pattern`) → `version_file` is NOT excluded.
- [ ] No `version_file` → nothing is added for this rule.
- [ ] The decision / assembled pathspec is tested directly (not via the regenerate command).
- [ ] On the forward path the rule is inert (the version file is unchanged at notes time) but the decision is still computed correctly.
- [ ] A `version_file` also listed in `diff_exclude` is excluded regardless of mode (union of tiers).
- [ ] Composes with the built-in `CHANGELOG.md` and `diff_exclude` exclusions.

**Tests**:
- `"plain mode adds a version_file exclude pathspec"`
- `"embedded mode does not exclude the version_file"`
- `"no version_file adds nothing for this rule"`
- `"the forward path computes the rule but it is inert (file unchanged)"`
- `"a version_file also in diff_exclude is excluded regardless of mode"`

**Edge Cases**:
- Plain mode → `version_file` excluded.
- Embedded mode → `version_file` NOT excluded.
- No `version_file` → nothing added.
- Forward path inherently unchanged so no effect.
- `version_file` also in `diff_exclude`.

**Context**:
> Diff exclusion (two tiers + strategy-aware version file): "`version_file` — NOT blanket-excluded (strategy-aware): Forward path: nothing to exclude — notes generate (Stage 4) before the version write (Stage 5), so the file is inherently unchanged at notes time. (The whole concern is therefore regenerate-only.) Regenerate, plain mode (whole file is the version, e.g. `release.txt`): exclude the file — pure bookkeeping. Regenerate, embedded mode (`version_pattern` in a real source file like `main.go`): do not exclude — it's source we want in notes. The lone version-line bump is negligible and neutralised by the default prompt's 'ignore version-number bumps' instruction, not by hiding real code." "Exclusion is path-based, never commit-based." The regenerate command itself is Phase 5; this task lands and unit-tests the diff-assembly decision. The default prompt's ignore-version-bumps rule is task 2-5.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 4 → Diff exclusion (strategy-aware version file)".

## mint-release-tool-3-11 | approved

### Task mint-release-tool-3-11: --dry-run skips all hooks and reports skipped

**Problem**: Hooks have side effects (builds, notifications, dispatches), so under `--dry-run` mint must **skip all hooks** (`preflight`/`pre_tag`/`post_release`) and **report** that they were skipped — never silently omit them. The injected `MINT_DRY_RUN` must be `1` for any context that would observe it. Full dry-run note caching is a separate, later concern (Phase 4) and must not be conflated with the hook skip. Without this, a dry run would execute side-effecting hooks.

**Solution**: Add a dry-run guard at each hook point (`preflight`/`pre_tag`/`post_release`): when `--dry-run` is set, **do not run** the hook and instead emit a Presenter report that the hook was skipped (naming the point). Because the `pre_tag` hook is skipped, no hook-artifact commit is produced under dry-run. `MINT_DRY_RUN` is set to `1` in the assembled `HookEnv` (task 3-1) so any reporting reflects dry-run state.

**Solution note**: This task implements ONLY the skip-and-report behaviour for hooks under `--dry-run`. Full dry-run semantics — the read-only preflight, the notes preview generation, and especially **dry-run note caching** (the cache key, TTL, reuse on the real run) — are **Phase 4** and explicitly out of scope here. Do not implement caching. The `--dry-run` flag's broader plan-printing/no-mutation behaviour is wired in its own phase; this task covers the hook dimension of dry-run.

**Outcome**: Under `--dry-run`, none of the three hooks execute; each configured hook point reports via the Presenter that it was skipped (e.g. "dry-run: skipping pre_tag hook"). No `chore(release): pre-tag artifacts for {tag}` commit is produced (the `pre_tag` hook didn't run, so nothing dirtied the tree by mint's hand). `MINT_DRY_RUN` is `1` in the hook env. Dry-run note caching is **not** implemented here (deferred to Phase 4).

**Do**:
- At each hook point wiring (tasks 3-2 `preflight`, 3-3 `pre_tag`, 3-4 `post_release`), add a `--dry-run` guard: when dry-run is active, **skip execution** of the hook and emit a Presenter report that the hook was skipped, naming the point. Report for configured points so the user sees what would have run (pick a consistent convention and document it).
- Ensure **no hook side effects**: under dry-run the hook runner (task 3-1) is **not invoked** for any point.
- **No artifact commit under dry-run**: because the `pre_tag` hook is skipped, the tree is not dirtied by a hook, so no `chore(release): pre-tag artifacts for {tag}` commit is created. (Dry-run also skips mutations generally — that broader behaviour is owned by the dry-run phase; here ensure specifically that skipping the hook prevents the artifact commit.)
- Set `MINT_DRY_RUN = 1` in the assembled `HookEnv` (task 3-1) when dry-run is active (and `0` otherwise) — even though hooks are skipped, keep the env builder correct for any reporting/consistency.
- **Explicitly out of scope**: dry-run note caching (cache key = hash of post-`diff_exclude` diff + version + prompt/context, ~1h TTL, reuse on the real run) is **Phase 4** — do NOT implement it. Add a comment marking it deferred.
- Test via `FakeRunner` + `RecordingPresenter`: under `--dry-run`, assert no hook command is run for any of the three points; assert each configured point reports "skipped"; assert no artifact commit is created; assert `MINT_DRY_RUN=1` in the env builder; assert no caching logic is present (caching is Phase 4).

**Acceptance Criteria**:
- [ ] Under `--dry-run`, none of `preflight`/`pre_tag`/`post_release` execute.
- [ ] Each configured hook point reports via the Presenter that it was skipped.
- [ ] No `pre_tag` artifact commit is produced under dry-run.
- [ ] `MINT_DRY_RUN` is `1` in the assembled hook env under dry-run (`0` otherwise).
- [ ] Dry-run note caching is NOT implemented here (deferred to Phase 4).
- [ ] No hook command is sent to the `CommandRunner` under dry-run.

**Tests**:
- `"--dry-run skips the preflight hook and reports it skipped"`
- `"--dry-run skips the pre_tag hook and reports it skipped"`
- `"--dry-run skips the post_release hook and reports it skipped"`
- `"--dry-run produces no pre_tag artifact commit"`
- `"MINT_DRY_RUN is 1 in the hook env under dry-run"`
- `"no hook command is run under dry-run"`

**Edge Cases**:
- All three hook points skipped + reported.
- `MINT_DRY_RUN=1` injected.
- No artifact commit when hooks skipped.
- Dry-run note caching out of scope (Phase 4).

**Context**:
> Dry-run behaviour (Hooks): "Under `--dry-run`, mint skips hooks (they have side effects) and reports that they were skipped." Dry-Run semantics: "Skips: all mutations (commit / tag / push / provider release) and all hooks (they have side effects) — and reports that hooks were skipped." Injected env: `MINT_DRY_RUN` (`0`/`1`). Dry-run note caching (Phase 4, OUT OF SCOPE here): "When `--dry-run` generates the notes preview, mint caches it so the subsequent real run reuses it … Cache key = hash of (post-`diff_exclude` diff + computed version + prompt / `[release].context`) … short TTL backstop (~1 hour) … gitignored cache, never committed." That caching is explicitly deferred to Phase 4; this task covers only the hook skip-and-report.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Hooks → Dry-run behaviour", "Dry-Run (`-d` / `--dry-run`) → Semantics (hooks skipped)".
