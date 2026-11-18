TASK: ai-model-selection-1-1 — Pin the shipped default AI command to claude -p --model sonnet

ACCEPTANCE CRITERIA:
- The exported constant equals "claude -p --model sonnet".
- config.Load on an empty dir returns cfg.AICommand == "claude -p --model sonnet".
- The constant is exported (referenceable from another package) so Phase 3's initgen can source it without re-typing.
- The const WHY-comment and Config.AICommand doc comment state the new default value and the canonical-source role.
- go build / gofmt / go vet / go test -race / golangci-lint all pass.

STATUS: Complete

SPEC CONTEXT: specification.md "Pinned default model" and "Single source of truth for config defaults". The shipped default moves from bare `claude -p` to `claude -p --model sonnet` so zero-config behaviour is predictable rather than inheriting the operator's mutable Claude CLI default. The pin uses the `sonnet` alias (tracks current version, no rebuild to follow releases), Sonnet not Opus/Haiku. The constant becomes the single canonical source from which the transport and initgen derive — the literal must not be re-typed elsewhere. Not a breaking change in practice (no users yet); only internal test pins migrate.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/config/config.go:87 — `const DefaultAICommand = "claude -p --model sonnet"` (exported, correct value).
  - internal/config/config.go:70-86 — const WHY-comment: states the pinned `--model sonnet` default, the alias-tracks-version rationale, and the canonical/exported single-source role (transport carries no self-default; initgen sources from here).
  - internal/config/config.go:118-127 — Config.AICommand doc comment states the pinned default "claude -p --model sonnet" and names config.DefaultAICommand as the single canonical source.
  - internal/config/config.go:308 — defaults() seeds AICommand: DefaultAICommand (the exported name; no stale unexported reference).
  - internal/config/config.go:626-637 — the absent→default helper is aiCommandOrDefault, referencing DefaultAICommand; the old resolveAICommand name is gone.
- Notes: The task's Do list named `resolveAICommand` as the helper to update; the as-built helper is `aiCommandOrDefault` (the blank-skip logic was folded into the AICommandFor accessor per the spec's "Resolution value semantics", a later-phase refactor). All in-package references resolve to the exported constant. grep confirms zero remaining `defaultAICommand` (unexported) or `resolveAICommand` references. The transport (internal/ai/transport.go) carries no `claude -p` literal — only comment references to config.DefaultAICommand — confirming the de-duplication the canonical-source role exists to enable. go build and gofmt are clean on the changed files.

TESTS:
- Status: Adequate
- Coverage:
  - internal/config/config_test.go:841 TestLoad_AbsentAICommand_DefaultsToClaudeSonnet — renamed from the old DefaultsToClaudeP per the task; asserts cfg.AICommand == "claude -p --model sonnet" on an empty dir (the zero-config acceptance criterion).
  - internal/config/config_test.go:858 TestDefaultAICommand_ExportedCanonicalValue — pins config.DefaultAICommand == "claude -p --model sonnet" AND that it is referenceable from the external test package (proves the exported-constant criterion directly).
  - internal/config/config_test.go:871 TestLoad_ExplicitAICommand_Honoured — explicit top-level ai_command carried verbatim (the "pinned default is the floor, not a forced value" test); unchanged and still passing.
  - internal/config/config_test.go:805 TestLoad_FullyValidFile_LoadsWithoutError — fixture keeps `ai_command = "claude -p"` as an explicit valid value (asserts a load succeeds, not the default), exactly as the task instructed it may stay.
- Notes: Coverage maps one-to-one onto the acceptance criteria and the task's two named tests. Not under-tested (constant value, zero-config default, export/cross-package reference, and explicit-honour all covered). Not over-tested — each test pins a distinct fact; no redundant assertions. Edge cases: none per the task table. Tests are external-package (config_test), table-free where a single assertion fits, t.Parallel() throughout — consistent with project test idioms.

CODE QUALITY:
- Project conventions: Followed. Exported const with a full WHY-comment per CLAUDE.md's heavy-comment contract; comments are true to as-built (default value, canonical-source role, alias rationale). The Config.AICommand doc edit was correctly scoped to the default-value change (transport re-defaulting wording is a later task's concern, per the Do list).
- SOLID principles: Good. Single canonical source removes the cross-package literal duplication; config never imports ai (decoupling preserved).
- Complexity: Low. A const value/name change plus comment rewrites.
- Modern idioms: Yes. Exported typed const, idiomatic Go.
- Readability: Good. Comment states the contract and the reasoning the code cannot show.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/initgen/initgen.go:47 — The spec (specification.md lines 126, 136) requires the init template's ai_command value be "sourced from the config constant, not re-typed". The as-built template carries `claude -p --model sonnet` as a static literal (initgen deliberately does not import config; a drift test pins the literal equal to config.DefaultAICommand instead). This is a Phase 3 concern, not task 1-1 (1-1 only had to export the constant, which it did). Flagging so the Phase 3 task review decides whether the drift-test-pinned literal satisfies "sourced from the constant" or whether a true compile-time reference is required — a genuine design-intent call, not a 1-1 defect.
