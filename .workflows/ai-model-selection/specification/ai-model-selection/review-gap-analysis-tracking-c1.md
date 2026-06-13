---
status: in-progress
created: 2026-06-13
cycle: 1
phase: Gap Analysis
topic: ai-model-selection
---

# Review Tracking: AI Model Selection - Gap Analysis

## Findings

### 1. The `verb` parameter for the typed accessors is undefined

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Single source of truth for config defaults" (the `cfg.AICommandFor(verb)` / `cfg.TimeoutFor(verb)` bullet); "Migration & mechanical carry-overs" (transport wiring sites)

**Details**:
The spec names typed accessors `cfg.AICommandFor(verb)` / `cfg.TimeoutFor(verb)` as the single home for the `verb → shared → default` chain, but never defines what `verb` *is*. There is no existing verb-identifier type in `internal/config` (confirmed: config has no `Verb` type or verb constants — only `Release`/`Commit` struct fields and TOML table tags). An implementer cannot write these accessors without inventing the parameter's type and value set, which is a design decision the spec should not leave open:

- Is `verb` a raw `string` (`"release"`/`"commit"`), or a typed enum (`config.Verb`)? A raw string invites typos that silently fall through to the shared baseline (no compile-time safety); a typed enum needs its own definition and exported constants.
- What is the exact, closed value set? The spec says "Verb config space is exactly two tables: `[release]` and `[commit]`" — so two values — but the accessor's domain is never stated as that closed set.
- How does each call site obtain its value? Critically, the regenerate wiring site (`internal/engine/regenerate_fresh.go`) "must deliberately resolve through `[release]`" — so it must pass the *release* verb, not a `regenerate` value. The accessor contract should make that the only expressible choice (there is no `regenerate` verb), and the spec should state how the three sites map to the two values.
- What does the accessor return for an unrecognized `verb` (if `string`-typed)? Fall through to shared, or panic/error?

Without this, three wiring sites and the accessor signatures cannot be planned into tasks without guessing the central abstraction.

**Proposed Addition**:
_(leave blank until discussed)_

**Resolution**: Pending
**Notes**:

---

### 2. `timeout` value-validity rules are undefined for the undecided TOML representation

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Config schema: `timeout` key" (TOML representation deferred); "Resolution value semantics" (the `timeout` bullets)

**Details**:
The spec defers the `timeout` TOML representation (int seconds vs string duration) to planning, yet simultaneously pins value-level semantics that depend on that representation:

- "Zero is an explicit, honored value meaning 'no time limit' … stops the fall-through"
- "Missing or invalid (e.g. negative) drops through … positive is used as-is"
- "A wrong *type* still surfaces as a strict decode error at `Load` … distinct from a value-invalid drop-through"

These three rules are clear for the **int-seconds** representation (absent → nil pointer; 0 → no deadline; negative → drop-through; non-int → strict type error). They are **ambiguous for the string-duration** representation, and the spec lists string duration as a live option:

- Is `"0s"` (or `"0"`) the honored zero? Is an unparseable duration string (e.g. `"fast"`) a *value-invalid drop-through* or a *strict type error*? With a string field it decodes fine as a string, so the "wrong type → strict decode error at Load" path (which the spec relies on for the int case) does not fire — duration parse failure would have to be a new validate-at-Load or a resolution-time drop-through, and the spec picks neither.
- Is a negative duration string (`"-5s"`) the "invalid drops through" case, and where is it detected (Load-time validation vs resolution-time)?

The "absent vs zero" distinction the spec mandates is mechanically different per representation (`*int` nil-vs-0 vs `*string` nil-vs-`"0s"`). Because the rules and the representation are decided in different places (one deferred, one pinned), an implementer choosing string-duration would have to invent the validity/parse-failure routing the spec leaves blank. Either pin enough of the representation to make the value rules unambiguous, or restate the value rules in representation-neutral terms that cover the parse-failure case.

**Proposed Addition**:
_(leave blank until discussed)_

**Resolution**: Pending
**Notes**:

---

### 3. Where blank/whitespace `ai_command` is detected and dropped is unspecified

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Resolution value semantics" (`ai_command` bullets); "Single source of truth for config defaults" (accessor + transport-default deletion)

**Details**:
The spec says `ai_command` that is "blank / whitespace / invalid / missing at a layer **drops through** to the next layer", that the shipped default is the floor (resolution never empty), and that the transport's empty-handling is therefore "unreachable and removed." But it does not say which component now owns the blank/whitespace detection across the three layers, and the as-built code makes this non-trivial:

- Today `config.resolveAICommand` only handles **absent (nil) vs present**, copying any present value (including `''` and `'   '`) verbatim, with the transport (`NewTransport`, `strings.TrimSpace(command) == ""`) doing the empty re-default. The spec deletes the transport's default but the *whitespace-blank* detection currently lives there too.
- For per-verb override + shared baseline + floor, "blank drops through" must be applied at **each** layer (a blank `[release].ai_command` falls to shared; a blank shared falls to the floor). The new `AICommandFor` accessor must therefore trim-and-skip at every layer — not just re-default a top-level nil. The spec implies this but never states that the accessor (not `resolveAICommand`, which is keyed on the file-shape pointer) is where multi-layer blank-skipping lives, nor whether `resolveAICommand` is replaced, kept, or folded in.

This is a concrete behavioral relocation (whitespace-blank detection moving from transport into config's per-layer resolution) that an implementer would otherwise have to reverse-engineer and design. Stating the owning function and that blank/whitespace skipping applies at every layer removes the guess.

**Proposed Addition**:
_(leave blank until discussed)_

**Resolution**: Pending
**Notes**:

---

### 4. Transport behavior for a non-positive timeout it still receives is left open

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Resolution value semantics" (`timeout` final bullet); "Single source of truth for config defaults" (transport-carries-no-defaults bullet)

**Details**:
The spec says "The transport must learn `timeout = 0` ⇒ no deadline, replacing its current non-positive → 60s re-default." Two things are underspecified at the transport boundary:

- **Zero handling mechanics.** Today every attempt runs under `context.WithTimeout(ctx, t.timeout)`. "No deadline" requires the transport to *skip* `WithTimeout` entirely (run the attempt on the parent ctx) when timeout is 0 — a structural branch, not a value swap. The spec introduces the semantic but never states the transport must conditionally apply the deadline; an implementer could wrongly pass a zero duration to `WithTimeout` (which fires immediately) and produce instant timeouts.
- **Negative at the transport.** The spec routes negatives to drop-through *in config* (floor = 60s), implying the transport should never receive a negative. But the current `NewTransport` defensively re-defaults `Timeout <= 0`. After this change, `<= 0` would collapse 0 (no-deadline) and negatives (which "can't happen") into the same branch — the spec must say whether the transport keeps a defensive negative→default floor or trusts config (so the `<= 0` guard must be split into `== 0` no-deadline vs `< 0` defensive). Left as-is, `timeout = 0` is silently swallowed by the existing `Timeout <= 0` re-default and the spec's intent breaks.

**Proposed Addition**:
_(leave blank until discussed)_

**Resolution**: Pending
**Notes**:

---

### 5. Cross-spec reconciliation sequencing is named as "decide in planning" but with no decision criteria

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Cross-spec reconciliation (commit spec)"

**Details**:
The spec correctly flags that promoting per-verb `ai_command` reverses the commit-spec's "Deliberately NOT added for commit" decision, and that the as-built `Commit` struct doc comment in `internal/config/config.go` encodes the old contract (confirmed at lines 83-89: "No push/scope/per-verb-engine override keys exist for commit (spec: 'Deliberately NOT added for commit')"). It then says "Planning must decide the sequencing: whether the commit-spec revision lands in *this* work unit or is handed to a separate commit-spec pass, and what blocks on what."

This is a genuine open decision with a hard dependency, but the spec gives planning **no criteria** to decide it and one constraint it does not reconcile: CLAUDE.md requires the doc comment stay true to as-built *in the same change* that alters the behavior. That means the moment this work unit adds `[commit].ai_command`, the `Commit` struct comment *must* be updated here regardless of where the spec-doc revision lands — so the "could be a separate pass" framing is partly contradicted by the in-repo as-built-comment rule. The spec should either (a) state that the code/comment reconciliation is in-scope here while only the external commit-spec *document* edit may be deferred, or (b) name the criteria that decide the split. As written, an implementer could defer the comment update with the spec and ship a comment that contradicts shipped code — the exact thing CLAUDE.md forbids.

**Proposed Addition**:
_(leave blank until discussed)_

**Resolution**: Pending
**Notes**:

---

### 6. Acceptance criteria for the value-semantics behaviors are implicit

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Resolution value semantics"; "Migration & mechanical carry-overs" (test-pin migration)

**Details**:
The project's test idioms ("assert exact argv / rendered lines"; behaviour-level proofs) mean each new value-semantics rule implies a specific test, but the spec enumerates the *migration* test breakages (the old `claude -p` pins, the initgen full-template-loads test) without enumerating the *new* behaviors that need positive coverage. The resolution matrix introduces several distinct, individually-testable outcomes that a planner would otherwise have to derive:

- per-key independence (overriding `ai_command` on a verb leaves that verb's `timeout` on the shared/floor),
- `ai_command = ''` at top level falling through to the floor (never empty out),
- blank `[verb].ai_command` falling to shared then floor,
- `timeout = 0` honored as no-deadline and stopping fall-through,
- negative/invalid `timeout` dropping through to the 60s floor,
- regenerate resolving through `[release]` (the "easy miss" wiring site) — argv asserted to carry the release command/timeout, not the shared/default.

These read as the implicit acceptance criteria of the feature. Stating them (even briefly) as the expected-behavior checklist turns the resolution section into directly task-able units and prevents a planner from missing the per-key-independence and regenerate-routing cases, which are the two most error-prone.

**Proposed Addition**:
_(leave blank until discussed)_

**Resolution**: Pending
**Notes**:

---
