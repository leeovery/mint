# Plan: AI Model Selection

## Phases

### Phase 1: Config resolution layer
status: approved
approved_at: 2026-06-13

**Goal**: Establish `internal/config` as the single source of truth for the AI command and timeout. Add a typed, closed verb enum and the layered accessors `AICommandFor(verb)` / `TimeoutFor(verb)` that resolve each key independently through `verb override → shared top-level → shipped default`. Introduce the net-new top-level-plus-per-verb `timeout` key, promote `ai_command` to per-verb override, and pin the shipped default command to `claude -p --model sonnet` as the canonical constant that all other sites will derive from.

**Why this order**: The accessors and the canonical default value are the foundation every subsequent phase consumes — the transport, the three wiring sites, the init template, and the README all read resolved values or the pinned constant from here. Building this first means later phases add to a working, fully-tested resolution layer rather than depending on something not yet built. This is the feature's "strongest foundation first" Phase 1: it integrates with the existing strict-decoding `fileShape`/`resolveX` pattern and proves the resolution semantics in isolation before any consumer is touched.

**Acceptance**:
- [ ] A typed, closed verb enum is defined in `internal/config` with exactly two values (release, commit) and no `regenerate` value — making the accessor domain exhaustive by construction
- [ ] `cfg.AICommandFor(verb)` resolves `[verb].ai_command → top-level ai_command → shipped default`, trimming each candidate and skipping it when blank/whitespace at every layer; a top-level `ai_command = ''` falls through to the shipped default
- [ ] `cfg.TimeoutFor(verb)` resolves `[verb].timeout → top-level timeout → 60s floor`; missing or negative/invalid drops through, a positive value is used as-is, and an explicit `0` is honored ("no deadline") and stops fall-through rather than being treated as missing
- [ ] The shipped default `ai_command` constant is `claude -p --model sonnet`; with no `.mint.toml`, both verbs resolve to it and to the 60s timeout
- [ ] The new per-verb `ai_command` / `timeout` keys (in both verb shapes) and the new top-level shared `timeout` key are added to the schema, seeded in `defaults()` at 60s, with `typeErrorMessages` entries; strict decoding accepts the new keys and a genuine TOML type mismatch still surfaces as a decode error at `Load`
- [ ] The old `resolveAICommand` helper is folded into / replaced by the accessor so blank-skipping happens in exactly one place across all layers
- [ ] Per-key independence holds: overriding `ai_command` on a verb leaves that verb's `timeout` resolving through shared/floor, and vice-versa
- [ ] `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `go test -race ./...`, and `golangci-lint run` (0 issues) all pass

#### Tasks
status: approved
approved_at: 2026-06-13

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| ai-model-selection-1-1 | Pin the shipped default AI command to `claude -p --model sonnet` | none |
| ai-model-selection-1-2 | Define the typed closed verb enum (release, commit; no regenerate) | no regenerate value, no unknown/zero-value verb falling through to shared |
| ai-model-selection-1-3 | Add the per-verb `ai_command` override to the schema and strict decoding | genuine TOML type mismatch still a strict decode error at Load, unknown sibling keys still rejected |
| ai-model-selection-1-4 | Add the layered `AICommandFor(verb)` accessor with multi-layer trim-and-skip | blank/whitespace `[verb].ai_command` falls to shared, blank/whitespace shared falls to floor, top-level `ai_command = ''` falls to shipped default, accessor never yields empty |
| ai-model-selection-1-5 | Add the net-new top-level shared `timeout` key to the schema | absent vs explicit zero distinguished, genuine TOML type mismatch a strict decode error at Load, zero-config resolves to 60s |
| ai-model-selection-1-6 | Add per-verb `timeout` overrides to the schema and strict decoding | per-verb absent vs explicit zero distinguished, type mismatch still fails loud |
| ai-model-selection-1-7 | Add the layered `TimeoutFor(verb)` accessor with value semantics | explicit `0` honored and stops fall-through (not missing), negative drops through to floor, unparseable/value-invalid drops through, positive used as-is |
| ai-model-selection-1-8 | Prove per-key resolution independence across `ai_command` and `timeout` | per-key independence both directions, both verbs |

### Phase 2: Transport adoption and wiring
status: approved
approved_at: 2026-06-13

**Goal**: Make the AI transport carry no defaults of its own and apply the per-attempt deadline conditionally — `timeout = 0` skips `context.WithTimeout` entirely and runs on the parent context, while a positive value uses `WithTimeout`. Thread the resolved per-verb command and timeout from Phase 1's accessors through all three transport construction sites (`internal/engine/release.go`, `internal/commit/run.go`, `internal/engine/regenerate_fresh.go`), with regenerate deliberately resolving through the release verb. Preserve the absent-vs-explicit-zero invariant at the `config → ai.Config` boundary.

**Why this order**: This phase consumes Phase 1's accessors and canonical default — it cannot resolve a value until they exist, so it must come second (no forward references). It is the integration slice that makes the feature take effect at the running AI call: removing the transport's now-unreachable self-defaults, rewiring the deadline, and connecting the resolution layer to the three call sites. The risk profile here is distinct from Phase 1 — the load-bearing concern is the "no deadline only via explicit operator 0, never by a forgotten field" invariant, which warrants its own checkpoint.

**Acceptance**:
- [ ] The transport's `defaultAICommand` and `defaultTimeout` self-defaults are removed; the transport runs the concrete command and timeout config resolves and hands it, never re-defaulting
- [ ] `timeout = 0` makes the transport skip `context.WithTimeout` and run the attempt on the parent context (no instant/immediate timeout); a positive value uses `WithTimeout` with that value; any residual defensive negative handling does not collapse into the `0` no-deadline branch
- [ ] All three construction sites source both the command AND the timeout from the accessors; `internal/engine/regenerate_fresh.go` resolves through the release verb (argv asserted to carry the release values, not the bare shared/default)
- [ ] The `config → ai.Config` boundary preserves absent-vs-explicit-zero so "no deadline" is only reachable by an operator's explicit `0` — a wiring site omitting the field can never silently run unbounded; all three sites source the timeout from the accessor (never zero-by-omission)
- [ ] The transport's WHY-comments encoding the deleted contracts (`Config.AICommand` empty-default, `Config.Timeout` non-positive fallback, `NewTransport` zero-Config resolution, the `Generate`/`attempt` unconditional-`WithTimeout` note) are corrected in the same change to match as-built
- [ ] Every test pinning the old `claude -p` (no `--model`) default command/argv is migrated to the new pinned default
- [ ] `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `go test -race ./...`, and `golangci-lint run` (0 issues) all pass

#### Tasks
status: draft

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| ai-model-selection-2-1 | Delete the transport's command self-default and correct its comments | blank/whitespace AICommand no longer re-defaulted in transport (config floor guarantees non-empty), empty-name parseCommand path now unreachable from production |
| ai-model-selection-2-2 | Make `ai.Config.Timeout` carry absent-vs-explicit-zero and apply the per-attempt deadline conditionally | explicit `0` skips `context.WithTimeout` (no instant/immediate timeout), positive uses `WithTimeout` with that value, parent-context cancellation still propagates unchanged on the no-deadline path, residual negative defensive handling must not collapse into the `0` no-deadline branch, "no deadline" reachable only via explicit `0` never by a forgotten/zero-by-omission field |
| ai-model-selection-2-3 | Thread resolved command + timeout through the release wiring site (`internal/engine/release.go`) | a per-verb `[release]` override drives the argv (not the bare shared/default), timeout sourced from the accessor never zero-by-omission |
| ai-model-selection-2-4 | Thread resolved command + timeout through the commit wiring site (`internal/commit/run.go`) | a per-verb `[commit]` override drives the argv (not the bare shared/default), timeout sourced from the accessor never zero-by-omission |
| ai-model-selection-2-5 | Route the regenerate wiring site through the release verb (`internal/engine/regenerate_fresh.go`) | regenerate resolves the `[release]` values (argv asserted to carry release's command/timeout, not the bare shared/default), the easy-miss distinct construction site |
| ai-model-selection-2-6 | Migrate the old `claude -p` default-argv test pins to `claude -p --model sonnet` | FakeRunner seeds keyed by binary name still match (only exact-argv assertions change), the transport's ex-default-command test now asserts the passed command verbatim rather than a transport default |

### Phase 3: Operator surfacing and contract reconciliation
status: approved
approved_at: 2026-06-13

**Goal**: Surface the new configuration to operators and reconcile the in-repo contract. Update the `internal/initgen` commented template to scaffold the new keys (top-level `ai_command` at the pinned value sourced from the config constant, the shared `timeout` at 60s, and commented per-verb `# ai_command` / `# timeout` overrides under both `[release]` and `[commit]`). Document the keys, resolution order, pinned default, `timeout = 0` semantics, and the unenforced override-both pattern in the README. Update the `Commit` struct doc comment in `internal/config` so it no longer encodes the old "no per-verb override" contract.

**Why this order**: This phase depends on Phase 1's canonical pinned-default value (the template sources it, not re-types it) and on Phases 1–2's resolved semantics (the README and comments describe behaviour that must already be as-built). It is the user-facing and documentation slice — distinct work from the resolution/transport mechanics, with its own test-pin breakage (the initgen "full template loads cleanly" test) and the same-change comment reconciliation CLAUDE.md mandates. It comes last because documenting and scaffolding a behaviour before it exists would be a forward reference.

**Acceptance**:
- [ ] The init template's top-level `ai_command` shows `claude -p --model sonnet`, sourced from the config constant (not re-typed), and adds the shared `timeout` key uncommented at its 60s default
- [ ] Commented `# ai_command = …` and `# timeout = …` per-verb overrides appear under both `[release]` and `[commit]`; comments stay model-agnostic (no sonnet/opus/haiku naming, no "stronger model" steer) and the timeout hint is framed around command latency, not a model
- [ ] The `initgen` "full template loads cleanly" test passes against the updated template and the Phase 1 schema
- [ ] The README documents `ai_command` at both levels, `timeout` at both levels, the `verb → shared → default` resolution order, the new pinned default value as a fact (not a recommendation), the `timeout = 0` ⇒ "no time limit" semantics including the unbounded-call trade-off, and the supported-but-unenforced pattern of overriding command and timeout together for a slow verb
- [ ] The `Commit` struct doc comment in `internal/config/config.go` no longer asserts "Deliberately NOT added for commit" / the old no-per-verb-override contract, reflecting the now-shipped `[commit].ai_command` / `[commit].timeout`
- [ ] `go build ./...`, `gofmt -l .` (prints nothing), `go vet ./...`, `go test -race ./...`, and `golangci-lint run` (0 issues) all pass
