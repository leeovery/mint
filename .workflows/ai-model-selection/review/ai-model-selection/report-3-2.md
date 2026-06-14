TASK: ai-model-selection-3-2 — Document the new config surface in the README

ACCEPTANCE CRITERIA:
1. Shared engine keys table's `ai_command` default is `claude -p --model sonnet`; a `timeout` row exists (default 60, latency-framed), stated as fact (no recommendation wording).
2. [release] and [commit] tables each document `ai_command` and `timeout` as optional per-verb overrides resolving [verb] → shared → default.
3. README states the `verb → shared → default` resolution order explicitly for both keys.
4. The AI Transport section documents `timeout = 0` ⇒ no per-attempt deadline / unbounded AI call, including the conscious trade-off (deliberate exception to "fail loud, never hang").
5. README documents the supported-but-unenforced pattern of overriding `ai_command` and `timeout` together for a slow verb (mint does not auto-bump, warn, or require the pair).
6. Scaffold snippet matches the Task 3-1 initgen template (`ai_command = claude -p --model sonnet`, `timeout = 60`).
7. No env-var override, AI driver/registry, or interactive-init scaffolding documented; the two-layer (defaults ← project file) model preserved.
8. Gates remain green (README-only change).

STATUS: Complete

SPEC CONTEXT:
specification.md "README documentation" section enumerates exactly the five doc items: both-level ai_command + timeout, the verb→shared→default order, the pinned default claude -p --model sonnet stated as a fact, timeout = 0 ⇒ no-limit with the unbounded-call trade-off, and the override-both-for-a-slow-verb supported-but-unenforced pattern; no breaking-change callout (mint has no users). Cross-referenced against the as-built code: config.DefaultAICommand = "claude -p --model sonnet" (config.go:87), config.DefaultTimeout = 60s (config.go:99), AICommandFor / TimeoutFor resolution chains (config.go:538, :596), and the transport's conditional WithTimeout / explicit-0 = no-deadline mapping (transport.go:105-122, :191-197).

IMPLEMENTATION:
- Status: Implemented
- Location: README.md — Configuration scaffold (L173-208), Shared engine keys table (L210-219), [release] table (L221-239), [commit] table (L248-257), The AI Transport section (L259-275).
- Notes: Every acceptance item is present and accurate to the as-built code:
  - AC1: Shared table `ai_command` default `claude -p --model sonnet` (L216) and `timeout` row default `60` with latency-framed, model-free description "per-attempt AI deadline in seconds; 0 = no limit; ... Raise it if your ai_command runs slowly" (L217). Stated as fact — no "recommended"/"we suggest"/"for best results" anywhere. Matches config.DefaultAICommand (config.go:87) and DefaultTimeout = 60*time.Second (config.go:99).
  - AC2: [release] table adds `ai_command` (L237) and `timeout` (L238) override rows; [commit] table adds the same (L254, L255). Each names "[release]/[commit] → shared → default". Matches AICommandFor (config.go:538) / TimeoutFor (config.go:596) chains.
  - AC3: Resolution order stated explicitly in the Shared engine keys preamble (L212: "[verb].<key> → top-level shared <key> → shipped default") and again in The AI Transport section (L269). Per-key independence stated at L275 ("resolve independently per verb, so overriding one does not touch the other"), matching the per-key-independent accessors.
  - AC4: timeout = 0 ⇒ unbounded documented at L273 with the conscious trade-off framing ("a deliberate, operator-chosen exception to mint's 'fail loud, never hang' posture ... you own that trade-off"). Accurate to the transport: explicit &0 maps to a nil internal deadline and the attempt runs on the parent ctx with no WithTimeout (transport.go:116-120, :191-197).
  - AC5: Override-both-for-a-slow-verb documented at L275 as supported-but-unenforced ("mint does not auto-bump the timeout, warn, or require the pair ... the supported pattern; it is your responsibility, not enforced"). Accurate — no coupling enforcement exists anywhere in config or transport.
  - AC6: Scaffold snippet (L176-177) reads `ai_command = 'claude -p --model sonnet'` and `timeout = 60` with the inline comment byte-identical to initgen.go:47-48. Per-verb commented lines at L195-196 ([release]) and L206-207 ([commit]) mirror initgen.go:66-67/:80-81 (`# ai_command = 'claude -p --model sonnet'`, `# timeout = 120`). Snippet and template agree.
  - AC7: No MINT_AI_COMMAND-style env layer, no AI driver/provider-registry, no interactive mint init prompting documented. Two-layer model preserved (L171: file fully optional, every key defaults). The Transport snippet (L264-266) reconciles the "default" vs "pin a model" examples per the task: `--model sonnet` is labelled "the shipped default", `--model opus` is "pin a different model" (a neutral example, not a steer).

TESTS:
- Status: Adequate (N/A by design)
- Coverage: Documentation/prose task — no Go test harness asserts README content (correctly so; the README is markdown, not compiled). Verification is by review against the acceptance criteria plus the gates remaining green, exactly as the task's Tests section prescribes. The scaffold↔schema agreement that the README depends on (AC6) is independently protected by the initgen drift tests pinning the template literals equal to config.DefaultAICommand / config.DefaultTimeout (described in initgen.go:18-22), so a stale README scaffold value cannot silently diverge from the schema without a build-time failure in that sibling task.
- Notes: No over- or under-testing concern — adding a Go test that greps README prose would be brittle and is correctly absent.

CODE QUALITY:
- Project conventions: Followed. The README documents the as-built two-layer model and the new keys without inventing narration; the AI Transport prose preserves the "fail loud, never hang" vocabulary used in CLAUDE.md and surfaces timeout = 0 as the explicit, documented exception the spec mandates. Model-agnostic framing is honoured (no recommended model; the default value carries --model sonnet as a stated fact, consistent with the config-comment convention in spec §"Init template scaffolding").
- SOLID principles: N/A (prose).
- Complexity: N/A.
- Modern idioms: N/A.
- Readability: Good. Tables are consistent with the existing README style; the resolution order is stated once authoritatively in the shared-keys preamble and echoed (not contradicted) in the transport section; cross-links to The AI Transport are present.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None. (No concrete, actionable change identified; every spec doc item is present, accurate to the as-built code, and free of recommendation wording. The minor redundancy of the "0 = no limit" / latency hint appearing in both the scaffold inline comment and the shared-keys table is intentional and mirrors the initgen template — not a finding.)
