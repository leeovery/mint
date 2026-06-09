---
phase: 1
phase_name: Walking Skeleton — First-Release Cut, End-to-End
total: 11
---

## mint-release-tool-1-1 | approved

### Task mint-release-tool-1-1: Project skeleton & CommandRunner seam

**Problem**: This is a greenfield Go project (no `go.mod`, no source). Every later task invokes `git`, `gh`, and `claude` as external processes, and the fragile logic around those invocations is the whole reason the spec chose Go. There is no execution seam yet, so nothing can be built or tested in isolation from the real binaries.

**Solution**: Initialise the Go module and package layout, then define a single `CommandRunner` interface that abstracts external-command execution, with a real `os/exec`-backed implementation and a fake implementation for tests. Every subsequent git/gh/claude call in the codebase goes through this seam.

**Outcome**: `go build ./...` and `go test ./...` succeed on a clean checkout; a `CommandRunner` can run a command and return stdout, stderr, and exit status; the fake `CommandRunner` lets a test script the result of any named command (including non-zero exits and command-not-found) without touching the host.

**Do**:
- Run `go mod init github.com/leeovery/mint` at the repo root to create `go.mod` (Go 1.22+).
- Create package `internal/exec` (or `internal/runner`) with a `CommandRunner` interface. Suggested shape:
  - `type Result struct { Stdout string; Stderr string; ExitCode int }`
  - `type CommandRunner interface { Run(ctx context.Context, name string, args ...string) (Result, error) }`
  - Include an option for stdin (e.g. a `RunWith(ctx, stdin io.Reader, name string, args...)` method or a variadic options arg) because Stage 4 pipes a prompt to `claude -p` on stdin — establish the stdin path now even though Phase 1 does not exercise AI.
- Implement `ExecRunner` backed by `os/exec.CommandContext`: capture stdout/stderr separately into buffers, populate `Result.ExitCode` from the process exit status (extract via `*exec.ExitError`), and return a distinct sentinel/typed error when the binary is not found on `PATH` (wrap `exec.ErrNotFound` / inspect `errors.Is`).
- Implement `FakeRunner` for tests: a recorder that matches on command name (and optionally args) and returns a pre-seeded `Result` or error; records every invocation in order so tests can assert what was run. Support seeding a command-not-found error.
- Decide and document the convention: a non-zero exit is returned as a populated `Result` with a non-nil error so callers can both inspect `Stderr`/`ExitCode` and branch on `err != nil`.

**Acceptance Criteria**:
- [ ] `go mod init` complete; `go build ./...` and `go test ./...` pass on a clean checkout with no external binaries required.
- [ ] `ExecRunner.Run` returns stdout and stderr separately and a correct `ExitCode` for both success and failure.
- [ ] A non-zero exit is surfaced to the caller (non-nil error) with `Stderr` and `ExitCode` still readable on the `Result`.
- [ ] A missing binary (not on `PATH`) is surfaced as a distinguishable command-not-found condition, not a generic failure.
- [ ] `FakeRunner` can script results for named commands, record invocation order, and simulate command-not-found, with zero host interaction.

**Tests**:
- `"it runs a command and returns stdout"`
- `"it captures stderr separately from stdout"`
- `"it surfaces a non-zero exit code with stderr to the caller"`
- `"it surfaces command-not-found distinguishably from a non-zero exit"`
- `"FakeRunner returns seeded results and records invocations in order"`
- `"FakeRunner can simulate command-not-found"`

**Edge Cases**:
- Non-zero exit captured with stderr — caller must be able to read both the exit code and stderr text (preflight/abort messages depend on this).
- Command-not-found (binary absent from `PATH`) surfaced distinctly — the `gh` preflight gate (task 1-8) relies on telling "missing" apart from "ran and failed".

**Context**:
> Settled foundation: "Language: Go — chosen for testability of the fragile logic (git/`gh`/`claude` invocation) behind a single `CommandRunner` interface that can mock those external commands." Runtime tools are "invoked behind the `CommandRunner` seam and are runtime prerequisites, not build/spec dependencies." Stage 4 notes that mint "pipes [the prompt] to the command's stdin and reads the body from stdout" — hence the stdin affordance on the runner now.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Settled foundations", "Dependencies → Notes".

## mint-release-tool-1-2 | approved

### Task mint-release-tool-1-2: Minimal config load (tag_prefix, commit_prefix, release_branch, publish)

**Problem**: The release pipeline needs a handful of `[release]` config values (`tag_prefix`, `commit_prefix`, `release_branch`, `publish`) to compute tags, write commit/tag subjects, gate the branch check, and decide whether to publish. The full schema and fail-loud validation are Phase 6, but Phase 1 still needs these four values loaded with correct defaults so the end-to-end skeleton can run with zero config.

**Solution**: A minimal TOML config loader that reads `.mint.toml` from the repo root, extracts only the four Phase 1 keys from the `[release]` table, and applies defaults when the file or any key is absent. No strict unknown-key/bad-type validation yet (Phase 6) — unknown keys are tolerated/ignored at this stage.

**Solution note**: This is a deliberately narrow slice. Do not implement the full schema, the shared engine keys, `[release.hooks]`, or fail-loud validation here.

**Outcome**: Given no `.mint.toml`, the loader returns `tag_prefix="v"`, `commit_prefix="🌿"`, `release_branch=""` (meaning "auto-derive", resolved in task 1-4), `publish=true`. Given a partial file, present keys override defaults and absent keys keep defaults. A comments-only or blank file behaves identically to an absent file.

**Do**:
- Add a TOML decoder dependency (`github.com/pelletier/go-toml/v2` or `github.com/BurntSushi/toml`; pick one and `go get` it).
- Create package `internal/config` with a `Config` struct (or a `Release` sub-struct) holding at least: `TagPrefix string`, `CommitPrefix string`, `ReleaseBranch string`, `Publish bool`.
- Implement `Load(root string) (Config, error)`: read `{root}/.mint.toml`; if absent, return defaults. Decode the `[release]` table into the struct.
- Apply defaults: `tag_prefix` → `"v"`, `commit_prefix` → `"🌿"`, `release_branch` → `""` (sentinel for auto-derive), `publish` → `true`. Because `publish`'s zero value is `false`, distinguish "absent" from "explicitly false" (e.g. decode into `*bool` then default a nil to true, or pre-seed the struct with defaults before decoding).
- An empty `tag_prefix = ""` in config is a valid explicit value (prefix-less tags) and must NOT be coerced back to `"v"` — only an *absent* key defaults.
- Do not error on unknown keys in Phase 1 (Phase 6 adds fail-loud validation); a malformed-TOML parse error may surface as an error.

**Acceptance Criteria**:
- [ ] Absent `.mint.toml` yields all four defaults (`v`, `🌿`, `""`, `true`).
- [ ] A file specifying only a subset of keys overrides those keys and leaves the rest at default.
- [ ] `publish = false` is honoured (not lost to the bool zero-value vs. default ambiguity).
- [ ] An explicit `tag_prefix = ""` is preserved as empty, not re-defaulted to `"v"`.
- [ ] A blank or comments-only file behaves exactly like an absent file.
- [ ] Loader requires no other Phase 1 task to function (testable standalone against a temp dir).

**Tests**:
- `"it returns all defaults when .mint.toml is absent"`
- `"it overrides only the keys present and defaults the rest"`
- `"it honours publish = false"`
- `"it preserves an explicit empty tag_prefix"`
- `"it treats a blank/comments-only file as defaults"`
- `"it returns the configured commit_prefix and release_branch"`

**Edge Cases**:
- File absent → all defaults.
- Only a subset of keys present → the rest default.
- Comments/blank file → defaults (no decode error, no spurious values).
- `publish = false` vs. absent (bool default trap).
- Explicit empty `tag_prefix`.

**Context**:
> Config is "Fully optional. Zero config = sensible defaults everywhere." Defaults from the schema: `tag_prefix = "v"`, `commit_prefix = "🌿"` ("release commit + tag subject; cosmetic"), `release_branch` "default: auto-derived from origin/HEAD", `publish = true` ("false = tag + push only"). The spec mandates "Typed validation, fail-loud on unknown keys / bad types" — but that is consolidated in Phase 6; Phase 1 loads only this subset. Format is TOML at `.mint.toml` at the repo root, resolved via `git rev-parse --show-toplevel`.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Config Format & Schema".

## mint-release-tool-1-3 | approved

### Task mint-release-tool-1-3: Version determination from git tags

**Problem**: The version is sourced entirely from git tags — there is no file-based or embedded source. mint must read the complete tag set, parse only strict 3-part SemVer tags matching the configured prefix, find the global numeric maximum (not `git describe`'s nearest-reachable), and treat a repo with no matching tags as `0.0.0`. Without this, no version can be computed and the first-release path (the whole point of Phase 1) cannot start.

**Solution**: A version package that lists tags via the `CommandRunner`, parses each against `^{tag_prefix}(\d+)\.(\d+)\.(\d+)$`, ignores non-matching tags, and returns the highest matching `SemVer` (or `0.0.0` when none match), plus a bump function computing the next version from a bump kind.

**Outcome**: Given the repo's tags, `CurrentVersion` returns the global maximum matching SemVer or `0.0.0`; `Next(current, bump)` returns `current` bumped by patch/minor/major. With no tags: default (patch) → `0.0.1`, minor → `0.1.0`, major → `1.0.0`. Non-conforming tags are ignored entirely.

**Do**:
- Create package `internal/version` with a `SemVer struct { Major, Minor, Patch int }` and a `Bump` kind (`patch`/`minor`/`major`).
- Implement tag listing through the `CommandRunner` (e.g. `git tag --list` or `git for-each-ref --format='%(refname:short)' refs/tags`). The fetch `--tags` that guarantees the full set is a preflight concern (task 1-6); this task just reads what is present.
- Implement parsing against the exact pattern `^{tag_prefix}(\d+)\.(\d+)\.(\d+)$` (build the regex with the prefix escaped, `regexp.QuoteMeta`). Tags that do not match — `1.2`, `1.2.0-rc.1`, `1.2.0.4`, `1.2.0+build5`, `release-1.2`, or any with a different/absent prefix — are skipped.
- Implement `CurrentVersion(prefix)`: parse all matching tags, return the numeric maximum by (Major, then Minor, then Patch) comparison; if none match, return `0.0.0`.
- Implement `Next(current SemVer, bump Bump) SemVer`: patch → `Patch+1`; minor → `Minor+1`, `Patch=0`; major → `Major+1`, `Minor=0`, `Patch=0`.
- Implement a `String(prefix)` / tag-formatting helper that writes the prefix back: `{prefix}{Major}.{Minor}.{Patch}` (used by tagging in task 1-10 and the free-tag check in task 1-5/1-6).
- Ensure numeric (not lexical) comparison: `v10.0.0` > `v9.0.0`, `v1.10.0` > `v1.9.0`.

**Acceptance Criteria**:
- [ ] No matching tags → current version `0.0.0`.
- [ ] With no tags: default bump → `0.0.1`, minor → `0.1.0`, major → `1.0.0`.
- [ ] Non-conforming tags (`1.2`, `1.2.0-rc.1`, `1.2.0.4`, `1.2.0+build5`, `release-1.2`) are ignored when computing latest.
- [ ] Tags with a non-matching prefix are ignored; only `{tag_prefix}…` tags count; an empty prefix matches bare `X.Y.Z` tags.
- [ ] "Latest" is the global numeric maximum across all tags, with double-digit segments sorted numerically (`v10.0.0` > `v9.0.0`).
- [ ] Tag formatting writes the configured prefix back (`v0.0.1`, or `0.0.1` with empty prefix).

**Tests**:
- `"it returns 0.0.0 when no tags exist"`
- `"it returns 0.0.0 when no tags match the prefix"`
- `"it ignores non-3-part-semver tags (1.2, rc, 4-segment, build metadata, release-1.2)"`
- `"it selects the global numeric maximum, not lexical order (v10.0.0 > v9.0.0, v1.10.0 > v1.9.0)"`
- `"it ignores tags with a different prefix and matches the configured prefix"`
- `"it matches bare X.Y.Z tags when prefix is empty"`
- `"it bumps patch/minor/major correctly including from 0.0.0"`
- `"it formats a version back with the configured prefix"`

**Edge Cases**:
- No matching tags → `0.0.0`.
- Non-matching tags ignored: `1.2`, `1.2.0-rc.1`, `1.2.0.4`, `release-1.2`.
- Mixed prefixes present (only the configured prefix counts).
- Double-digit segments sorted numerically, not lexically.

**Context**:
> "The current version is the highest SemVer tag in the repository (stripped of its prefix). There is no file-based or embedded version source … With no matching tags, the current version is `0.0.0`." "'Latest' = the numerically highest matching version, globally — not `git describe`'s nearest-reachable-from-HEAD." Grammar: "Strict SemVer 2.0.0, three numeric segments only: MAJOR.MINOR.PATCH. Anything else … is not a mint version and is ignored entirely." Pattern: `^{tag_prefix}(\d+)\.(\d+)\.(\d+)$`. "`tag_prefix` config, default 'v' … Overridable to '' or anything else." First release: "with no tags the current version is `0.0.0`, so `mint release` → `0.0.1`, `mint release -m` → `0.1.0`, `mint release -M` → `1.0.0`." `--set-version` and explicit-version validation are deferred to Phase 4 — do not implement here.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 1 — Version Determination & Tag Grammar".

## mint-release-tool-1-4 | approved

### Task mint-release-tool-1-4: Repo root anchoring & release-branch resolution

**Problem**: mint must anchor every operation at the repository root and know which branch is the release branch before it can run the preflight gates. The root is resolved via `git rev-parse --show-toplevel`; the release branch defaults to the one auto-derived from `origin/HEAD` (resolving `main`/`master` with zero config) unless overridden by config. Without these, config loading (which reads `.mint.toml` at root) and the branch gate (task 1-5) have no anchor.

**Solution**: A repository-context resolver that uses the `CommandRunner` to find the repo root (failing cleanly if not in a git repo) and to determine the release branch — config override if set, else the branch `origin/HEAD` points at.

**Outcome**: `ResolveRoot` returns the absolute repo root or a clean "not a git repository" abort when outside one. `ResolveReleaseBranch` returns the configured `release_branch` if non-empty, otherwise the branch derived from `origin/HEAD` (e.g. `main`), and surfaces a clear condition when `origin/HEAD` is unset and no override is configured.

**Do**:
- In a `internal/gitrepo` (or `internal/repo`) package, implement `ResolveRoot(runner)`: run `git rev-parse --show-toplevel`; on success return the trimmed path; on failure (non-zero exit / not a git repo) return a clean error suitable for an abort message, not a panic.
- Note in code comments the spec's resolution semantics: `--show-toplevel` resolves to the innermost enclosing repo or linked worktree; a submodule is its own repo, a linked worktree shares the main repo's ref store. No special handling is required — git's resolution is authoritative — but the comment records the intent.
- Implement `ResolveReleaseBranch(runner, cfg)`: if `cfg.ReleaseBranch != ""`, return it. Otherwise derive from `origin/HEAD` — e.g. `git symbolic-ref --short refs/remotes/origin/HEAD` (yielding `origin/main`) and strip the `origin/` prefix, or `git rev-parse --abbrev-ref origin/HEAD`. Return the short branch name (`main`/`master`).
- Handle `origin/HEAD` unset: if the derivation fails and no config override exists, return a distinguishable error/condition the caller can turn into a clear abort or guidance. (Phase 1 may surface this as an abort; do not auto-pick `main` silently.)
- Wire the config-loaded `release_branch` from task 1-2 as the override source.

**Acceptance Criteria**:
- [ ] `ResolveRoot` returns the repo root path via `git rev-parse --show-toplevel`.
- [ ] Outside a git repo, `ResolveRoot` aborts cleanly with a "not a git repository" condition (no panic).
- [ ] `ResolveReleaseBranch` returns the config `release_branch` when set, ignoring `origin/HEAD`.
- [ ] With no config override, `ResolveReleaseBranch` derives the branch from `origin/HEAD` (e.g. `main`).
- [ ] With `origin/HEAD` unset and no override, the resolver surfaces a clear, distinguishable condition rather than silently defaulting.
- [ ] All git calls go through the `CommandRunner` and are exercised with the `FakeRunner`.

**Tests**:
- `"it resolves the repo root from git rev-parse --show-toplevel"`
- `"it aborts cleanly when not in a git repository"`
- `"it returns the configured release_branch when set"`
- `"it derives the release branch from origin/HEAD when no override"`
- `"it surfaces a clear condition when origin/HEAD is unset and no override is set"`

**Edge Cases**:
- Not a git repo → clean abort.
- `origin/HEAD` unset → distinguishable condition (no silent default).
- `release_branch` config override → used verbatim, derivation skipped.

**Context**:
> Preflight gate 1: "Git repo present, anchored at the repo root (resolved via `git rev-parse --show-toplevel`; mint runs from root)." Submodule/worktree resolution: "`git rev-parse --show-toplevel` resolves to the innermost enclosing repository or linked worktree … mint always anchors to and runs from that resolved root." Gate 2: "On the release branch — default-on, auto-derived from `origin/HEAD` (resolves main/master with zero config). Override via `release_branch` in config." Config location: "mint resolves the root via `git rev-parse --show-toplevel`, looks for `.mint.toml` there, and runs from root." The `--any-branch` escape hatch is deferred to Phase 4 — not in scope here.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 2 — Preflight & Safety Gates" (gates 1–2), "Config Format & Schema → Location".

## mint-release-tool-1-5 | approved

### Task mint-release-tool-1-5: Local preflight gates (clean tree, on branch, tag-free local)

**Problem**: Releasing is high-consequence, so mint forces a known-good starting state before any mutation. The cheap local gates — clean working tree, on the release branch, and the target tag not already existing locally — must run (in that cheap-first order) and abort cleanly on failure. Without them, a re-run could double-release or a dirty tree could be tagged.

**Solution**: A preflight package implementing the three local gates over the `CommandRunner`, each returning a clear pass/fail with an actionable abort message, runnable in order before any network call or mutation.

**Outcome**: Given a clean tree, current branch == release branch, and no local tag matching the computed `{tag_prefix}X.Y.Z`, all three gates pass. A dirty tree, an off-branch checkout, or a pre-existing local tag each aborts cleanly with a specific message. Gitignored files never trip the clean-tree gate.

**Do**:
- In `internal/preflight`, implement `CheckCleanTree(runner)`: run `git status --porcelain`; non-empty output → fail. Gitignored files are exempt by default (`git status --porcelain` already excludes them); the gate must block on uncommitted/unstaged tracked changes AND non-ignored untracked files. Do NOT pass `--ignored`.
- Implement `CheckOnBranch(runner, releaseBranch)`: determine the current branch (`git rev-parse --abbrev-ref HEAD` or `git symbolic-ref --short HEAD`); fail if it differs from `releaseBranch` (resolved in task 1-4), with a message naming both.
- Implement `CheckTagFreeLocal(runner, tag)`: check whether `{tag}` exists locally (`git rev-parse -q --verify refs/tags/{tag}` or `git tag --list {tag}`); if it exists → fail ("tag {tag} already exists").
- Order the local gates cheap-first: clean tree, on branch, tag-free local. (Network gates — remote sync, tag-free remote — are task 1-6 and run after these.)
- Each gate returns a typed failure carrying a human abort message; nothing mutates state.
- The `--autostash` (clean-tree bypass) and `--any-branch` (branch bypass) escape hatches are Phase 4 — do NOT implement them here.

**Acceptance Criteria**:
- [ ] Clean tree + on branch + tag-free local → all gates pass.
- [ ] Dirty tracked changes (uncommitted/unstaged) → clean-tree gate fails.
- [ ] Non-ignored untracked files → clean-tree gate fails.
- [ ] Gitignored files present but no other changes → clean-tree gate passes (exempt).
- [ ] Current branch != release branch → on-branch gate fails with a message naming both branches.
- [ ] A local tag matching the computed tag already exists → tag-free-local gate fails.
- [ ] Gates run cheap-first (tree, branch, tag) and abort on the first failure; all via `CommandRunner`/`FakeRunner`.

**Tests**:
- `"it passes when tree is clean, on branch, and tag is free locally"`
- `"it fails on uncommitted tracked changes"`
- `"it fails on non-ignored untracked files"`
- `"it passes when only gitignored files are present"`
- `"it fails when not on the release branch"`
- `"it fails when the target tag already exists locally"`

**Edge Cases**:
- Dirty tracked changes → fail.
- Non-ignored untracked files → fail.
- Gitignored files exempt → pass.
- Not on release branch → fail.
- Tag exists locally → fail.

**Context**:
> Gate 3: "Clean working tree (strict) — `git status --porcelain` must be empty. Gitignored files are exempt (build outputs don't trip it); blocks on uncommitted/unstaged tracked changes and non-ignored untracked files." Gate 2: "On the release branch." Gate 4: "Target tag is free — the computed `{tag_prefix}X.Y.Z` must not exist locally or on the remote. Closes the double-release / re-run footgun." "All preflight checks are cheap and reversible, and all run before any mutation or hooks." "the gate set (run in order — cheap local checks first, then network)." The `--autostash` and `--any-branch` escape hatches are deferred to Phase 4.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 2 — Preflight & Safety Gates" (gates 2–4).

## mint-release-tool-1-6 | approved

### Task mint-release-tool-1-6: Network preflight gates (fetch --tags, remote sync, tag-free remote)

**Problem**: The local gates can't see the remote. mint must fetch (including `--tags`, so it always sees the complete tag set) and then refuse to release if the local branch is behind or diverged from its upstream (auto-pulling would silently drag in and release unseen commits), and refuse if the target tag already exists on the remote. Being ahead is fine — those are the commits being released.

**Solution**: The network half of preflight: a fetch that pulls tags, a remote-sync gate computing ahead/behind against the release branch's upstream, and a remote tag-free check — all over the `CommandRunner`, all read-only, all aborting cleanly without ever auto-pulling.

**Outcome**: After `git fetch --tags`, a branch that is up-to-date or ahead passes; behind aborts with a commit count ("N commits behind origin/main — pull and review, then re-run"); diverged aborts. A target tag already present on the remote aborts. mint never pulls.

**Do**:
- In `internal/preflight`, implement `Fetch(runner)`: run `git fetch --tags` (and the release branch's upstream) so the full tag set and upstream refs are current. This must run before version determination's "latest" is trusted and before the remote gates. Coordinate ordering with task 1-11's wiring (fetch precedes the remote-sync and remote-tag checks).
- Implement `CheckRemoteSync(runner, releaseBranch)`: compute ahead/behind against the upstream — e.g. `git rev-list --left-right --count {upstream}...HEAD` (or `@{u}...HEAD`). Behind (>0 behind) or diverged (>0 behind AND >0 ahead) → fail with a clear message including the behind count; up-to-date or purely ahead → pass. Never run `git pull`.
- Handle no upstream configured: surface a clear condition (the branch has no tracking remote) rather than crashing; treat as a distinguishable preflight result the caller can report.
- Implement `CheckTagFreeRemote(runner, tag)`: after the fetch, the tag would appear locally if it exists remotely; alternatively probe `git ls-remote --tags origin refs/tags/{tag}`. A non-empty result → fail ("tag {tag} already exists on remote").
- Keep all checks read-only; no mutation.

**Acceptance Criteria**:
- [ ] `git fetch --tags` runs before the remote gates so the full tag set is visible.
- [ ] Branch behind upstream → abort with the behind commit count in the message.
- [ ] Branch diverged (both ahead and behind) → abort.
- [ ] Branch ahead only (or up-to-date) → pass.
- [ ] mint never runs `git pull` (no auto-integration).
- [ ] No upstream configured → distinguishable, clearly-reported condition (no crash).
- [ ] Target tag present on remote → abort.

**Tests**:
- `"it fetches with --tags before remote checks"`
- `"it aborts with a commit count when behind upstream"`
- `"it aborts when diverged from upstream"`
- `"it passes when ahead of upstream"`
- `"it passes when up-to-date with upstream"`
- `"it never invokes git pull"`
- `"it reports clearly when there is no upstream"`
- `"it aborts when the target tag exists on the remote"`

**Edge Cases**:
- Behind → abort with count.
- Diverged → abort.
- Ahead → pass.
- No upstream → clear, distinguishable condition.
- Tag exists on remote → abort.

**Context**:
> Gate 5: "Remote sync — `git fetch`, then abort (never auto-pull) if local is behind or diverged from the release branch's upstream. Being ahead is fine and expected (those are the commits being released). Auto-pulling would silently drag in unseen remote commits and release them; integrating remote work must be a conscious act. Clear abort message, e.g. 'N commits behind origin/main — pull and review, then re-run'." Gate 4: "Target tag is free — … must not exist locally or on the remote." "Preflight's fetch includes `--tags`, so mint always sees the complete tag set even after a fresh/partial clone." The `gh` install+auth gate is task 1-8 (runs only when publishing, before the tag).

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 2 — Preflight & Safety Gates" (gates 4–5), "Stage 1 → Source of truth".

## mint-release-tool-1-7 | approved

### Task mint-release-tool-1-7: Presenter interface & recording fake

**Problem**: The release run emits semantic events (plan summary, computed version, gate results, notes, the review gate, push/publish outcomes) to the user, but gate *rendering* and the concrete `pretty`/`plain` renderers are owned by the separate CLI Presentation specification (a cross-spec dependency not built here). The engine must still compile and be fully testable without a concrete renderer.

**Solution**: Define the `Presenter` interface — the event/method surface the engine calls — and a recording/fake implementation for tests that captures every emitted event. The engine is wired to emit through this interface only; no concrete renderer is required to build or test it.

**Outcome**: A `Presenter` interface exists with the semantic methods the Phase 1 pipeline needs; a `RecordingPresenter` captures emitted events so tests can assert what the engine reported. The engine builds and all Phase 1 tests pass against the fake, with no concrete renderer present.

**Do**:
- In `internal/presenter`, define a `Presenter` interface covering the semantic events the Phase 1 forward path emits. Keep methods semantic (what happened), not presentational (how to draw it). Suggested surface, to be confirmed against the pipeline's needs:
  - plan / version summary (e.g. `Plan(summary PlanInfo)` carrying current → next version, tag, publish target).
  - stage/gate progress and failure (e.g. `GateFailed(name, message)` or a generic `Step`/`Warn`/`Error`).
  - notes shown (the body about to be recorded).
  - push and publish outcomes, including the post-PONR warn-only case (`Warn(message)` used by task 1-11's publish-failure path).
- Because the interactive notes-review gate's *rendering* and the four-choice mapping belong to the CLI Presentation spec, model the engine's need as a minimal decision request the engine can call (e.g. a `Confirm`/gate method returning a choice) so the engine has a seam — but do NOT implement gate rendering. For Phase 1's first-release path the gate is `y`/`n`/`e` only (no `r`); a `-y` run bypasses it. The fake returns a scripted choice.
- Implement `RecordingPresenter`: records each call (method + payload) in order; for any decision method, returns a pre-seeded answer. Provide assertion helpers (e.g. last warning, recorded events).
- Add a doc comment marking this interface as the cross-spec boundary: "Concrete pretty/plain renderers and gate rendering are owned by the CLI Presentation specification; the engine depends only on this interface."

**Acceptance Criteria**:
- [ ] A `Presenter` interface exists with semantic methods for the Phase 1 pipeline (plan/version, gate failure, notes shown, push/publish outcome incl. warn).
- [ ] A decision/gate seam exists so the engine can request the review choice without owning its rendering.
- [ ] `RecordingPresenter` captures emitted events in order and returns scripted decisions.
- [ ] The engine builds and the Phase 1 tests run entirely against the fake — no concrete renderer is referenced.
- [ ] A doc comment records the cross-spec ownership boundary.

**Tests**:
- `"the recording presenter captures emitted events in order"`
- `"the recording presenter returns a scripted review-gate choice"`
- `"the recording presenter records a warn (post-PONR) event"`
- `"the engine compiles and reports through the Presenter with no concrete renderer"`

**Edge Cases**:
- none (per the task table).

**Context**:
> Dependencies — Partial Requirement: "The release run emits semantic events to a `Presenter` and shows the interactive notes-review gate through it; this spec defers gate rendering and the `--plain` flag's behaviour to that spec." Minimum scope: "The `Presenter` interface (the event/method surface the engine calls) and the notes-review gate rendering contract … Concrete pretty/plain renderers and `--plain` detection can land in parallel — the engine builds and tests against the interface (a fake/recording presenter)." Interactive review (Phase 1 first-release): the gate offers only `y`/`n`/`e` on no-AI paths ("`r` regenerate is omitted, since there is no AI invocation to nudge"); `-y`/`--yes` skips the gate. Phase 1 builds the *seam*; the full four-choice gate behaviour lands in Phase 2.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Dependencies → Partial Requirement", "Interactive Confirmation & Notes Review".

## mint-release-tool-1-8 | approved

### Task mint-release-tool-1-8: Publisher interface & GitHub driver (gh gate when publishing)

**Problem**: Publishing the GitHub release is first-class and provider-abstracted — not a hook, not hardcoded to `gh` at the call site. mint also must gate `gh` install + auth **only when actually publishing** and **before the tag**, so a missing/unauthenticated `gh` never strands a pushed tag with no release. Phase 1 needs the `Publisher` seam plus the GitHub driver and that conditional gate.

**Solution**: A `Publisher` interface (`CreateRelease`/`UpdateRelease`) with a GitHub driver that shells `gh` through the `CommandRunner`, plus a `gh` install+auth preflight check that runs conditionally (only when `publish = true`) and is sequenced before tag creation.

**Outcome**: When `publish = true`, mint verifies `gh` is installed and authenticated before the tag is created; if `gh` is missing or unauthenticated it aborts before any tag/push. When `publish = false`, the `gh` gate is skipped and no publish is attempted. The GitHub driver's `CreateRelease` invokes `gh release create … --verify-tag` with the tag and the notes body via the `CommandRunner`.

**Do**:
- In `internal/publish`, define `type Publisher interface { CreateRelease(ctx, tag, title, body string) error; UpdateRelease(ctx, tag, title, body string) error }`. (Phase 1 exercises `CreateRelease`; `UpdateRelease` is part of the seam for Phase 5 — stub it but keep it on the interface.)
- Implement `GitHubPublisher` backed by the `CommandRunner`: `CreateRelease` runs `gh release create {tag} --title {title} --notes {body} --verify-tag` (pass the body safely, e.g. `--notes-file -` on stdin or a temp file, to avoid arg-length/escaping issues with a long body).
- Implement `CheckGhAuth(runner)` in the preflight package: verify `gh` is installed (command-not-found → fail "gh not installed") and authenticated (`gh auth status` non-zero → fail "gh not authenticated"). Distinguish the two using the command-not-found signal from task 1-1.
- Make the gate conditional and ordered: the caller (task 1-11) runs `CheckGhAuth` **only when `publish = true`** and **after the other preflight gates but before tag creation**. Document this ordering invariant in the gate/driver code.
- For Phase 1, the provider is GitHub directly. Full auto-detection from the remote host and the unknown-provider/no-driver downgrade are Phase 4 — do NOT implement detection/downgrade here; assume GitHub when `publish = true`.

**Acceptance Criteria**:
- [ ] `Publisher` interface defined with `CreateRelease` and `UpdateRelease`; `GitHubPublisher` implements it over the `CommandRunner`.
- [ ] `CreateRelease` invokes `gh release create {tag} … --verify-tag` with the title and full notes body passed safely.
- [ ] When `publish = true`, the `gh` install+auth gate runs before tag creation.
- [ ] `gh` missing → abort before any tag/push, with a "not installed" message.
- [ ] `gh` unauthenticated → abort before any tag/push, with a "not authenticated" message.
- [ ] When `publish = false`, the `gh` gate is skipped and no publish is attempted.
- [ ] All `gh` invocations go through the `CommandRunner`/`FakeRunner`.

**Tests**:
- `"CreateRelease invokes gh release create with the tag, title, body and --verify-tag"`
- `"the gh gate passes when gh is installed and authenticated"`
- `"it aborts before the tag when gh is not installed"`
- `"it aborts before the tag when gh is not authenticated"`
- `"the gh gate is skipped when publish = false"`
- `"no publish is attempted when publish = false"`

**Edge Cases**:
- `publish = false` → no gh gate, no publish.
- `gh` missing → abort before tag.
- `gh` unauthenticated → abort before tag.

**Context**:
> Gate 6: "`gh` installed + authenticated — gated only when actually publishing a GitHub release, and before the tag, so a missing/unauthenticated `gh` never strands a pushed tag with no release." Publishing: "first-class but provider-abstracted — not hardcoded to `gh`, and not a `post_release` hook." "Behind a small `Publisher` interface (`CreateRelease` / `UpdateRelease`). mint auto-detects the provider from the remote host (`github.com` → GitHub driver via `gh`), overridable by the `provider` config." "GitHub is the only driver implemented now." Config: "`publish` (default `true`; `false` = tag + push only)." Auto-detection from host and the unknown-provider/no-driver loud-downgrade are Phase 4 — not implemented here.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 2 → gate 6", "Stages 6–7 → Publishing: provider driver abstraction".

## mint-release-tool-1-9 | approved

### Task mint-release-tool-1-9: First-release body & Record (changelog + bookkeeping commit)

**Problem**: On a first release there is no prior tag to diff and diffing the whole repo is useless, so mint must skip the AI entirely and use the fixed body `"Initial release."`. That single body then flows into the Record stage: mint writes the CHANGELOG entry (creating the file with a Keep a Changelog preamble if absent) and creates the one release-bookkeeping commit. Phase 1 has no hooks and no version-file projection, so this is a single bookkeeping commit.

**Solution**: A first-release body provider returning the fixed `"Initial release."` string (no AI invocation at all), plus the Record stage: a Keep a Changelog writer that prepends a `## [x.y.z] - YYYY-MM-DD` section (creating the file with the standard preamble when absent) and stages + commits it as `{commit_prefix} Release {tag}`.

**Outcome**: For a no-prior-tag run, the notes body is exactly `"Initial release."` with no `claude` invocation recorded. The CHANGELOG gets a dated first version section (file created with the KaC preamble if it didn't exist), and mint produces one commit subjected `{commit_prefix} Release {tag}`. If the changelog write yields no net change, no commit is made.

**Do**:
- Implement a first-release body provider that returns the constant `"Initial release."` and never calls the AI transport. This is the highest-precedence notes path; for Phase 1 it is the only path (the AI engine arrives in Phase 2).
- In `internal/record` (or `internal/changelog`), implement the changelog writer:
  - If `CHANGELOG.md` is absent, create it with the standard Keep a Changelog header preamble, then the first version section.
  - Insert the new `## [{x.y.z}] - {YYYY-MM-DD}` section (carrying the full body) below the KaC preamble and above any prior `## [` block (newest on top). For the first release there are no prior sections.
  - Use the current date for the section header.
  - Skip the changelog entirely when `changelog = false` (config). (Phase 1's minimal config does not expose `changelog`; treat absent as the default `true`. The `changelog` key wiring can be deferred, but honour the no-op-skip behaviour.)
- Implement the release-bookkeeping commit: stage the changelog change and commit with subject `{commit_prefix} Release {tag}` (e.g. `🌿 Release v0.0.1`) via the `CommandRunner` (`git add` + `git commit`).
- No-op safety: if the changelog write produces no net change (nothing to stage), skip the commit rather than creating an empty commit.
- Phase 1 scope: exactly one (bookkeeping) commit. The optional hook-artifact commit and the version-file projection fold-in are Phase 3 — do NOT implement them here.

**Acceptance Criteria**:
- [ ] First-release body is exactly `"Initial release."` with no AI/`claude` invocation.
- [ ] Absent `CHANGELOG.md` → created with the Keep a Changelog preamble plus the first `## [x.y.z] - date` section.
- [ ] Existing `CHANGELOG.md` → the new section is inserted below the preamble, newest on top (no prior section for first release).
- [ ] The section header uses the `## [x.y.z] - YYYY-MM-DD` format with the current date.
- [ ] The release-bookkeeping commit subject is `{commit_prefix} Release {tag}` (default `🌿 Release v0.0.1`).
- [ ] No-op changelog (no net change) → no commit is created.

**Tests**:
- `"the first-release body is 'Initial release.' with no AI call"`
- `"it creates CHANGELOG.md with the KaC preamble when absent"`
- `"it inserts the first version section with the current date"`
- `"it prepends the new section below the preamble, newest on top"`
- `"it commits with subject {commit_prefix} Release {tag}"`
- `"it skips the commit when the changelog write is a no-op"`

**Edge Cases**:
- `CHANGELOG.md` absent → create with KaC preamble.
- No-op changelog → skip the commit (no empty commit).

**Context**:
> Diff base: "First release (no prior tag): there's no base to diff and diffing the whole repo is useless to an AI → mint skips the AI and uses a fixed body, 'Initial release.'" Notes-path precedence (1): "First release (no prior tag) → fixed body 'Initial release.', no AI. Wins over everything below." Changelog mechanics: "A complete `## [x.y.z] - YYYY-MM-DD` section (the full notes body), written atomically at release time, inserted above the most recent existing `## [` block." "First release — if `CHANGELOG.md` doesn't exist, mint creates it with the standard Keep a Changelog header preamble first, then the first version section." "Newest on top." "No `[Unreleased]` section." Commit graph: "Release bookkeeping — the CHANGELOG entry and the version-file projection folded into one commit: `{commit_prefix} Release {tag}`." No-op safety: "No empty commits — if the changelog yields no net change … mint skips that commit." Hook-artifact commit and version-file projection are Phase 3.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 4 → Diff base", "Notes-path precedence", "Stage 5 — Record".

## mint-release-tool-1-10 | approved

### Task mint-release-tool-1-10: Annotated tag & atomic push

**Problem**: Stage 6 is the single point of no return. mint must create an annotated tag (signable, offline, immutable) whose subject is `{commit_prefix} Release {tag}` and whose body is the full notes, then `git push --atomic origin HEAD {tag}` so commit(s) and tag go up together or not at all. A rejected push must be surfaced and must not lead to a publish attempt.

**Solution**: A tag+push module: create the annotated tag at the release-bookkeeping commit with the subject+body, then run the atomic push, treating push success as the crossing of the point of no return and a push failure as a clean pre-PONR abort that surfaces and stops before publishing.

**Outcome**: mint creates an annotated tag `{tag}` pointing at the bookkeeping commit, subject `{commit_prefix} Release {tag}`, body = the full notes (`"Initial release."` in Phase 1). `git push --atomic origin HEAD {tag}` sends commit(s) + tag together. A rejected push is surfaced via the Presenter and no publish is attempted.

**Do**:
- In `internal/release` (tag/push area), implement annotated-tag creation via the `CommandRunner`: `git tag -a {tag} -F -` (body on stdin) or `-m` with the subject line `{commit_prefix} Release {tag}` followed by the full notes body. Annotated (not lightweight) is mandatory — the annotation body is the single source mint ever reads later.
  - Compose the tag message as: subject `{commit_prefix} Release {tag}` (e.g. `🌿 Release v0.0.1`) + a blank line + the full body.
- Implement the atomic push: `git push --atomic origin HEAD {tag}`. This is the point of no return — commits + tag together.
- On push rejection/failure (non-zero exit): surface the failure (Presenter error + the abort) and STOP — do not proceed to publish. Because this is still pre-PONR (the push did not succeed), nothing was published; the surgical local auto-unwind that deletes the just-made tag and resets the commit(s) is Phase 4 (this task surfaces the failure; it does not implement the full unwind).
- On push success: signal to the caller that the PONR has been crossed (publish may now proceed; from here failures are warn-only — wired in task 1-11).
- Lock-resilient git wrapping (retry/clear stale `.git` lock) around these mutations is Phase 4 — call the runner directly here; do not implement lock resilience.

**Acceptance Criteria**:
- [ ] An annotated tag (not lightweight) is created at the bookkeeping commit with subject `{commit_prefix} Release {tag}`.
- [ ] The tag annotation body carries the full notes body verbatim.
- [ ] `git push --atomic origin HEAD {tag}` is the push form (commits + tag together).
- [ ] A rejected/failed push is surfaced via the Presenter and the run stops.
- [ ] No publish is attempted when the push fails.
- [ ] Push success signals the PONR crossing so the caller may publish.
- [ ] All git operations go through the `CommandRunner`/`FakeRunner`.

**Tests**:
- `"it creates an annotated tag with subject {commit_prefix} Release {tag} and the full body"`
- `"it pushes with git push --atomic origin HEAD {tag}"`
- `"it surfaces a rejected push and stops"`
- `"it does not attempt publish when the push fails"`
- `"it signals the point of no return on push success"`

**Edge Cases**:
- Push rejected → surfaced; no publish attempted.

**Context**:
> Tag annotation: "subject `{commit_prefix} Release {tag}` + the FULL notes body (default `commit_prefix` is 🌿). Annotated (not lightweight): signable, offline, in-repo, immutable. This is the single source mint ever reads." Point of no return: "`git push --atomic origin HEAD {tag}` is the single point of no return — commits + tag go up together or not at all." Failure model (before the push): "Everything mint did is local-only. mint auto-unwinds its own mutations …" — the *surgical auto-unwind itself is Phase 4*; Phase 1 surfaces the failure and stops. Lock-resilient git wrapping is also Phase 4. Optionality stack: "Annotated tag — Mandatory — always created, always carries a body."

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Body Distribution → Tag annotation", "Stages 6–7 → Point of no return", "Stage 5 → Commit graph".

## mint-release-tool-1-11 | approved

### Task mint-release-tool-1-11: Release command wiring (end-to-end first-release)

**Problem**: The Phase 1 pieces — config, version, preflight gates, Presenter, Publisher, first-release body/Record, tag/push — exist independently but nothing threads them into a runnable `mint release` that takes a repo with no tags to a published `0.0.1`. This task is the vertical seam that proves the spine end-to-end and demonstrates the point-of-no-return asymmetry (warn-only after push).

**Solution**: The CLI entry point and release orchestrator: parse the bump flag and `-y`, run the pipeline in spine order (version → preflight → first-release body → record → gh gate → tag → atomic push → publish), and apply the PONR asymmetry — pre-push failures abort, a publish failure after the push warns only.

**Outcome**: In a repo with no matching tags, `mint release` produces `0.0.1` end-to-end (computed version, gates passed, `"Initial release."` recorded, annotated tag, `git push --atomic`, GitHub release created). `-m` → `0.1.0`, `-M` → `0.0.1`… i.e. `1.0.0` for major. A publish failure after a successful push warns only (the tag is already public); a pre-push failure aborts.

**Do**:
- Create `cmd/mint/main.go` (or a `cmd` package) wiring a CLI. Implement `mint release [bump]` with flags: `-p/--patch` (default), `-m/--minor`, `-M/--major`, `-y/--yes`. (`--set-version`, `-d/--dry-run`, `--no-ai`, `--autostash`, `--any-branch` are later phases — do not implement their behaviour now; you may reserve `mint version` and the `release` subcommand surface but only `mint release` needs to work.)
- Implement the orchestrator running the spine in order:
  1. Resolve repo root (task 1-4); load config (task 1-2).
  2. Resolve release branch (task 1-4); determine current version + compute next from the bump (task 1-3).
  3. Run preflight: fetch `--tags`, then local gates (clean tree, on branch, tag-free local — task 1-5) and network gates (remote sync, tag-free remote — task 1-6). Order per spec: cheap local first, then network. Abort cleanly on any gate failure (surfaced via Presenter).
  4. Build the notes body: first-release path → `"Initial release."` (task 1-9), no AI.
  5. Show the plan + notes via the Presenter; honour the review gate unless `-y` (Phase 1 first-release gate is `y`/`n`/`e`; `n` aborts — for Phase 1, abort = surface and stop, full auto-unwind is Phase 4).
  6. Record: write changelog + bookkeeping commit (task 1-9).
  7. If `publish = true`: run the `gh` install+auth gate (task 1-8) — **before** the tag.
  8. Create the annotated tag and `git push --atomic` (task 1-10). Push success = PONR crossed.
  9. If `publish = true`: `Publisher.CreateRelease` with the tag, title, and body (task 1-8).
- Apply the PONR asymmetry: any failure in steps 1–8 before the push succeeds → abort (surface, stop). A `CreateRelease` failure in step 9 (after a successful push) → **warn only** via the Presenter ("tag is already published; heal with regenerate --reuse"), exit without unwinding.
- Wire the recording Presenter in tests to assert the full event sequence and the warn-only publish-failure case; wire the `FakeRunner` to script git/gh.

**Acceptance Criteria**:
- [ ] In a no-tags repo, `mint release` resolves `0.0.0` → `0.0.1` and runs the full spine to a created GitHub release.
- [ ] `-m` → `0.1.0`; `-M` → `1.0.0`; no flag (default) → `0.0.1`.
- [ ] The pipeline runs in spine order: version → preflight (local then network) → first-release body → record → gh gate (if publishing, before tag) → tag → atomic push → publish.
- [ ] A pre-push gate/step failure aborts cleanly and nothing is tagged/pushed/published.
- [ ] A publish failure **after** a successful push warns only (post-PONR) and does not unwind.
- [ ] With `publish = false`, no `gh` gate and no publish; the run ends at a successful tag + atomic push.
- [ ] The whole flow is exercised with the `FakeRunner` and `RecordingPresenter`; no real git/gh/claude is invoked in tests.

**Tests**:
- `"mint release on a no-tags repo produces 0.0.1 end-to-end and creates the GitHub release"`
- `"-m produces 0.1.0 and -M produces 1.0.0"`
- `"default bump produces 0.0.1"`
- `"a failing preflight gate aborts before any mutation"`
- `"a publish failure after a successful push warns only and does not unwind"`
- `"publish = false ends at a successful atomic push with no gh gate or publish"`
- `"the spine runs stages in order (version, preflight, record, gh gate, tag, push, publish)"`

**Edge Cases**:
- `-m` → `0.1.0`, `-M` → `1.0.0`, default → `0.0.1`.
- Publish failure after push → warn only (post-PONR).
- `publish = false` → tag + push only, no gh gate.

**Context**:
> Release Lifecycle spine (strict order): Version → Preflight → Project prep → Release notes → Record → Make official (tag + `git push --atomic`) → Publish. Invariants: "Everything before stage 6 is local-only and recoverable … `git push --atomic` (stage 6) is the single point of no return … After the point of no return, mint never unwinds … Failures in stage 7 warn and point to the heal path." Failure model: "Push succeeds but provider release create fails — The tag is already public, so mint never unwinds … mint warns and points to the heal path: `regenerate --reuse`." First-release: "`mint release` → `0.0.1`, `-m` → `0.1.0`, `-M` → `1.0.0`." Phase 1 surfaces failures and stops; the full surgical auto-unwind, `--autostash` restore ordering, and lock resilience are Phase 4. `--set-version`/`--dry-run`/`--no-ai`/`--any-branch` are later phases.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Release Lifecycle (the spine)", "Stages 6–7 → Failure model", "Stage 1 → Bump selection", "CLI Surface & Flags".
