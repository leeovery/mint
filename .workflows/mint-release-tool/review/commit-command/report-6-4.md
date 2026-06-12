TASK: Consolidate the duplicated commit test-suite scaffolding (invocation-filter helpers and per-file Deps builders) [tick-686386, suffix 6-4]

ACCEPTANCE CRITERIA:
- Two shared invocation-filter helpers exist; no test file contains a re-rolled "Name==git" or "Args[0]==<verb>" filter loop (the nine wrappers are delegations or removed).
- A single editor-path Deps builder centralises the git.NewMutator(...WithBackoff...) wiring and StdinInteractive default; the previously-identical/subset builders are thin wrappers or inlined, with each scenario's distinct fields preserved.
- newCommitDeps (bare path) remains a separate builder.
- go test ./internal/commit/..., go vet, and golangci-lint pass clean; no test assertion changed meaning.

STATUS: Complete

SPEC CONTEXT: This is a test-scaffolding refactor (severity medium, source: duplication), not a behaviour change. The commit command suite was authored per-file in isolation, producing nine copy-pasted invocation-filter helpers (two filtering shapes: Name==git, and Name==git && Args[0]==verb) and seven per-file commit.Deps builders that had converged on the same git_safe Mutator wiring (git.NewMutator(er, git.WithBackoff(func(int){}))) and StdinInteractive=true default. Goal: collapse the two filter shapes to one helper each (serving both raw-runner and editorRunner.fake sources), and the editor-path Deps wiring to one builder, leaving newCommitDeps (bare FakeRunner path) separate. Test behaviour and assertions unchanged.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - gitInvocationsOf (run_test.go:74) and gitVerbInvocations (run_test.go:87) — the two shared filter helpers, each taking an already-extracted []runner.Invocation so one helper serves both sources. gitVerbInvocations delegates to gitInvocationsOf (run_test.go:89), so both shapes live in one place.
  - All nine wrappers are now one-line delegations: gitInvocations (run_test.go:100), gitInvocationsGen (generate_test.go:97), editorGitInvocations (run_noai_test.go:30) route through gitInvocationsOf; addInvocations (staging_defer_test.go:17), editorAddInvocations (run_noai_test.go:34), commitInvocations (run_test.go:760), editorCommitInvocations (run_noai_test.go:38), pushInvocations (run_push_test.go:18), editorPushInvocations (run_editor_push_test.go:18) route through gitVerbInvocations.
  - editorDepsOptions (run_failloud_test.go:91) + editorDeps (run_failloud_test.go:108) — the single editor-path builder; the Mutator wiring (line 112) and StdinInteractive default (line 118, modelled as !NonInteractiveStdin so the zero value = interactive TTY) live here alone. Options-struct shape, as the task's Solution preferred.
  - aiFailDeps (run_aifail_test.go:28), oversizedDeps (run_oversized_test.go:106), regenFailDeps (run_regen_fallback_test.go:58), editDeps (run_edit_test.go:21), noAIDeps (run_noai_test.go:24) reduced to thin one-line wrappers over editorDeps, each preserving its distinct fields (noAIDeps fixes NoAI:true; editDeps omits Staging, keeping the StagedOnly zero value; aiFail/oversized/regenFail pass Transport+Root+Staging).
  - newCommitDeps (run_test.go:61) kept separate as the bare FakeRunner + no-editor builder, per criterion.
- Notes: Spot-check confirmed both source axes route through the shared helpers — raw-runner via r.Invocations() (gitInvocations, commitInvocations, pushInvocations) and editorRunner via er.fake.Invocations() (editorGitInvocations, editorAddInvocations, editorPushInvocations). editorRunner is defined once (editor_open_test.go:24). gofmt reports no formatting issues; all nine wrappers retain active call sites (none orphaned). Out-of-scope raw-runner/bare-path Deps that still carry inline git.NewMutator(...WithBackoff...) wiring (run_test.go:218, staging_defer_test.go:315, run_push_test.go:250, regenDeps run_regen_test.go:54) are FakeRunner harnesses, not editorRunner — correctly excluded by the same "bare path stays separate" rationale that keeps newCommitDeps out.

TESTS:
- Status: Adequate (the existing suite IS the test for this refactor)
- Coverage: This is a refactor of test scaffolding, so the acceptance is that the existing commit suite passes unchanged with no assertion meaning altered. Reading the rewritten helpers and builders, the delegations are behaviour-preserving: gitVerbInvocations reproduces the prior "Name==git AND Args[0]==verb" semantics exactly (via gitInvocationsOf + the Args[0] guard), and editorDeps reproduces the prior field set for each scenario. The !NonInteractiveStdin inversion correctly maps the zero-value common case (interactive TTY) to StdinInteractive=true, matching the pre-refactor default the subset builders hard-set.
- Notes: Per the agent definition I did not execute the suite; adequacy is judged by reading. No over-testing introduced (the refactor removes duplication rather than adding assertions). The "no test assertion changed meaning" criterion cannot be machine-verified here without running, but the delegations are structurally equivalence-preserving on inspection.

CODE QUALITY:
- Project conventions: Followed. Helpers take []runner.Invocation at the call boundary (Go-idiomatic, decouples filter from source), options-struct over boolean-positional for editorDeps (matches the task's stated preference and golang-design-patterns guidance). Doc comments on every shared helper explain the dual-source intent.
- SOLID principles: Good. Single filter shape per helper; editorDeps is the single construction point for editor-path Deps.
- Complexity: Low. The two helpers are trivial loops; wrappers are one-liners.
- Modern idioms: Yes.
- Readability: Good. Comments justify each scenario's field choices (e.g. noAIDeps explains why the fail-loud guard does not fire).
- Issues: One residual that the task's own Solution called out — failLoudDeps (run_failloud_test.go:126) survives as a SECOND thin wrapper over editorDeps with a 7-parameter positional signature including a boolean triple (yes, stdinInteractive, noAI). The Solution explicitly said "options-struct variant preferred to avoid a boolean-triple parameter list." The wiring centralisation the criterion demands is met (failLoudDeps now delegates to editorDeps), but the boolean-triple call shape the Solution wanted gone persists across 13 call sites (positional calls like failLoudDeps(rec, er, commit.StagedOnly, root, false, true, true, nil) are hard to read at the call site). Non-blocking: the acceptance criteria require centralised wiring, not elimination of failLoudDeps, and editorDepsOptions already exists to absorb it.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/commit/run_failloud_test.go:126 — failLoudDeps keeps the boolean-triple positional signature the task's Solution advised replacing; since it now delegates to editorDeps, have its 13 call sites pass editorDepsOptions directly and delete failLoudDeps, removing the unreadable positional bool runs (e.g. ...root, false, true, true, nil).
