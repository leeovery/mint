---
topic: mint-release-tool
cycle: 3
total_proposed: 2
---
# Analysis Tasks: mint-release-tool (Cycle 3)

## Task 1: Extract single release-bookkeeping commit-subject builder
status: approved
severity: high
sources: duplication

**Problem**: The release-bookkeeping commit subject `{commitPrefix} Release {tag}` is built from scratch via `fmt.Sprintf("%s Release %s", commitPrefix, tag)` in three independent units that all describe the SAME release commit and MUST stay byte-identical:
- `internal/record/commit.go:80` — `record.CommitBookkeeping` makes the actual bookkeeping commit with it.
- `internal/release/release.go:125` — `composeTagMessage` embeds it as the annotated-tag subject line.
- `internal/engine/release.go:1258` — `bookkeepingSubject` rebuilds it for the `--dry-run` plan (its own comment concedes "It mirrors record.CommitBookkeeping's own subject format").
The dry-run plan promises the exact subject a real run will commit, and the tag annotation subject is expected to match the bookkeeping commit subject. Because each call site owns its own literal, any format change in one silently desyncs the plan and the tag from the real commit. A genuine drift risk across three packages.

**Solution**: Extract one exported subject builder in the `record` package — `record.BookkeepingSubject(commitPrefix, tag string) string` — alongside the existing `record.ChangelogFileName`, and route all three call sites through it. `record` is the natural owner (it owns the commit primitives and is already imported by both engine and release).

**Outcome**: The `{commitPrefix} Release {tag}` format exists in exactly one place; the real commit, the dry-run plan, and the tag annotation can no longer drift.

**Do**:
1. Add `BookkeepingSubject(commitPrefix, tag string) string` to `internal/record/commit.go` (exported, alongside `ChangelogFileName`), returning `fmt.Sprintf("%s Release %s", commitPrefix, tag)` with a doc comment naming it as the single source.
2. In `record.CommitBookkeeping` (commit.go:80) replace the inline `fmt.Sprintf(...)` with `BookkeepingSubject(commitPrefix, tag)`.
3. In `release.composeTagMessage` (release.go:125) replace the inline `fmt.Sprintf(...)` with `record.BookkeepingSubject(commitPrefix, tag)` (keep the record import).
4. In `internal/engine/release.go` make `bookkeepingSubject` (1254-1259) delegate to `record.BookkeepingSubject` (or call it directly at the dry-run plan site), removing the local `fmt.Sprintf`; update the stale "mirrors ..." comment.
5. Run go build / go vet / golangci-lint / go test ./...

**Acceptance Criteria**:
- A single exported `record.BookkeepingSubject(commitPrefix, tag)` is the only producer of the `{commitPrefix} Release {tag}` subject.
- All three former call sites obtain the subject from it; no `fmt.Sprintf("%s Release %s", ...)` literal remains in any of them.
- The actual bookkeeping commit subject, the annotated-tag subject line, and the dry-run plan subject are observably identical.
- go build / go vet / golangci-lint / go test ./... all pass.

**Tests**:
- Unit test for `record.BookkeepingSubject` asserting the exact `"{prefix} Release {tag}"` format.
- Existing tests in record/commit_test.go, release/release_test.go, and engine/release_test.go that assert the bookkeeping commit subject, tag annotation subject, and dry-run plan subject still pass (adjust only to reference the shared builder if they hardcode the literal).

## Task 2: Extract shared CHANGELOG stage-and-commit helper for regenerate paths
status: approved
severity: medium
sources: duplication

**Problem**: Both regenerate paths perform the same two-step git mutation — `m.Mutate(ctx, nil, "git", "-C", root, "add", record.ChangelogFileName)` then `m.Mutate(ctx, nil, "git", "-C", root, "commit", "-m", subject)`:
- `internal/engine/regenerate_changelog.go:68-74` (single-version, subject `docs(changelog): regenerate notes for {tag}`).
- `internal/engine/regenerate_batch_changelog.go:148-153` (`commitAndPushRebuild`, subject `batchRebuildSubject`).
The only difference in the core mutation is the commit subject. The shared push/recovery tail was already consolidated into `pushChangelogCommit` (task 8-1), but this matching stage+commit half was left duplicated. NOTE: the two paths wrap errors differently — single-version wraps with `tag` and returns `(bool, error)`; the batch path routes failures through `resetAndAbort(... startingHEAD ...)` — so the extraction must cover the bare `git add`/`git commit` Mutate pair while leaving each caller's distinct error-recovery wrapping intact.

**Solution**: Extract an engine helper `stageAndCommitChangelog(ctx, m, root, subject) error` (matching the actual Mutator type) owning the `git add CHANGELOG.md` + `git commit -m subject` sequence, parameterised only by subject. Each caller invokes it with its own subject and applies its own error handling. Mirrors how `pushChangelogCommit` consolidates the push tail.

**Outcome**: The `git add CHANGELOG.md` + `git commit -m subject` idiom lives in one engine helper; the single-version and batch regenerate paths can no longer drift on the stage/commit half.

**Do**:
1. In the engine package (near `pushChangelogCommit`) add `stageAndCommitChangelog(ctx, m, root, subject)` running the two `m.Mutate` calls, returning an error identifying which step failed (e.g. `staging %s` / `committing %q`) so callers retain failed-step context. Confirm the Mutator param type matches the call sites.
2. In regenerate_changelog.go:68-74 replace the two `m.Mutate` calls with one call to the helper, preserving the single-version error wrapping (`tag` context, `(bool, error)` contract).
3. In commitAndPushRebuild (regenerate_batch_changelog.go:148-153) replace the two `m.Mutate` calls with one call to the helper, route any error through the existing `resetAndAbort(...)` recovery, leave the subsequent `pushChangelogCommit` call untouched.
4. Keep the distinct subjects unchanged.
5. Run go build / go vet / golangci-lint / go test ./...

**Acceptance Criteria**:
- A single engine helper `stageAndCommitChangelog` owns the `git add CHANGELOG.md` + `git commit -m subject` sequence.
- Both regenerate_changelog.go and commitAndPushRebuild use it; no duplicated inline changelog `git add`/`git commit` Mutate pair remains.
- Each caller's error-recovery is preserved: single-version still wraps with `tag` and returns `(bool, error)`; batch still routes failures through `resetAndAbort` with `startingHEAD` and still calls `pushChangelogCommit` afterward.
- The single-version and batch commit subjects are unchanged.
- go build / go vet / golangci-lint / go test ./... all pass.

**Tests**:
- Existing single-version regenerate tests still assert the `docs(changelog): regenerate notes for {tag}` subject and no-change/no-empty-commit behaviour.
- Existing batch regenerate tests still assert the `batchRebuildSubject` commit, the single end-of-batch commit, and reset-to-startingHEAD recovery on a staging/commit failure (the `resetAndAbort` path still exercised through the helper's returned error).
