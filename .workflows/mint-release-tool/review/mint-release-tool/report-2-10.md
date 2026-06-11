TASK: mint-release-tool-2-10 — Notes-path precedence resolution

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/notes/select.go (SelectBody / SelectBodyWithCacheInputs / SelectBodyWithReuse, resolveNormalPathFailure); wired as single Stage-4 decision point in engine/release.go resolveBody. Precedence strictly ordered: first-release short-circuits before any assemble; diff assembled once and reused by branch 4 (GenerateFromDiff); degenerate before --no-ai; on_notes_failure reached only in branch 4. Composes existing providers, reimplements none. Reports Kind.

TESTS:
- Status: Adequate. select_test.go covers all seven ACs (first-release wins w/ zero git/transport, degenerate wins over --no-ai, --no-ai wins over normal AI, normal AI only when no guard, on_notes_failure abort + fallback, branches 1-3 never route through on_notes_failure, Kind reported). cacheinputs_test.go covers cache-input/reuse surface. Invocation-count + argv assertions.

CODE QUALITY:
- Followed conventions (clear seams, CommandRunner, %w, table test). SOLID good — single decision point, DI via NewSelector, composes. Low complexity, doc comments make precedence explicit.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/notes/select.go:155-158 — an AssembleDiff git error on a non-first-release run returns KindNormalAI even though no AI path was entered (failure happened during the degenerate-check assemble). Harmless for abort (no body, error propagates) but the reported Kind is slightly misleading for callers branching on Kind for reporting. Decide whether a dedicated "assemble-failure" signal/neutral Kind is warranted, or document the convention.
- [do-now] internal/notes/select.go:185 — in SelectBodyWithReuse, a reuse-hook error returns KindNormalAI with a comment-free literal; add a one-line comment (mirroring resolveNormalPathFailure's doc) noting the reuse error is reported as KindNormalAI because the cacheable AI path was entered.
