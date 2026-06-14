TASK: ai-model-selection-4-1 — Consolidate the three duplicated transport-construction wiring sites into one shared helper

ACCEPTANCE CRITERIA:
- The production construction expression ai.NewTransport(r, ai.Config{AICommand: cfg.AICommandFor(verb), Timeout: cfg.TimeoutFor(verb)}) appears in exactly one place; the three wiring sites delegate to it.
- Each wiring site retains its own local nil-injected-transport test-seam guard (unchanged behaviour for the injected path).
- internal/config still does not import internal/ai — the decoupling is preserved.
- Each site passes its correct verb constant: VerbRelease for aiTransport and resolveFreshTransport, VerbCommit for commitTransport; regenerate continues to resolve through [release].
- All existing white-box transport tests pass unchanged; no argv or rendered-line drift.
- All gates pass: build, gofmt -l . prints nothing, go vet, go test -race, golangci-lint run reports 0 issues.

STATUS: Complete

SPEC CONTEXT:
The spec ("Migration & mechanical carry-overs" → "Transport wiring sites (3)" and "Resolution value semantics" → timeout boundary) requires the resolved per-verb command AND timeout be threaded at all three construction sites (release.go, commit/run.go, regenerate_fresh.go), with regenerate_fresh.go flagged as the "easy miss" that must deliberately resolve through [release]. The mandatory invariant: "no deadline" must only ever be reachable by an operator's explicit 0, never by a wiring site omitting the field. The ai↔config decoupling is mandatory (config never imports ai; the transport stays content-agnostic). This task is the analysis-cycle-1 DRY consolidation of those three now-byte-identical construction expressions into one helper, without disturbing the per-site injected-transport seam.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/aitransport/aitransport.go:40-45 — the single shared helper New(r, cfg, verb) holding the sole ai.NewTransport(r, ai.Config{AICommand: cfg.AICommandFor(verb), Timeout: cfg.TimeoutFor(verb)}) expression.
  - internal/engine/release.go:933-938 — aiTransport keeps its deps.Transport != nil short-circuit, delegates with config.VerbRelease.
  - internal/engine/regenerate_fresh.go:133-138 — resolveFreshTransport keeps its transport != nil short-circuit, delegates with config.VerbRelease (the easy-miss site, correctly routed through [release]).
  - internal/commit/run.go:777-782 — commitTransport keeps its deps.Transport != nil short-circuit, delegates with config.VerbCommit.
- Notes:
  - Exactly-one-place confirmed: a repo-wide grep for ai.NewTransport finds only the definition (internal/ai/transport.go:105) and the single production call inside aitransport.go:41. No other production site constructs a transport.
  - config↔ai decoupling preserved: grep for "mint/internal/ai" under internal/config returns nothing. The helper's own package doc (aitransport.go:9-15) correctly reasons why it cannot live in ai (transport stays content-agnostic, CLAUDE.md seam #5) nor in config (config must not import ai) nor in engine/commit (independent siblings) — a seam-clean dedicated package is the only valid home, and it imports only ai+config+runner with no cycle.
  - Each site keeps its own injected-transport guard, as required (the deps-wrapper type legitimately differs per site); only the construction expression moved.
  - The *time.Duration absent-vs-zero invariant survives: TimeoutFor returns *time.Duration and is total (never nil; floor always present, config.go:596-624), and is assigned DIRECTLY to ai.Config.Timeout (also *time.Duration); NewTransport panics on a nil Timeout (transport.go:106-110), so a forgotten field is a loud programmer error, never a silent unbounded run. The explicit-0 → no-deadline path is preserved end-to-end.
  - Verb enum (internal/config/verb.go) is the closed two-value set with no regenerate member, so resolveFreshTransport structurally can only pass VerbRelease — the routing is un-missable by construction, matching the spec.

TESTS:
- Status: Adequate
- Coverage:
  - New focused package test internal/aitransport/aitransport_test.go:28-117 — table-driven across both verbs, proving New sources BOTH the command (observed as invoked argv via runner.DeadlineRecordingRunner) and the per-attempt deadline (observed as context-deadline presence) from cfg.AICommandFor(verb)/cfg.TimeoutFor(verb): VerbRelease reads [release], VerbCommit reads [commit], and explicit-0 threads a deadline-free context for both verbs. Directly matches the task's optional "if independently unit-testable, add a focused per-verb test asserting accessor-resolved command + timeout, never zero-by-omission."
  - Per-site white-box tests retained and intact: internal/engine/release_aitransport_internal_test.go, internal/engine/regenerate_fresh_aitransport_internal_test.go, internal/commit/run_aitransport_internal_test.go (+ the black-box run_aitransport_test.go). The regenerate internal test (regenerate_fresh_aitransport_internal_test.go:24-100) still pins the [release] override binary/args and the explicit-0/positive deadline behaviour through resolveFreshTransport — proving regenerate-rides-[release] and per-verb threading survive the consolidation, exactly as the task's test section requires.
- Notes:
  - Balanced — not over-tested: the new helper test does not duplicate the per-site tests; it proves the centralized expression once per verb, while the per-site tests prove each call site passes the right verb constant through the (now-shared) construction. This division is appropriate, not redundant.
  - Not under-tested: the explicit-0 no-deadline edge case (the highest-risk part of the absent-vs-zero invariant) is covered for both verbs at the helper and at the easy-miss regenerate site. The nil-Timeout panic guard is documented and exercised implicitly (reaching Generate proves the field was threaded).
  - Tests assert behaviour (invoked argv + context-deadline presence) rather than implementation details — consistent with project test idioms.

CODE QUALITY:
- Project conventions: Followed. Honours CLAUDE.md seams (#1 every command via runner; #5 ai stays content-agnostic; config strictness untouched). Heavy WHY-comments are present and true to as-built — the package doc, New's doc, and all three call-site comments accurately describe the shared-helper contract and the per-site retained guard. DurationPtr/DeadlineRecordingRunner correctly ship in a production runner file (not _test.go) so the three test packages can share them across package boundaries.
- SOLID principles: Good. Single responsibility (one mapping: (runner, config, verb) → *ai.Transport). The helper depends only on the three already-imported packages; no new coupling introduced.
- Complexity: Low. New is a single return expression; call sites are a guard plus a delegation.
- Modern idioms: Yes. Typed closed enum for verb; *time.Duration pointer to distinguish absent/zero; direct assignment preserving the boundary contract.
- Readability: Good. Intent is self-evident and the rationale for the package's existence is documented where a reader would question it.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None. (The DRY consolidation is justified — three byte-identical expressions, the spec-flagged drift risk is real — without over-abstraction; the helper is a thin, single-responsibility mapping with a clean seam-justified home, and all observations about it propose no concrete change.)
