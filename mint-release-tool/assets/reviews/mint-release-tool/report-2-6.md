TASK: mint-release-tool-2-6 — Normal AI notes path wiring (prior-tag release)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/notes/generate.go Generate/GenerateWithContext → shared generateFromDiffWithContext. Orchestrates AssembleDiff → CheckDiffSize → BuildChangeMapForRange → ResolveInstructions+ComposePrompt → transport.Generate. Body returned byte-identical; failures wrapped with %w preserving ErrDiffTooLarge/ai.ErrTimeout/ai.ErrNotesFailure/ai.ErrCommandMissing. Transport seam defined at point of consumption (DIP). AI input diff-derived only — no git log channel.

TESTS:
- Status: Adequate. Covers validated body returned, body whole/multi-line passthrough, whitespace passthrough, in-order assemble→changemap→compose, exact instructions+ChangeMap+diff equality (no git-log invocation), too-large surfaces ErrDiffTooLarge with transport.calls()==0, post-exclusion size via diff_exclude glob, typed timeout/notes-failure cause preservation, one-time-context behaviour. recordingTransport fake.

CODE QUALITY:
- Followed conventions (runner/FakeRunner seam, focused tests, doc comments). SOLID good — locally-defined Transport interface (ISP+DIP). Low complexity, %w wrapping, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/notes/generate.go:163-165 — the shared core adds an IsDegenerate short-circuit (returns StubBody) owned by task 2-8, sitting in front via precedence (2-10). Well-justified and documented as harmless on the forward path, but it is functionality beyond 2-6's scope; confirm 2-8/2-10 are the authoritative owners and that this co-located guard doesn't create two sources of truth for the degenerate rule.
- [do-now] internal/notes/generate_test.go:144-176 (InvokesAssembleGuardChangeMapComposeTransportInOrder) — the test name claims the full assemble→guard→changeMap→compose→transport order but asserts only assemble-before-changeMap and that compose ran; the guard's position is verified only indirectly elsewhere. Tighten the name to what it proves, or add an assertion that the guard precedes the transport in this same test.
