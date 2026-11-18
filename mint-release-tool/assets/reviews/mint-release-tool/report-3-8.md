TASK: mint-release-tool-3-8 — Up-to-two-commit graph (hook-artifact then bookkeeping)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. Assembled in the spine: Stage 3 runPreTagHook→record.CommitDirtyTree, Stage 5 record.CommitBookkeeping, tag+push via Releaser.TagAndPush. Tag target correct BY CONSTRUCTION — `git tag -a {tag} -F -` tags HEAD (no explicit target), HEAD = last commit made, so bookkeeping-when-present / hook-when-only-hook / pre-existing-HEAD-when-neither all fall out. No-op safety inherited from 3-3/3-7 (each commits only on real change); no empties when combining. Atomic push sends all commits + tag.

TESTS:
- Status: Adequate. release_commitgraph_test.go drives all four combinations (BOTH / ONLY-BOOKKEEPING / ONLY-HOOK / NEITHER) through the real spine on FakeRunner+RecordingPresenter, asserting exact subjects-before-tag, last-commit-before-tag (= tag target), absence of empty commits/staging, atomic push after tag. version_file w/ changelog=false path covered in release_versionfile_test.go.

CODE QUALITY:
- Followed conventions (git through Runner/Mutator, accept-interfaces, tracked MadeState, single-source subject builders). SOLID/DRY good (one deliberate parallel-rule duplication noted). Low complexity, excellent readability (invariants documented inline).

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/engine/release.go:918 / internal/record/commit.go:100 — bookkeepingWillCommit re-implements the no-op rule that record.bookkeepingPaths independently owns; a future change to one could silently desync made.Commits from the actual commit. Expose a single shared predicate from record (e.g. record.BookkeepingWillCommit) that both CommitBookkeeping and the spine call. [Same as 3-7.]
- [quickfix] internal/engine/release_commitgraph_test.go:33-56 — lastCommitBeforeTag/commitsBeforeTag match commit invocations but assertNoArtifactCommit covers add -A only on the negative paths, not the positive BOTH path; add a guard that a commit with add -A staging precedes the artifact commit so a regression dropping the add -A stage but keeping the commit is still caught.
- [do-now] internal/engine/release_commitgraph_test.go:22-24 — commitGraph*Subject constants hardcode v0.0.1 and the literal 🌿 prefix; add a one-line comment cross-referencing record.BookkeepingSubject/pretagArtifactSubject as the source of truth.
