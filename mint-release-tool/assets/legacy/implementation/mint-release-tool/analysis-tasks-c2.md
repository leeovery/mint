---
topic: mint-release-tool
cycle: 2
total_proposed: 3
---
# Analysis Tasks: mint-release-tool (Cycle 2)

## Task 1: Extract shared changelog push/recovery tail for the single-version and batch regenerate paths
status: approved
severity: medium
sources: duplication

**Problem**: `writeAndPushChangelog` (internal/engine/regenerate_write.go:231-273) and `commitAndPushRebuild` (internal/engine/regenerate_batch_changelog.go:146-165) independently sequence the same regenerate-changelog write/push/recovery shape: capture the clean starting HEAD, stage+commit CHANGELOG.md, plain-push via the literal `git push origin HEAD` as the point of no return (narrated with the same `emitBlockingStageStarted("push")` spinner), and route any pre-push failure through `resetAndAbort`. The only real differences are the commit subject and the no-op short-circuit. Because the push form and PONR/reset semantics are load-bearing, the two copies risk drifting.

**Solution**: Extract the shared "plain-push as PONR, narrate the blocking push stage, reset-on-abort" tail into a single engine helper that both functions call after producing their respective commit — e.g. `pushChangelogCommit(ctx, deps, startingHEAD) error`. Keep the two distinct commit subjects and the differing no-op short-circuit at the call sites.

**Outcome**: The `git push origin HEAD` literal, the blocking "push" narration, and the `resetAndAbort` wiring for the PONR push live in exactly one place; both regenerate paths invoke it; the two paths can no longer drift on push form or recovery semantics.

**Do**:
1. Define `pushChangelogCommit(ctx, deps ReleaseDeps, startingHEAD string) error` in the engine package, encapsulating: `emitBlockingStageStarted(deps.Presenter, "push")`, the `git push origin HEAD` mutation, `resetAndAbort(...)` on push failure (with the existing `pushing regenerated changelog: %w` error wrap), and the push `StageSucceeded` on success.
2. In `writeAndPushChangelog`, after the `committed`/no-op checks, replace the inline push+reset block with a call to `pushChangelogCommit`, preserving the `(pushed bool, err error)` contract.
3. In `commitAndPushRebuild`, after the stage+commit, replace the inline push+reset block with a call to `pushChangelogCommit`.
4. Preserve each call site's distinct commit subject and short-circuit logic; do not move the stage/commit construction into the helper.

**Acceptance Criteria**:
- The `git push origin HEAD` literal, the blocking "push" StageStarted/StageSucceeded narration, and the PONR `resetAndAbort` wiring appear in exactly one engine function.
- Both `writeAndPushChangelog` and `commitAndPushRebuild` route their PONR push through that single helper.
- `writeAndPushChangelog` still returns `pushed=false` for the no-op case and `pushed=true` after a successful push; `commitAndPushRebuild` still returns nil after a successful push.
- The two commit subjects and the differing no-op short-circuit remain at the call sites.

**Tests**:
- Single-version regenerate: successful path pushes via `git push origin HEAD` and reports pushed=true; existing commit-subject + push-command assertions still pass.
- Single-version regenerate: pre-push failure resets the local commit to the captured starting HEAD and surfaces a "push" StageFailed (no StageSucceeded).
- Batch (`--all`) regenerate: successful end-of-batch push and pre-push-failure reset both behave identically through the shared helper.
- No-op single-version write makes no commit and pushes nothing (short-circuit still at the call site).

## Task 2: Close the release-success footer URL seam — feed the real release URL into RunResult.URL
status: approved
severity: low
sources: architecture

**Problem**: The presenter exposes a `RunResult.URL` field with full rendering in both modes (pretty: `{leaf} released {project} v{X} · {url}`; plain: `done: {project} v{X} {url}`), but the forward release path hardcodes `releaseURL := ""` (internal/engine/release.go:624) and feeds that empty string into `RunResult.URL` at release.go:646. `Publisher.CreateRelease`/`UpdateRelease` return only `error`, and `GitHubPublisher` discards the runner Result (`_, err := p.runner.RunWith(...)` at internal/publish/publish.go:88), throwing away the release URL that `gh release create` prints to stdout. The entire `· {url}` / `done: ... {url}` render branch is dead in production — every real release closes with an empty URL segment, yet the seam is plumbed end to end (field, both renderers, tests). The presentation layer deliberately built the URL footer, so the intent was to show it — the seam just stops one layer short at the Publisher boundary.

**Solution**: RESOLVED to OPTION (a) — CLOSE the seam (do NOT drop the field; the presenter built the footer on purpose). Change `Publisher.CreateRelease`/`UpdateRelease` to return the created/updated release URL from the `gh` stdout the driver already discards, and thread it into `RunResult.URL` so the rendered footer is actually fed.

**Outcome**: A successful real release renders a non-empty URL footer sourced from the publisher's actual `gh` output, in both presenter modes. A warn-only post-PONR publish failure does NOT render a bogus URL (URL stays empty on that path). A downgrade/no-publish run renders no URL (nothing was published).

**Do**:
1. Change the `Publisher` interface: `CreateRelease`/`UpdateRelease` return `(url string, err error)` instead of just `error`.
2. In `GitHubPublisher`, capture the runner `Result` from `RunWith` (currently discarded at publish.go:88) and extract the created release URL from its stdout (`gh release create`/`gh release edit` print the release URL to stdout — `TrimSpace` it; if stdout is empty, return an empty URL, not an error). Update the `UpdateRelease` (gh release edit) site equivalently.
3. Update ALL `Publisher` implementations, the dispatch helper (`engine.DispatchRelease` — it must now return/propagate the URL), and all fake Publishers in tests to the new signature.
4. In `internal/engine/release.go`, replace the hardcoded `releaseURL := ""` with the URL returned from the publish dispatch on the success path, and thread it into `RunResult.URL` at the `RunFinished` site (release.go:646). On the warn-only publish-failure path and the downgrade/no-publish path, leave `releaseURL` empty.
5. Keep the presenter `RunResult.URL` field and both render branches (pretty.go:925-929, plain.go:482-486) — they are now genuinely fed.

**Acceptance Criteria**:
- `Publisher.CreateRelease`/`UpdateRelease` return the release URL; `GitHubPublisher` parses it from the `gh` stdout it previously discarded.
- A successful real release threads the publisher's URL into `RunResult.URL`, and the footer renders the non-empty URL in both pretty and plain modes.
- A warn-only post-PONR publish failure renders NO URL (empty), and a downgrade/no-publish run renders no URL.
- All Publisher implementations, `DispatchRelease`, and fakes compile against the new signature; the full suite passes.

**Tests**:
- Publisher driver test: asserts the release URL is parsed from the `gh` stdout and returned (create AND update paths).
- Engine test: asserts the publisher URL reaches `RunResult.URL` on a successful release; asserts empty URL on the warn-only publish-failure path and the downgrade path.
- Presenter tests: assert the populated URL footer renders in both modes (extend/keep the existing URL render tests).

## Task 3: Remove the orphaned Phase-1 presenter mappers (EmitPlan/EmitStageFailed/EmitNotes/EmitWarning)
status: approved
severity: low
sources: architecture

**Problem**: `EmitPlan`, `EmitStageFailed`, `EmitNotes`, and `EmitWarning` (internal/engine/engine.go:123-154) are exported thin event->method mappers whose own doc comments describe them as "thin Phase 1 event->method mappers proving the adoption is real." The real orchestrator (release.go) calls `p.ShowPlan` / `p.StageFailed` / `p.ShowNotes` / `p.Warn` directly and never routes through these helpers — they have zero production callers; the only references are in engine_test.go:153-212. Dead exported API that invites a future caller onto a deprecated path and adds noise to the engine's public boundary. (`ReviewDecision` and `FirstReleaseReviewGate` in the same file are live and out of scope.)

**Solution**: Remove the four `Emit*` functions and their dedicated tests, since the orchestrator already calls the presenter methods directly.

**Outcome**: The engine's exported surface no longer carries the four dead `Emit*` mappers; the only paths to the presenter for plan/stage-failed/notes/warning are the orchestrator's direct calls; no test exists solely to exercise dead production code.

**Do**:
1. Confirm via search that `EmitPlan`, `EmitStageFailed`, `EmitNotes`, `EmitWarning` have no callers outside engine_test.go.
2. Delete the four functions and their doc comments from internal/engine/engine.go:123-154.
3. Delete their dedicated tests from internal/engine/engine_test.go:153-212.
4. Leave `ReviewDecision`, `FirstReleaseReviewGate`, `emitGateSucceeded`, `emitBlockingStageStarted`, and the unexported helpers untouched.

**Acceptance Criteria**:
- `EmitPlan`, `EmitStageFailed`, `EmitNotes`, and `EmitWarning` no longer exist in the engine package.
- No reference to them remains in production or test code.
- The orchestrator's direct `ShowPlan` / `StageFailed` / `ShowNotes` / `Warn` calls are unchanged and the release flow still emits plan, stage-failure, notes, and warning events as before.
- The package builds and the full engine test suite passes after removal.

**Tests**:
- The engine test suite compiles and passes with the four `Emit*` tests removed (no orphaned references).
- Existing release-flow / presenter-interaction tests still assert that plan, stage-failed, notes, and warning events reach the presenter via the orchestrator's direct calls.
