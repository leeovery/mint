---
topic: mint-release-tool
cycle: 1
total_proposed: 5
---
# Analysis Tasks: mint-release-tool (Cycle 1)

## Task 1: Wire a per-run Regenerator on the regenerate fresh path so the rendered `[r]` choice works
status: pending
severity: high
sources: architecture

**Problem**: The fresh regenerate path (`mint release regenerate <ver> --fresh` and `--all --fresh`) runs the four-choice notes-review gate via `presenter.NotesReviewGate()`, which renders `[y] [n] [e] [r]`. The `[r]` (regenerate-with-context) choice is offered for `KindNormalAI`, but `runRegenerate` (cmd/mint/main.go:122) builds `engine.ReleaseDeps` WITHOUT a `Regenerator`, and the regenerate engine paths (`RegenerateRun`/`RegenerateWrite`/`RegenerateAll`) never build one — unlike the forward `Release` orchestrator, which constructs a per-run regenerator via `perRunRegenerator(...)`. So `deps.Regenerator` is nil on every interactive fresh regenerate. When the user picks `r`, `regenerateBody` hits `regen == nil` and surfaces `errRegeneratorUnavailable` and aborts. The asymmetry is sharp: the `e` (edit) choice on the SAME gate IS wired (main.go:128 sets `Editor`), so `e` works and `r` does not, even though the spec (lines 448/454/533) presents them as peers. Guaranteed user-facing abort on a rendered, user-reachable choice.

**Solution**: Wire a per-run `Regenerator` on the regenerate fresh path the same way the forward path does. The fresh body producer (`newRegenerateBodyProducer` / `newBatchBodyProducer`) already builds the notes Generator from `(r, cfg, root, res)`; bind its `GenerateWithContext` over the resolved diff range into `deps.Regenerator` (or pass a per-run regenerator into `RegenerateWrite`/`gatePerVersion`) so `r` re-runs the same fresh AI path with the one-time context appended. Spec-faithful because the spec lists `r` for fresh regenerate.

**Outcome**: On an interactive `mint release regenerate <ver> --fresh` (and `--all --fresh`), selecting `[r]` re-runs the fresh AI generation with the user-supplied one-time context appended over the resolved `vX-1..vX` range, returning to the review gate with the regenerated body — never aborting with `errRegeneratorUnavailable`. `r` and `e` behave as peers.

**Do**:
1. Inspect how the forward path constructs its per-run regenerator (`perRunRegenerator(...)` in internal/engine/release.go ~1337-1350) and how `reviewGate(ctx, p, deps.Editor, deps.Regenerator, notes.KindNormalAI, ...)` consumes it.
2. In the fresh regenerate body producers (`newRegenerateBodyProducer` / `newBatchBodyProducer`), build a regenerator calling the constructed notes Generator's `GenerateWithContext` bound over the resolved diff range for that version.
3. Thread that per-run regenerator into the deps used by the fresh regenerate gate (populate `deps.Regenerator` for the run, or pass into `RegenerateWrite`/`gatePerVersion`). Match the forward path's wiring shape so the same `reviewGate` call receives a non-nil regenerator.
4. Confirm `runRegenerate` no longer leaves `Regenerator` unset for the fresh interactive path, or that the engine builds it per-run.
5. Do not change y/n/e behaviour or the gate rendering — only supply the backing for `r`.

**Acceptance Criteria**:
- Selecting `[r]` on the fresh regenerate notes-review gate (single and `--all`) re-runs the fresh AI generation with one-time context appended and returns to the gate with the new body.
- `errRegeneratorUnavailable` is no longer reachable on any interactive fresh regenerate path.
- The forward `Release` path's regenerate behaviour is unchanged.
- The `e` (edit) choice continues to work on the regenerate fresh gate.

**Tests**:
- Interactive fresh single-version regenerate, user selects `r`: asserts the per-run regenerator's `GenerateWithContext` is invoked over the `vX-1..vX` range with the supplied context and the gate re-renders (no abort).
- Interactive fresh `--all` regenerate, user selects `r` on a version: asserts the per-version regenerator is wired and invoked (no `errRegeneratorUnavailable`).
- Forward `Release` regenerate `r` path remains green (regression guard).

## Task 2: Apply the degenerate-diff guard on the regenerate fresh path
status: pending
severity: medium
sources: standards

**Problem**: The spec's "Degenerate release" guard (specification.md:258-262) states mint "does not call the AI" when the post-`diff_exclude` diff is empty/whitespace-only — "an empty diff is the one input it will reliably hallucinate on" — and writes the `StubBody` instead. The FORWARD path enforces this via `notes.Selector.SelectBody` -> `IsDegenerate(diff)` before the AI generator (internal/notes/select.go:160-161). The regenerate FRESH path does NOT route through `SelectBody`: `RegenerateFreshBody` calls `generator.GenerateFromRange` directly, which assembles the range diff then proceeds straight to `CheckDiffSize`/`transport.Generate` (internal/notes/generate.go:102-108, 136-157) with no `IsDegenerate` check. The fresh path is MORE exposed: its range always contains mint's release-bookkeeping commit, so a version whose only non-excluded change was the bookkeeping yields an empty post-exclusion diff that reaches the AI. The `--all` skip-and-continue net does NOT cover this: a degenerate diff is not an AI *failure* (the AI hallucinates a body on success), so it is never skipped.

**Solution**: Apply the same `notes.IsDegenerate` short-circuit the forward path uses: after `AssembleRange` produces the post-exclusion range diff and before the AI call, if `IsDegenerate(diff)` return `notes.StubBody()` with no AI invocation. Cleanest reuse is inside `GenerateFromRange` (or a thin wrapper `RegenerateFreshBody` calls) so both single (regenerate_run.go:64) and batch (regenerate_all.go:89) fresh producers inherit it.

**Outcome**: An empty/all-excluded `vX-1..vX` fresh range diff yields `notes.StubBody()` with zero transport/AI calls, on both single and `--all` fresh paths — matching the forward path and the spec's path-agnostic "never run the AI on an empty diff" rationale.

**Do**:
1. Examine the forward degenerate branch in `SelectBody` (internal/notes/select.go:160-161) and `notes.IsDegenerate`/`notes.StubBody()` for the exact predicate and stub value.
2. Locate where `GenerateFromRange` assembles the post-`diff_exclude` range diff (internal/notes/generate.go:102-108) after `AssembleRange`.
3. Insert the `IsDegenerate(diff)` check after the post-exclusion diff and before `CheckDiffSize`/`transport.Generate`; on degenerate return `notes.StubBody()` with no AI call. Place it so both single and batch fresh producers inherit it (inside `GenerateFromRange` or a thin shared wrapper).
4. Preserve existing non-degenerate behaviour unchanged.

**Acceptance Criteria**:
- An empty/whitespace-only post-`diff_exclude` `vX-1..vX` fresh diff produces `notes.StubBody()` and makes no transport/AI call, on both single and `--all` fresh paths.
- A non-degenerate fresh diff still proceeds through `CheckDiffSize` and `transport.Generate` exactly as before.
- The degenerate guard is one shared rule reached by both fresh producers, not duplicated forward-only.

**Tests**:
- Fresh single-version regenerate with an empty/all-excluded `vX-1..vX` range: asserts `StubBody` returned, zero transport calls.
- Fresh `--all` regenerate with a degenerate version: asserts that version yields `StubBody` with no AI call and the batch continues.
- Fresh regenerate with a non-degenerate diff still invokes transport (regression guard).

## Task 3: Emit StageStarted / StageSucceeded around the release and regenerate stages
status: pending
severity: medium
sources: architecture

**Problem**: The Presenter contract declares `StageStarted` and `StageSucceeded` (consumed per Dependencies line 681), and the presentation side built+tested a blocking-stage spinner that animates between a blocking `StageStarted` and its completion. But the engine orchestrator emits `RunStarted`, `ShowPlan`, `ShowNotes`, `Prompt`, `Warn`, `StageFailed`, `Unwound`, `RunFinished` — `StageStarted`/`StageSucceeded` have ZERO production call sites in `internal/engine` or `cmd/` (referenced only in the presenter's golden-transcript test). Consequences: (1) a real run shows no per-stage narration; (2) long stages (AI notes ~60s, `pre_tag` build hooks, atomic push) run with no progress indicator — the exact use case `StageStart.Blocking` + the spinner were built for; (3) the editor's `SuspendSpinner`/`ResumeSpinner` bracket (editor.go:103-104) presumes a live spinner, so it is permanently a no-op. The presenter half was built to a contract the engine half never drives.

**Solution**: Have the release (and regenerate) orchestrators emit `StageStarted`/`StageSucceeded` around stages they already sequence — at minimum the blocking ones (notes generation, `pre_tag` hook, push) with `Blocking:true` so the spinner activates, and ideally the cheap read-only gates (version, preflight) for completion narration. The payload structs (`StageStart`, `StageSuccess` with engine-measured `Elapsed`) already exist; this is wiring emit calls into the existing sequence.

**Outcome**: A real `mint release` (and regenerate) run narrates each stage and animates the spinner during blocking stages, with engine-measured `Elapsed` on completion; the editor's suspend/resume bracket suspends a genuinely active spinner.

**Do**:
1. Map the existing release stage sequence (internal/engine/release.go:279-617) and the regenerate sequence (internal/engine/regenerate_interactive.go:134-170).
2. Confirm the `StageStart`/`StageSuccess` payloads, the Presenter methods (internal/presenter/presenter.go:64-79), and the spinner blocking behaviour (internal/presenter/spinner.go:29-30).
3. Emit `StageStarted` (Blocking:true) before each blocking stage — notes generation, `pre_tag` hook, push — and `StageSucceeded` (engine-measured `Elapsed`) after each completes.
4. Emit `StageStarted`/`StageSucceeded` for the cheap read-only gates (version, preflight) for completion narration (non-blocking acceptable).
5. Ensure `StageStarted` fires around the editor invocation so the existing suspend/resume bracket wraps a live spinner.
6. Do not alter `StageFailed`/`Unwound` emission or existing plan/notes/gate/final-line output.

**Acceptance Criteria**:
- The release orchestrator emits `StageStarted` (Blocking:true) + `StageSucceeded` around notes-generation, `pre_tag` hook, and push with engine-measured `Elapsed`.
- Read-only gates (version, preflight) emit completion narration.
- The regenerate orchestrator emits equivalent stage events for its sequenced stages.
- The spinner is active during a blocking stage; the editor suspend/resume bracket suspends a live spinner.
- Existing events unchanged.

**Tests**:
- A release run drives a fake Presenter and asserts `StageStarted`/`StageSucceeded` in order around notes generation, `pre_tag` hook, push, with `Blocking:true` on the blocking stages.
- A regenerate run asserts the equivalent stage events fire.
- The editor bracket asserts `SuspendSpinner` is called while a stage spinner is active (no longer a no-op).

## Task 4: Extract a single shared atomic-write helper and have all three sites delegate
status: pending
severity: medium
sources: duplication

**Problem**: Three independently-authored implementations of the same crash-safe write idiom (CreateTemp in target dir, Write, Close-with-cleanup, optional Chmod(0o644), Rename). record's two (`writeAtomic` at internal/record/changelog.go:219, `writeFileAtomic` at internal/record/versionfile.go:203) are in the SAME package, identical apart from temp prefix and error nouns; versionfile.go comments "It mirrors writeAtomic". notescache's copy (internal/notescache/cache.go:211) is the same algorithm minus Chmod with bare errors. Rule-of-Three: a ~25-line crash-safety primitive in triplicate that must stay correct in lockstep.

**Solution**: Extract ONE shared helper (e.g. internal `fsutil`/`atomicwrite` package exposing `WriteFile(path string, data []byte, perm fs.FileMode) error`) doing CreateTemp(dir)/Write/Close/Chmod/Rename with the established cleanup branches. All three sites delegate, passing their perm and wrapping the returned error with their domain noun at the call site.

**Outcome**: A single tested crash-safe atomic-write primitive; record's two writers and notescache's writer delegate to it; per-domain error wording preserved at call sites.

**Do**:
1. Read the three implementations and confirm the shared shape and where they differ (temp prefix, Chmod presence, error wrapping).
2. Create the shared helper doing CreateTemp(dir)/Write/Close/Chmod/Rename with `os.Remove` cleanup on each error branch.
3. Replace `record.writeAtomic`/`record.writeFileAtomic` bodies with calls to it (perm 0o644), wrapping errors with the existing domain noun ("changelog"/"version file") at the call site.
4. Replace notescache's writer body with a call to it, preserving observable error behaviour.
5. Keep each site's final written path and permissions unchanged.

**Acceptance Criteria**:
- A single shared helper performs the CreateTemp/Write/Close/Chmod/Rename idiom with cleanup-on-error.
- record's changelog + version-file writers and notescache's writer all delegate to it.
- Each site's observable error wording (domain noun) is preserved.
- Final written paths and file permissions (0o644 where previously applied) unchanged.

**Tests**:
- The shared helper writes atomically and removes the temp file when Write/Close/Rename fails (no leftover temp, target unchanged on error).
- record changelog + version-file writers still produce correct contents/permissions and existing domain error messages on failure.
- notescache writer still writes correctly via the shared helper.

## Task 5: Consolidate copied cross-boundary constants and the remote-URL reader to single owned symbols
status: pending
severity: low
sources: duplication

**Problem**: Four small cross-boundary duplications rely on a copied literal rather than a shared symbol: (1) the `git remote get-url origin` reader is line-for-line identical in `regenerateRemoteURL` (cmd/mint/main.go:191) and `remoteURL` (internal/engine/release.go:861); (2) the changelog date layout `"2006-01-02"` is defined twice — record owns it (internal/record/changelog.go:19 `dateLayout`) and engine redefines it to parse the historical date (internal/engine/regenerate_write.go:119 `regenerateDateLayout`); a mismatch silently breaks the no-empty-commit/no-data-loss guarantees; (3) `changelogFileName = "CHANGELOG.md"` is a const in both record (the writer, internal/record/changelog.go:16) and engine (which stages it, internal/engine/regenerate_changelog.go:41); (4) the parallel changelog-disabled validators (cmd/mint/regenerate_validate.go:57, internal/engine/regenerate_batch.go:118) each return the identical pinned literal "changelog is disabled in config".

**Solution**: For (1)-(3) have the owner export the canonical symbol and the mirror reference it, deleting the duplicate. For (4) keep BOTH validators (the concrete-enum split is correct and the engine enforces independently) but lift only the shared message into one exported sentinel both reference. Do NOT merge the validator functions or thread untyped values across the boundary.

**Outcome**: The remote-URL read, changelog date layout, changelog filename, and changelog-disabled message each exist as a single owned symbol — making "written == staged", "parse layout == emit layout", and pinned-wording invariants compile-time facts. The two validators remain separate functions.

**Do**:
1. Export the engine's remote reader (e.g. `engine.RemoteURL(ctx, r)`), have the cmd call site use it, delete `regenerateRemoteURL`.
2. Export `record.ChangelogDateLayout`, have engine/regenerate_write.go parse with it, remove the engine-local const.
3. Export `record.ChangelogFileName`, have the engine staging site(s) reference it, delete the engine-local const.
4. Lift "changelog is disabled in config" into one exported sentinel (e.g. `engine.ErrChangelogDisabled`); both validators reference it; keep both functions + concrete-enum signatures.
5. Do not change observable behaviour, error wording, written/staged paths, or the publisher-resolution contract.

**Acceptance Criteria**:
- `regenerateRemoteURL` deleted; cmd path calls the exported engine reader; behaviour unchanged.
- engine-local `regenerateDateLayout` removed; engine parses with the record-exported layout.
- engine-local `changelogFileName` removed; engine staging references `record.ChangelogFileName`; written == staged.
- Both changelog-disabled validators reference one exported sentinel; both functions + signatures remain.
- No error wording, paths, or contracts change observably.

**Tests**:
- The cmd regenerate path resolves the publisher using the shared remote reader (empty -> unresolved/downgrade) as before.
- A regenerate changelog heal produces a header whose date layout matches existing record-emitted sections.
- engine stages the same `CHANGELOG.md` path record writes (written == staged) via the shared symbol.
- Both changelog-disabled validators return the identical pinned message sourced from the shared sentinel.
