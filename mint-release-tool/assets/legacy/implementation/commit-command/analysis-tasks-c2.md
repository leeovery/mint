---
topic: commit-command
cycle: 2
total_proposed: 4
---
# Analysis Tasks: Commit Command (Cycle 2)

## Task 1: Structurally single-source the per-mode git source selection shared by the preflight probes and the L1 diff sources
status: approved
severity: high
sources: duplication, architecture

**Problem**: The same three per-mode git argv "shapes" are encoded independently in two files and must stay byte-aligned by hand. `run.go`'s preflight probe builders (`stagedProbeArgs`/`trackedProbeArgs`/`untrackedProbeArgs`, run.go:739-749) emit `["diff","--cached","--name-only","--","."]`+excludes, `["diff","HEAD","--name-only","--","."]`+excludes, `["ls-files","--others","--exclude-standard","--","."]`+excludes; `generate.go`'s L1 sources (`stagedDiff`/`trackedWorktreeDiff`/`untrackedFiles`, generate.go:168,185,239) emit the identical argv minus `--name-only`. The base verb/refspec, the `-- .` selector, and the `:(exclude)` excludes tail are duplicated across the two files. On top of that, the StagingMode dispatch is also written twice: `wouldStageNothing` (run.go:715-732) and `sourceDiff` (generate.go:144-153) each switch on the mode with the same three-way structure and the same AddAll "tracked first, short-circuit, else untracked" composition. The code's own comments (run.go:734-738, 700-712, 698-712) state the preflight argv must be "identical to generate's argv" and that each probe branch "mirrors" its generate counterpart. The spec makes "the preflight and the AI's L1 diff read ONE exclusion-filtered source and cannot drift" a load-bearing invariant, but it currently rests on two copies agreeing — a swap of `git diff HEAD` for `git diff --merge-base HEAD`, a reorder of the exclude tail, or a change to AddAll source semantics silently breaks the guarantee and makes the preflight emptiness verdict diverge from what the AI actually sees.

**Solution**: Factor the per-mode source selection into one place both consumers derive from. Introduce a single set of unexported per-mode base argv builders (e.g. `stagedBaseArgs`/`trackedBaseArgs`/`untrackedBaseArgs`) returning the `[verb, refspec/flags, "--", "."]` prefix, with `excludePathspecs(...)` appended by callers as today. The generate L1 sources use the prefix as-is; the preflight probe builders derive their argv by appending `--name-only` to the same prefix for the diff cases (the untracked `ls-files` prefix is shared verbatim). Likewise centralise the per-mode dispatch description (which sources/probes compose for each StagingMode, including the AddAll tracked-then-untracked short-circuit) so both `wouldStageNothing` and `sourceDiff` derive their mode behaviour from one mode descriptor rather than two parallel switches. Colocate the new shared selection helpers with the empty-staging preflight cluster by moving that cluster into a dedicated `preflight.go` (the three sentinel errors `errNothingToCommit`/`errNoChangesStaged`/`errNoTrackedChanges`, `checkSomethingToCommit`, `wouldStageNothing`, the `*ProbeArgs` builders, `emptyStagingError`, `gitOutputEmpty`), leaving `run.go` as the orchestration spine. Preserve the AddAll probe's deliberate `--name-only` lightness (it must not render each untracked file's body) — only the source-selection logic is single-sourced, not the probe's cost optimisation.

**Outcome**: The per-mode source-command prefix, the `-- .` selector, and the AddAll composition rule each live in exactly one place; the preflight probe argv is provably the same exclusion-filtered source as the L1 diff (differing only by the appended `--name-only`), so the "one source, cannot drift" invariant is structural rather than comment- and test-enforced. The preflight subsystem is colocated in `preflight.go` with the shared selection helper, and `run.go` is reduced to orchestration.

**Do**:
1. In `internal/commit`, add unexported per-mode base argv builders returning the `[verb, refspec/flags, "--", "."]` prefix for staged (`diff --cached`), tracked (`diff HEAD`), and untracked (`ls-files --others --exclude-standard`). Keep `excludePathspecs(...)` appended by callers so the exclusion tail stays single-sourced as it already is.
2. Rewrite the generate L1 sources (`stagedDiff`, `trackedWorktreeDiff`, `untrackedFiles`, and the AddAll composition in `sourceDiff`/`addAllWorktreeDiff`) to build their argv from the shared prefixes plus excludes — no re-spelling of the verb/refspec or `-- .`.
3. Rewrite the preflight probe builders (`stagedProbeArgs`, `trackedProbeArgs`, `untrackedProbeArgs`) to derive from the same shared prefixes, appending `--name-only` for the two diff cases only.
4. Introduce one mode-descriptor (or equivalent) that both `wouldStageNothing` and `sourceDiff` consume so the StagingMode→sources mapping (including the AddAll tracked-then-untracked short-circuit) is defined once.
5. Move the empty-staging preflight cluster from `run.go` into a new `internal/commit/preflight.go`: the three sentinel errors, `checkSomethingToCommit`, `wouldStageNothing`, the `*ProbeArgs` builders, `emptyStagingError`, `gitOutputEmpty`, plus the new shared selection helper. Leave `run.go` holding only the orchestration spine.
6. Run `go vet ./...`, `golangci-lint run`, and the commit package tests; confirm clean.

**Acceptance Criteria**:
- Each per-mode git argv prefix and the `-- .` selector is spelled exactly once and consumed by both the preflight probe and the L1 source.
- The preflight probe argv is the shared prefix plus `--name-only` (diff cases) / the shared `ls-files` prefix (untracked case), with no independently re-spelled verb/refspec/selector.
- The StagingMode→sources mapping, including the AddAll tracked-then-untracked short-circuit, is defined in one place consumed by both the emptiness path and the diff path.
- The empty-staging preflight cluster (3 sentinels, `checkSomethingToCommit`, `wouldStageNothing`, `*ProbeArgs`, `emptyStagingError`, `gitOutputEmpty`) plus the shared selection helper live in `internal/commit/preflight.go`; `run.go` no longer contains them.
- The AddAll probe still uses `--name-only` (does not render each untracked file body).
- `go vet ./...`, `golangci-lint run`, and the commit tests pass; the verbatim empty-staging messages and per-mode emptiness verdicts are unchanged.

**Tests**:
- Existing preflight/emptiness tests and the generate L1 source tests continue to pass unchanged, confirming behaviour parity after the refactor.
- A test asserting the preflight probe argv for each mode equals the corresponding L1 source argv plus `--name-only` (diff cases) / equals the shared untracked prefix — making the "same source" property checkable against the single shared builder rather than two copies.
- A test exercising each StagingMode (All, AddAll, default/staged) confirming the emptiness verdict and the L1 source agree (both empty / both non-empty) through the shared mode descriptor, including the AddAll tracked-then-untracked short-circuit.

## Task 2: Factor the read-only "run git, return stdout or wrapped error" idiom on the generate side
status: skipped
severity: low
sources: duplication

**Problem**: `stagedDiff` (generate.go:167-174) and `trackedWorktreeDiff` (generate.go:184-191) are near-identical 8-line bodies: build `append([]string{...}, excludePathspecs(...)...)`, call `g.runner.Run(ctx,"git",args...)`, on error `return "", fmt.Errorf("running git <cmd>: %w", err)`, else `return res.Stdout, nil`. Only the literal argv prefix and the error-string command name differ. `untrackedFiles` (generate.go:238-252) shares the same run+wrap opening before its line-split. The probe side already factors exactly this "run read-only git, wrap on error" pattern as `gitOutputEmpty` (run.go:787-793), but the generate side hand-rolls the wrap idiom three more times, risking inconsistent error wording as sources are added.

**Solution**: Add a single read-only helper on `Generator` mirroring `gitOutputEmpty` but returning stdout instead of an emptiness bool — i.e. "run git with the given argv, return `res.Stdout` or a wrapped error". Collapse `stagedDiff` and `trackedWorktreeDiff` to an argv build plus one call to the helper; have `untrackedFiles` reuse it for its raw stdout before the line-split. Keep the existing wrap-string format (`running git <cmd>: %w`) so error wording is preserved and centralised.

**Outcome**: The read-only git run+wrap pattern exists once on the generate side (paralleling the probe side's `gitOutputEmpty`), the L1 source helpers shrink to argv-build + one call, and error wording for read-only git failures is single-sourced.

**Do**:
1. Add an unexported method on `Generator` (e.g. `gitOutput(ctx, args...) (string, error)`) that runs `g.runner.Run(ctx,"git",args...)` and returns `res.Stdout` or `fmt.Errorf("...: %w", err)` with the wrap wording matching the current helpers.
2. Rewrite `stagedDiff` and `trackedWorktreeDiff` to build their argv (from the Task-1 shared prefixes if Task 1 has landed, otherwise as-is) and call the helper.
3. Rewrite `untrackedFiles` to obtain raw stdout via the helper, then perform its existing line-split.
4. Run `go vet ./...`, `golangci-lint run`, and the commit tests; confirm clean.

**Acceptance Criteria**:
- One read-only git run+wrap helper exists on `Generator`; `stagedDiff`, `trackedWorktreeDiff`, and `untrackedFiles` all route their git read through it.
- Error wording for read-only git failures on the generate side is produced in one place and unchanged in format from the current `running git <cmd>: %w` strings.
- `go vet ./...`, `golangci-lint run`, and the commit tests pass.

**Tests**:
- Existing generate L1 source tests (success and git-error paths) continue to pass, confirming stdout and wrapped-error behaviour is unchanged after centralisation.

## Task 3: Consolidate near-duplicate commit test scaffolding (Deps builders and the git-invocation filter wrapper)
status: skipped
severity: low
sources: duplication

**Problem**: The package carries parallel test scaffolding authored per-file that has already drifted. `regenDeps` (run_regen_test.go:50-59) is `newCommitDeps` (run_test.go:61-69) plus two deltas (a no-op `WithBackoff` on the Mutator and `StdinInteractive:true`); `regenFailDeps` (run_regen_fallback_test.go:55) is the same base shape over an editorRunner with a mode parameter. Each re-lists the full `commit.Deps` literal (Presenter/Runner/Mutator/Transport/Root), and as `Deps` grows (it already carries Staging/NoAI/Yes/StdinInteractive/Push) every builder must be touched in parallel — they have already diverged, since only `regenDeps` sets `StdinInteractive`/`WithBackoff`. Separately, `gitInvocations` (run_test.go:99-101) and `gitInvocationsGen` (generate_test.go:96-98) have byte-identical bodies (`return gitInvocationsOf(r.Invocations())`) under two names: the shared core `gitInvocationsOf` was correctly extracted, but the trivial `*runner.FakeRunner` wrapper was copied rather than shared.

**Solution**: Make `regenDeps` and `regenFailDeps` start from `newCommitDeps` and override only their deltas (set `StdinInteractive`, swap the Mutator backoff, set Staging/mode), or give `newCommitDeps` a functional-option / struct-override form so the regen variants express only what differs — keeping one canonical `Deps` shape for the package's tests. Drop `gitInvocationsGen` and have `generate_test.go` call the existing `gitInvocations` (or `gitInvocationsOf` directly).

**Outcome**: One canonical commit `Deps` builder that the regen variants derive from (so future `Deps` fields are added in one place and the builders cannot silently drift), and a single git-invocation filter wrapper used across both test files.

**Do**:
1. Refactor `newCommitDeps` to support overrides (functional options or a base struct the caller mutates) without changing its current default behaviour.
2. Rewrite `regenDeps` and `regenFailDeps` to start from `newCommitDeps` and set only their deltas (`StdinInteractive`, Mutator `WithBackoff`, Staging, editorRunner/mode), removing the full re-listed `Deps` literals.
3. Delete `gitInvocationsGen` and update `generate_test.go` to call `gitInvocations` (or `gitInvocationsOf`).
4. Run the commit package tests; confirm all pass.

**Acceptance Criteria**:
- `regenDeps` and `regenFailDeps` no longer re-list the full `Deps` literal; they derive from `newCommitDeps` and express only their deltas.
- `gitInvocationsGen` is removed; `generate_test.go` uses the shared `gitInvocations`/`gitInvocationsOf`.
- The commit package tests pass with no behavioural change to the test outcomes.

**Tests**:
- The full commit package test suite passes unchanged (the consolidation is test-only; existing assertions confirm the derived Deps and shared invocation filter behave identically).

## Task 4: Derive commit's review gate from the shared NotesReviewGate constructor instead of re-declaring it
status: skipped
severity: low
sources: architecture

**Problem**: `commitReviewGate` (run.go:633-646) is byte-identical to `presenter.NotesReviewGate` (presenter/gate.go:143-159) except for `Subject` ("message" vs "notes") — the same `Question` ("Continue?"), `AcceptEcho` ("accepted"), `Default` (`ChoiceYes`), and the same four-choice y/n/e/r `Choices` slice are hand-copied. The documented reason for not importing directly (the `-y` echo must read "message: accepted", not "notes: accepted") is valid, but the entire gate is re-declared to override one string field. `Gate` is a plain value type, so if the shared gate's choice vocabulary evolves (e.g. a new action key), commit's hand-built copy silently drifts out of alignment with the notes gate it is meant to mirror.

**Solution**: Derive commit's gate from the shared constructor and override only the differing field — e.g. `g := presenter.NotesReviewGate(); g.Subject = "message"; return g` — or add a small parameterised constructor on the presenter that takes the `Subject`. Either keeps the y/n/e/r choice vocabulary single-sourced while preserving commit's distinct "message" Subject and its `-y` echo wording.

**Outcome**: The review-gate choice vocabulary (Question, AcceptEcho, Default, Choices) is single-sourced via `NotesReviewGate`; commit only overrides `Subject`, so future changes to the shared gate's choices automatically flow to commit and the two gates cannot drift.

**Do**:
1. Choose the lower-churn approach: either have `commitReviewGate` call `presenter.NotesReviewGate()` and set `g.Subject = "message"` before returning, or add a parameterised presenter constructor accepting the Subject and call it from both consumers.
2. Remove the hand-copied Question/AcceptEcho/Default/Choices from `commitReviewGate`.
3. Confirm the `-y` echo still reads "message: accepted" and the gate offers the same y/n/e/r choices.
4. Run `go vet ./...`, `golangci-lint run`, and the commit tests; confirm clean.

**Acceptance Criteria**:
- `commitReviewGate` no longer hand-copies the Question/AcceptEcho/Default/Choices fields; the choice vocabulary comes from `NotesReviewGate` (directly or via a shared parameterised constructor).
- Commit's gate `Subject` is "message" and the `-y` echo reads "message: accepted".
- The gate offers the same y/n/e/r choices as the notes gate.
- `go vet ./...`, `golangci-lint run`, and the commit tests pass.

**Tests**:
- Existing commit review-gate tests (choice handling and the `-y` "message: accepted" echo) pass unchanged, confirming the derived gate matches the previously hand-built one.
