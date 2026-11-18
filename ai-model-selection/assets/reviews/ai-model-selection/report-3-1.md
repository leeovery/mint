TASK: ai-model-selection-3-1 — Scaffold the new ai_command / timeout keys in the initgen commented template

ACCEPTANCE CRITERIA:
1. Active top-level ai_command reads `claude -p --model sonnet`, sourced from / pinned against config.DefaultAICommand (never a free-floating literal).
2. Active top-level shared `timeout` key at the 60s default (integer seconds), tied to config.DefaultTimeout the same way.
3. Commented `# ai_command = …` and `# timeout = …` per-verb override lines under BOTH [release] and [commit], each a single-# config line with a trailing # explanation.
4. All comments touching the new keys are model-agnostic: no sonnet/opus/haiku in any explanation, no "stronger model" steer, timeout hint framed around command latency, no concrete model-tied timeout number.
5. TestMintTOML_UncommentedLoadsCleanly passes against the Phase 1 schema and asserts cfg.AICommand == claude -p --model sonnet and a resolved 60s timeout.
6. commonKeyDefaults includes the new ai_command and timeout active lines; common/optional-key tests pass.
7. initgen package/MintTOML doc comments stay true-to-as-built (static-template / no-project-auto-detection contract preserved; constant-sourcing noted).
8. go build / gofmt / go vet / go test -race / golangci-lint all pass.

STATUS: Complete

SPEC CONTEXT:
Spec section "Init template scaffolding" (specification.md L131-141) plus "Single source of truth for config defaults" (L116-129). The template must surface the new keys per project convention. Top-level uncommented ai_command bumped to claude -p --model sonnet (pinned default VALUE sourced from the config constant, not re-typed) and new shared timeout at 60s; per-verb overrides shown COMMENTED under both [release] and [commit]; comments stay MODEL-AGNOSTIC (the default VALUE may carry --model sonnet, but no model naming or "stronger model" steer in any explanation); timeout hint framed around command latency, no concrete model-tied timeout number. The "DELIBERATELY static — no project auto-detection" contract must survive. CLAUDE.md: new config keys must appear in the initgen template.

IMPLEMENTATION:
- Status: Implemented (matches all acceptance criteria, no drift)
- Location: internal/initgen/initgen.go
  - L47: active top-level `ai_command = 'claude -p --model sonnet'` (matches config.DefaultAICommand, internal/config/config.go:87).
  - L48: active top-level `timeout = 60  # per-attempt AI deadline in seconds; raise it if your ai_command runs slowly (0 = no limit)` (matches int(config.DefaultTimeout/time.Second) = 60, config.go:99; documents the 0 = no-limit semantic per spec L97).
  - L66-67: commented per-verb `# ai_command = …` / `# timeout = 120 …` under [release].
  - L80-81: commented per-verb `# ai_command = …` / `# timeout = 120 …` under [commit] (whose header is itself commented `# [commit]` at L77).
  - L16-22: doc-comment updated to record that the pinned default VALUES are config.DefaultAICommand / config.DefaultTimeout (single source of truth), the template carries them as static literals for readability, and the drift tests pin the literals equal to the constants. The "static template / no project auto-detection" contract is explicitly preserved.
- Notes:
  - Constant-sourcing mechanism chosen = static literal + build-failing drift test (the spec offered either build-from-constant OR test-pin; test-pin is taken, satisfying "sourced from / pinned against the constant, never a free-floating literal"). initgen does NOT import config, preserving the package's zero-dependency static design — a deliberate, spec-sanctioned choice.
  - Uncomment round-trip verified by reading the schema: [release].ai_command, [release].timeout, [commit].ai_command, [commit].timeout are all valid fileShape fields (config.go:329-367) with strict-decode + typeErrorMessages support. Uncommenting adds exactly ONE ai_command and ONE timeout per table (active blocks carry neither), so no duplicate-key error; timeout = 120 is a valid positive integer; the document strict-decodes cleanly.
  - Commit ff7e000 lands the change on a clean tree; gate-passing claimed by the task and consistent with the committed state.

TESTS:
- Status: Adequate (focused, 1:1 with acceptance criteria, includes drift guards; no over-testing)
- Coverage (internal/initgen/initgen_test.go):
  - commonKeyDefaults (L19-28) updated: includes `ai_command = 'claude -p --model sonnet'` and `timeout = 60` — AC6.
  - TestMintTOML_AICommandIsPinnedSonnetDefault (L46) — AC1 surface.
  - TestMintTOML_AICommandValueEqualsConfigConstant (L62) — AC1 drift guard, build-fails on template/constant divergence.
  - TestMintTOML_ScaffoldsActiveSharedTimeout (L73) — AC2 surface.
  - TestMintTOML_TimeoutValueEqualsConfigConstant (L87) — AC2 drift guard, pins to int(config.DefaultTimeout/time.Second).
  - TestMintTOML_PerVerbAICommandAndTimeoutOverridesShownCommented (L102) — AC3, asserts both keys commented-with-explanation under both [release] and [commit] via tableSection + hasExplainedCommentedKey. tableSection (L218) correctly matches the commented `# [commit]` header.
  - TestMintTOML_CommentsNameNoModelOrStrongerModelSteer (L126) — AC4, scans only the trailing EXPLANATION (commentExplanation), correctly permitting `--model sonnet` in a config VALUE while forbidding model tokens / stronger-model phrases in prose.
  - TestMintTOML_TimeoutHintFramedAroundLatency (L156) — AC4, asserts every timeout config-line explanation references slow/latency and (by the model-agnostic test) carries no model-tied number.
  - TestMintTOML_UncommentedLoadsCleanly (L469) — AC5, extended to assert cfg.AICommand == claude -p --model sonnet (L487) and shared cfg.Timeout == config.DefaultTimeout (L494); the L491-493 comment correctly explains why per-verb resolution is NOT asserted here (uncommenting activates the per-verb timeout = 120 overrides, which correctly win).
- Notes:
  - commentExplanation correctly returns "" for the active ai_command line (no trailing #), so the value's `sonnet` is not scanned as prose — the allow/forbid boundary is right.
  - No redundant assertions; the surface test + drift-guard pair per key is deliberate (surface = "shows the value"; drift = "value tracks the constant"), not duplication.
  - Edge cases from the task's Tests/Edge-Cases list are all covered (constant-source, model-agnostic prose, latency framing, commented per-verb optional-key convention, uncomment-loads-cleanly, sanity-pin default update).

CODE QUALITY:
- Project conventions: Followed. New config keys appear in the scaffolded template (CLAUDE.md non-negotiable). initgen stays a PURE static generator (no IO, no config import); doc-comment kept true-to-as-built in the same change (CLAUDE.md comment contract). Test idioms honoured: external test package, t.Parallel() throughout, exact-line assertions, behaviour-level proofs (uncomment-loads-cleanly through real config.Load).
- SOLID principles: Good. Single responsibility intact; the drift guarantee is enforced by test rather than coupling initgen to config at compile time — a reasonable trade preserving the package's zero-dependency design.
- Complexity: Low. Template is a single static string; test helpers are small and clear.
- Modern idioms: Yes.
- Readability: Good. Comments are model-agnostic, latency-framed, and self-documenting; the 0 = no-limit semantic is surfaced inline.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/initgen/initgen.go:66,80 — the per-verb commented example value `'claude -p --model sonnet'` is a hand-typed literal not covered by any drift test (only the top-level active ai_command line is pinned to config.DefaultAICommand). The spec only mandates the top-level pinned-default value be constant-tied, so this is in-spec, but the per-verb example could silently drift from the shipped default. Decide whether to extend the drift pin to the per-verb example lines or accept them as deliberately illustrative.
- [idea] internal/initgen/initgen.go:67,81 — both per-verb timeout override examples use the same illustrative `120`. The task said the example should be "distinct from 60 so it reads as an override example"; 120 satisfies that. Consider whether reusing 120 across both verbs (vs distinct values) best communicates "this is an arbitrary example," or leave as-is for consistency.
