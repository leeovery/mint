# Discussion: Token-Aware Diff Sizing & Graceful Oversized-Diff Handling

## Context

Mint's notes engine guards oversized release diffs with a raw line count (`CheckDiffSize` in `internal/notes/size.go`, default `max_diff_lines = 50000`). Under the default `on_notes_failure = abort`, an over-ceiling diff dies with a bare "diff too large" message and no remediation hint. Two facets are in scope:

1. **UX dead-end** — the abort names the cause + counts but points at none of the escape hatches (raise `max_diff_lines`, add `diff_exclude` globs, `on_notes_failure = fallback`, `--no-ai`). A big release reads as a wall.
2. **Correctness gap** — line count is a crude token proxy. The thing that must fit is the *assembled prompt* (instructions + Change Map + diff + reminder) against the model window. An under-limit diff can still overflow and fail the AI call.

The guard is shared via `notes.CheckDiffSize` / `notes.ErrDiffTooLarge` and consumed by three callers — **release**, **regenerate** (per-version skip on `--all`, forward-like on single), **commit** (generate-SKIP → `$EDITOR`). Any change must account for all three. Confirmed a single coherent feature, not an epic — re-confirmed 2026-10-16 when the scope widened to include a lean default AI command and a notes-file backdoor.

### Research standing position (to ratify / resolve here)

Research (`research/token-aware-diff-sizing.md`, incl. two deep-dives + three review passes) has matured most of the design into *emerging directions the discussion phase must ratify*:

- **mint CANNOT fit-predict** — it can't see the model window, the tokenizer, *or* the `ai_command` CLI's own wrapper overhead (Claude Code's system prompt + tool defs, empirically ~800k tokens in one live case). So any proactive ceiling is a crude **policy** cap, never a fit predictor.
- **Trigger: reactive-primary.** An **AI-as-error-classifier** (ask the same AI to map its own captured error to one of a closed code set) is the standing front-runner trigger — provider-agnostic, dissolves the string-match portability wall. A crude **proactive byte-cap** survives only as a **backstop** for silent-truncating providers (Ollama) + claude's 10 MB stdin cap. *(Amended 2026-10-16 — this cast the byte cap as a backstop only; the Proactive Size Ceiling decision makes it the replacement for `max_diff_lines`, the route that sends over-cap releases straight to chunking, and every chunk's target size — the blind-spot backstop is one of its three jobs.)*
- **Response: a graceful-degradation ladder** — full single-pass → **map-reduce** (parallel map + ONE serial reduce, *not* refine) → concat partials → existing `on_notes_failure` floor. Change Map is the size-independent **spine** handed to every map + the reduce to preserve salience.
- **Config: auto-degrade silently by default.** A new `[release]`-scoped axis (`oversize = chunk|off`, name TBD), orthogonal to `on_notes_failure`.
- **Scope: chunking is a release-notes concept.** Commit stays as-is; regenerate `--all` inherits chunking uniformly (skip stays only as its floor); forward release + single regenerate chunk cleanly.
- **Overhead-dominated failures are NOT mint-fixable** — chunking shrinks mint's prompt but every chunk re-pays the overhead; mint can only surface + advise. `--bare` is off the table (breaks subscription OAuth, live-confirmed). *(Amended 2026-10-16 — this claimed the overhead is beyond mint's reach; the Lean Default AI Command decision trims it at source for the shipped default (~105k → ~561 tokens measured, subscription login intact). The claim still holds only for repos whose own `ai_command` carries the overhead; `--bare` stays ruled out.)*

### References

- Seed: `seeds/2026-06-25-token-aware-diff-sizing.md`
- Research: `research/token-aware-diff-sizing.md`
- Discovery: `discovery/session-001.md`
- Code: `internal/notes/size.go`, `prompt.go`, `changemap.go`, `generate.go`; `internal/ai/transport.go`; `internal/commit/generate.go`; `internal/engine/release.go`

*Subtopics are documented below as they reach `decided` or accumulate enough exploration to capture. Not every subtopic needs its own section — minor items resolved in passing can be folded into their parent. The Discussion Map (which subtopics exist and their states) lives in the manifest, not this file.*

---

## AI-as-Error-Classifier as Primary Trigger

### Context

mint cannot predict whether a prompt will fit: it sees neither the model window, the tokenizer, nor the AI tool's own wrapper overhead. So the decision to break a release up has to come from the AI call's actual outcome. Today every generation failure — size overflow, auth, rate limit, refusal — lands on the one overloaded `ai.ErrGenerationFailed` sentinel and renders the one phrase `AI returned empty/invalid notes after retry` (`internal/notes/resolve.go:124-125`), which mislabels a size overflow. The transport has already fired the identical oversized call twice by then (`internal/ai/transport.go`, single bad-content retry).

### Options Considered

**String-match the provider's overflow message** (e.g. claude's `Prompt is too long`)
- Pros: no extra call; claude's message is stable and already captured on `GenerationError.Stdout`.
- Cons: provider-specific — Codex, `llm` and others word it differently, so mint re-acquires the per-provider knowledge `ai_command` was designed to avoid.

**Split-and-observe** — on any generation failure, break the release up and let the outcome say whether size was the cause
- Pros: provider-agnostic, no classify call.
- Cons: on a rate limit it fires the parallel map wave at the service already throttling; on a one-off server error it ships lower-fidelity chunked notes where a re-run would have produced a single-pass set; on auth it multiplies one failure into N.

**AI-as-error-classifier** — on `ErrGenerationFailed`, send the captured error text to the same configured AI in a small diff-free prompt asking for exactly one code from a closed set; pivot on the code
- Pros: provider-agnostic by construction (the AI interprets its own error); never sends a wave of calls at a failure chunking cannot fix; gives the ✗ line a correct cause phrase.
- Cons: one extra AI call per failure; a non-deterministic step in control flow (testable through the faked transport).

### Journey

Research reached the classifier as its standing front-runner after string-matching hit the portability wall (deep-dive 002 F6/F7) and blanket splitting proved unsafe on the overloaded sentinel (research F8). This sitting's comparison against split-and-observe turned on the parallel map wave: split-and-observe's failure cases are exactly the ones where a wave of N calls does harm.

One research claim was corrected in the comparison: the classify prompt is only "tiny" in mint's part. Every call — classify included — also carries the AI tool's own overhead (≈800k tokens in the agentic-workflows live failure). That overhead has a useful consequence: when overhead alone overflows the window, the classify call fails the same way, and mint learns "not fixable by splitting" without classifying anything — the correct stop for an overhead-dominated failure. (The overhead itself is now under its own subtopic: it turns out to be trimmable — see Lean Default AI Command.)

The classifier only ever fires on `ErrGenerationFailed`: `ErrTimeout`, `ErrCommandMissing` and `context.Canceled` are already distinguished structurally by the transport and never reach it.

The user ratified the approach on returning to the session (2026-10-16): "we go the classifier approach, we just ask Claude using the mechanism that we have in place" — i.e. the classify call goes through the existing `ai.Transport` and `ai_command`, no new AI seam.

### Decision

**Reactive classify-on-failure is the primary trigger for degradation.** On `ErrGenerationFailed`, mint sends the captured error text to the same configured AI through the existing transport and pivots on a single code from a closed set; only a size code fires the degradation ladder. Deciding factor: it is the only provider-agnostic trigger that does not amplify non-size failures into a wave of N calls. Accepted trade-off: one extra AI call per failure. Confidence: high. Code-set granularity, prompt hardening, and the proactive size cap are settled under their own subtopics. *(Amended 2026-10-16 — read "proactive backstop"; per Proactive Size Ceiling the cap is more than a backstop.)*

---

## Classifier Prompt Hardening

### Context

The classify prompt embeds the captured error text — the AI command's own stdout and stderr, which mint does not control. On a real failure it can echo diff or prompt fragments, be arbitrarily large, or read like an instruction ("ignore the above, respond OTHER"). Research asserted the classify prompt is "tiny + inert"; that holds for the code list and instruction, not for the embedded error text (research review-003 F4).

### Journey

The stakes are low and bounded: the classify call's output only picks a branch, an injected answer can at most cause a misclassification, and the OTHER catch-all absorbs an off-list one. With the lean command the classify call runs with no tools at all, so the text has nothing to act through. Still, standard untrusted-content hygiene is cheap.

### Decision

**Settled by derivation** — not discussed. Determined by standard untrusted-content-in-prompt hygiene (research review-003 F4) and the bounded stakes set by Off-List Classifier Answer and the lean command's empty tool set.

The classify prompt truncates the captured error text to a bounded length and fences it as captured tool output, explicitly not instructions, before embedding it. The bound itself is specification detail.

Sibling check: no overlap found — this topic's own classifier ground.

---

## Classifier Result Alongside the Verbatim Error

### Context

Research framed the classifier as *replacing* today's raw-error passthrough with a normalised message. Today the ✗ line for every generation failure reads `AI returned empty/invalid notes after retry` (`internal/notes/resolve.go:124-125`) — wrong for a size overflow — with the AI's captured output rendered verbatim beneath it (`notesFailureOutput`, `internal/engine/release.go:1656`).

### Journey

Replacing the captured output would break the project contract that subprocess failure diagnostics travel verbatim in `StageFailure.Output`, never summarised or reworded (CLAUDE.md, Error & exit idioms). The AI's own text is also the most specific remediation available — `Not logged in · Please run /login`, or claude's token breakdown on an overflow — so dropping it would lose information.

### Decision

**Settled by derivation** — not discussed. Determined by CLAUDE.md's verbatim-diagnostics rule.

The classifier adds to the verbatim error, never replaces it: its code drives control flow (split or stop) and selects the ✗ line's cause phrase and mint's guidance, while the AI's captured output still renders verbatim beneath the ✗ line.

---

## Off-List Classifier Answer

### Context

The research sketch had mint re-prompt the classifier more firmly when its answer is not one of the listed codes, then fall back to surfacing the error.

### Journey

CLAUDE.md's AI seam makes the transport the sole owner of validation and the single bad-content retry — "consumers never re-retry". A firmer second classify call would be exactly such a consumer re-retry. The mandatory OTHER catch-all already guarantees safe termination, so a re-ask buys nothing the catch-all does not.

### Decision

**Settled by derivation** — not discussed. Determined by CLAUDE.md's AI seam (the transport owns the single retry; consumers never re-retry).

No firmer re-ask: a classifier answer that is not exactly one listed code maps straight to OTHER. Worst case is surfacing the error where auto-chunking might have helped.

---

## Closed Code Set: Minimal vs Rich Granularity

### Context

The classifier returns one code from a closed set; mint pivots on it. Only about three control-flow reactions exist however many codes are defined — split (size), retry/back off (rate limit, transient), surface-and-stop (auth, quota, refusal, unknown) — and codes sharing a reaction should merge. An OTHER/UNKNOWN catch-all is mandatory.

### Options Considered

**Minimal — SIZE / OTHER**
- Pros: least to get wrong; SIZE fires the degradation ladder, OTHER stops with the AI's verbatim message plus mint's general guidance. Extensible later at no cost to existing codes — anything unrecognised already falls to OTHER.
- Cons: no automatic recovery from a rate limit hit mid-way through the parallel chunk calls.

**Rich — SIZE / RATE_LIMIT / AUTH / TRANSIENT / QUOTA / REFUSAL / OTHER**
- Pros: per-cause wording; a wait-and-retry on rate limits.
- Cons: most codes add no distinct reaction. TRANSIENT has already had the transport's single retry, and retrying again breaks the "consumers never re-retry" rule. AUTH/QUOTA/REFUSAL all surface-and-stop, and the AI's own verbatim message (now always rendered — see Classifier Result Alongside the Verbatim Error) is more specific than mint-written wording, e.g. `Not logged in · Please run /login`.

### Journey

The comparison reduced to one candidate extra code: RATE_LIMIT, the only one with a distinct reaction (wait, then retry the affected chunks). Whether it pays depends on the auth model: on a Claude subscription a usage limit is a window measured in hours, which a CLI run should not sit out; on an API key a 429 clears in seconds and parallel map waves are exactly what trigger it. A wait-and-retry would also be a deliberate exception to the no-re-retry rule — not worth making before the case is observed. The subscription is the case to design for.

### Decision

**Ship SIZE / OTHER.** SIZE fires the degradation ladder; OTHER stops, showing the AI's verbatim message with mint's general escape hatches. Deciding factors: no other code has a reaction worth its cost for the subscription case, and the set extends additively — a RATE_LIMIT code (with its wait-and-retry exception) can be added later if API-key setups hit throttling mid-wave. User agreed 2026-10-16. Confidence: high.

---

## Proactive Size Ceiling

### Context

Today the post-exclusion diff is checked against `max_diff_lines` (default 50,000, shared top-level key): over it, a release aborts, `regenerate --all` skips that version, and `mint commit` drops to `$EDITOR` (`internal/notes/size.go`). Aborting cannot survive this feature's goal — AI notes however big the release. The classifier is now the primary trigger, so a proactive ceiling needs a job of its own to keep existing. Line count is also a poor proxy: the notes-failure work saw an 8.6k-line, ~867 KB byte-dense diff slip under the line ceiling.

### Options Considered

**A. Same line ceiling, new meaning for releases** — over 50,000 lines a release goes straight to chunking instead of aborting; commit unchanged.
- Pros: smallest change; no config rename.
- Cons: lines say little about size — one minified or generated line can be any size; gives the chunker no size to aim at.

**B. A per-call size cap in bytes** — no single AI call mint makes is ever bigger than the cap.
- Pros: gives the chunker its target size; tracks what the AI actually receives; the only guard where nothing reactive works.
- Cons: a config key rename and a meaning change.

**Operator-set token budget / model registry** (research)
- Not pursued: mint cannot see the window, the tokenizer, or the tool's overhead, so any token figure is a guess dressed as precision; a registry adds a maintained table that goes stale every model release. A byte cap is honest about being a size policy rather than a fit prediction.

### Journey

B won on three grounds:
- **The chunker needs a target size.** Without one it can only halve after each failure — whole, fail, halves, fail, quarters. Cheap with claude (its overflow rejection is instant and client-side), but a provider that takes a full round trip to say no turns each halving into real delay.
- **Bytes over lines.** A cap in bytes tracks the payload; a line count does not.
- **Where reactive detection is blind.** Research found claude has a hard 10 MB stdin limit, and silent-truncating providers (Ollama) return plausible success, so the classifier never fires. A per-call cap is the only guard there.

The rename cost is low: mint has no users outside the user's own repos yet, so it costs the README tables, the config-metadata row and their drift tests.

### Decision

**`max_diff_lines` is replaced by a per-call size cap measured in bytes** (new key name and default value are specification detail). It bounds the composed prompt mint sends in any single AI call — instructions, Change Map, diff and reminder; the AI tool's own overhead is invisible to mint and outside it. It stays a shared top-level key covering both verbs.

- **Release (forward and single regenerate):** a prompt over the cap skips the single-pass call and goes straight to chunking; every chunk call is sized to fit under the cap. (A single file whose diff alone exceeds the cap is stood in for — see Chunk Boundaries and the Single-File Floor.)
- **`regenerate --all`:** inherits the same behaviour; its per-version skip remains only as the floor when chunking is exhausted.
- **`mint commit`:** over the cap drops to `$EDITOR` as today — commit has no chunking.

The cap remains a policy, never a fit prediction: a prompt under it can still overflow, which is the classifier's case. User agreed 2026-10-16. Confidence: high on the shape; the default value needs choosing in specification with the lean command's small overhead in mind.

Sibling check: notes-failure-output-ugly-and-uninformative — its specification lists a byte/token-aware ceiling as deliberately out of scope and promotable to its own work later, so it anticipates this decision; nothing is owed. mint-release-tool's specification introduced `max_diff_lines` (shared-only, default 50000) and ai-model-selection kept it shared-only; this decision supersedes the key once it lands.

---

## Classify Call Failure

### Context

The classifier decisions define what mint does with every *answer* — SIZE, OTHER, or an off-list answer mapped to OTHER — but not what happens when the classify call itself fails: a generation failure, a timeout, a missing binary, or a Ctrl-C while it runs.

### Journey

The classifier Journey already reasoned that a failed classify call is the stop — when the tool's overhead alone overflows the window, the classify call fails the same way, and mint learns that splitting cannot fix it. The off-list rule settles the shape: anything short of a clean listed code resolves to OTHER. The verbatim-error decision settles what renders: the output of the failure being explained, which is the original generation call's, not the classify call's.

### Decision

**Settled by derivation** — not discussed. Determined by the classifier Journey (a failed classify call is the stop), Off-List Classifier Answer, Classifier Result Alongside the Verbatim Error, and CLAUDE.md's seam 5 (a cancel is not an AI failure) (review-001 F4).

Any classify-call failure — generation failure, timeout, or missing binary — takes the OTHER path: no split, OTHER's cause phrase on the ✗ line, and the *original* generation call's captured output rendered verbatim beneath it. A `context.Canceled` during the classify call propagates unchanged and never falls to `on_notes_failure`.

Sibling check: no overlap found — the classify call and its codes are this topic's own terms.

---

## Empty Error Output

### Context

Some generation failures carry no text at all — a tool that exits non-zero silently, or exits 0 with an empty body and nothing on stderr. The classifier decision sends "the captured error text" to the AI regardless, and the transport keeps stdout and stderr as separate fields, with diagnostics sometimes arriving only on stderr behind a whitespace-only body (`internal/ai/transport.go`, `GenerationError`).

### Journey

Classifying empty input can only be a guess; a guessed SIZE would fire the whole parallel chunk wave at a failure chunking cannot fix — exactly the amplification the classifier was chosen to prevent. Sending stdout alone would break the provider-agnostic premise for any tool that writes its errors to stderr. Distinct from the pending prompt-hardening subtopic: that covers how the text is embedded; this covers which text, and the case where there is none.

### Decision

**Settled by derivation** — not discussed. Determined by the classifier's deciding factor (never amplify non-size failures into a wave of N calls), Off-List Classifier Answer (unreliable answers resolve to OTHER), and the transport's separate captured streams (review-001 F5).

The classifier's input is both captured streams, stdout and stderr. When both are empty or whitespace-only, mint skips the classify call and takes the OTHER path directly.

Sibling check: no overlap found — this topic's own classifier ground.

---

## Transport Retry Before the Classifier

### Context

The transport retries once on any bad-content failure, including a non-zero exit (`internal/ai/transport.go`). So on an overflow under the size cap, the identical oversized call is sent twice before the classifier ever looks; and on a persistent non-size failure the classify call — through the same transport — also gets its built-in retry, up to four AI calls before the ✗ line. Raised by the background review (review-001 F6).

```text
1. notes call           → "Prompt is too long"
2. same call again      → "Prompt is too long"   (built-in single retry)
3. classify call        → SIZE
4. → chunk
```

### Options Considered

**Keep the transport's single retry as it is**
- Pros: one uniform retry rule for every AI call (CLAUDE.md seam 5: the transport owns the single retry, consumers never re-retry).
- Cons: the repeat of an oversized call can never succeed.

**Classify before any retry** — on the first failure, ask the classifier; SIZE chunks with no repeat, OTHER gets the one retry
- Pros: no wasted repeat on an overflow.
- Cons: whether the transport retries would depend on which command is calling — notes skipping it, commit keeping it.

### Journey

On the design target the waste is close to nothing: research found claude rejects an oversized prompt client-side before sending anything, and the per-call size cap already sends most oversized releases straight to chunks, so an under-cap overflow — the only case that pays the repeat — is the rarer path. What would change the call: providers whose rejection costs a full round trip, where the repeat becomes real wasted time on every overflow.

### Decision

**The transport's single retry stays as it is, for every AI call including the classify call.** Deciding factor: on claude the repeat is near-free and the size cap makes it rare, so the saving does not justify making the retry rule depend on the calling command. User agreed 2026-10-16. Confidence: medium-high — revisit if non-claude providers with server-side rejection become a target.

---

## Lean Default AI Command

### Context

Research concluded the AI tool's wrapper overhead is the operator's environment and not mint-fixable: `--bare` strips it but forces API-key auth and breaks the subscription OAuth the shipped default relies on (live-confirmed: `Not logged in · Please run /login`). That conclusion made overhead-dominated failures a surface-and-advise case only.

### Journey

Measured 2026-10-16 in the mint repo (claude CLI flags from `claude --help`), same one-word prompt, `usage` read from `--output-format json`:

- `echo "Reply with the single word: ok" | claude -p --model sonnet --output-format json --no-session-persistence` → 105,188 input tokens (cache_creation 105,186 + input 2).
- `echo "Reply with the single word: ok" | claude -p --model sonnet --safe-mode --tools "" --strict-mcp-config --system-prompt "You write release notes." --output-format json --no-session-persistence` → 561 input tokens, answered `ok` on the subscription login — no auth break.

`--safe-mode` disables CLAUDE.md, skills, plugins, hooks and MCP servers while (per its help text) leaving auth and model selection working; `--tools ""` drops the built-in tool definitions; `--system-prompt` replaces Claude Code's own system prompt. That is roughly a 190× cut of the overhead without touching OAuth — the research's "not mint-fixable" holds for `--bare` only.

Consequence for the live agentic-workflows failure: mint's own prompt there was ~532k tokens against a 1M limit, and the request failed only because ~803k of overhead rode on top. With the lean command it would have fitted in one pass.

### Decision

**The shipped default `ai_command` becomes the lean claude form** — `claude -p --model sonnet` plus `--safe-mode --tools "" --strict-mcp-config --system-prompt "…"` (exact flag set and system-prompt wording are specification detail). The user approved it 2026-10-16. Deciding factor: it removes the overhead-dominated failure class for the shipped default at its source, keeps the subscription login working, and is the single cheapest change toward big releases succeeding in one pass — chunking remains for releases whose own prompt is genuinely too big.

Consequences accepted:
- The shipped default is shared by both verbs, so `mint commit` gets the lean command too — it needs no tools or CLAUDE.md either, and a smaller overhead only shortens its calls.
- It reaches only repos that have not set their own `ai_command`; an explicit `ai_command` is untouched.
- Model choice is unchanged — Sonnet, alias form.

**CLAUDE.md and Claude Code's system prompt are dropped on purpose** (review-001 F3). `--safe-mode` switches off every CLAUDE.md that today loads into mint's calls (the repo's and the user's global), and `--system-prompt` replaces Claude Code's own system prompt. Instructions kept there that currently shape notes or commit messages stop doing so. The user called this a positive side effect, not a cost (2026-10-16): project and global CLAUDE.md "should never really have been included" in writing release notes — they are noise, as is Claude Code's system prompt. The goal is a raw ask of the model carrying only mint's own clear instructions, so mint controls the output strongly. This follows mint's founding rule that mint owns the prompt and the command is only transport (mint-release-tool specification). Conventions an operator wants applied belong in mint's own `[release]`/`[commit]` `context` and `prompt` keys, which ride in every call; the change is documented alongside the lean default.

**System prompt wording** — **Settled by derivation** — not discussed. Determined by the shared shipped default: one `DefaultAICommand` serves release, commit and the classify call (review-001 F2). The shipped `--system-prompt` names no task — neutral wording to the effect of "follow the instructions you are given and output only what they ask for" — and each call's own prompt carries its job. Exact wording is specification detail.

**Older claude versions** — **Settled by derivation** — not discussed. Determined by ai-model-selection's raw `ai_command` design (mint holds no per-provider knowledge). A claude too old for `--safe-mode` rejects the unknown option and exits non-zero; that lands on the normal generation-failure path; the classify call runs the same command and fails the same way, so mint takes the OTHER path (Classify Call Failure) and claude's own "unknown option" message renders verbatim. mint adds no version probe and no fallback to the plain command — either would put claude-specific knowledge into mint.

Sibling check: ai-model-selection — its specification pins the shipped default as `claude -p --model sonnet` (alias form, Sonnet, rationale: aliases track model versions without rebuilds). This decision keeps that model and alias and adds flags; it supersedes the command string once it lands.

---

## AI Command Quoting

### Context

The lean default was measured by typing it into zsh, which handles quotes. mint does not: it splits `ai_command` on whitespace with `strings.Fields` and handles no quoting (`internal/ai/transport.go`, `parseCommand`; its comment says the command "carries no quoting or shell syntax"). Two decided flags need quoting, so as written the lean default reaches claude garbled on every release, commit and classify call:

```text
what we decided:   --tools ""          --system-prompt "You write release notes."
what claude gets:  --tools '""'        --system-prompt '"You'  'write'  'release'  'notes."'
```

An operator copying the lean form into their own `ai_command` hits the same wall. Raised by the background review (review-001 F1).

### Options Considered

**Quote-aware parsing of `ai_command`**
- Pros: the lean form — and any argument containing spaces — becomes expressible by operators, not only by the shipped default. Backward compatible in practice: every working `ai_command` today has no quotes in it, because quotes never worked.
- Cons: changes how mint reads every `ai_command`, an as-built contract from the earlier model-selection work.

**Choose only flags that need no quoting**
- Pros: parser untouched.
- Cons: gives up part of the overhead cut, and operators still cannot express multi-word arguments.

### Journey

The deciding case was the operator writing the lean form (or an `--append-system-prompt "…"`) in their own `ai_command`: only quote-aware parsing makes that work. The parser change was put to the user because it reaches beyond this topic into the model-selection work's as-built contract.

### Decision

**mint reads quotes in `ai_command` the way a shell does** — single and double quotes group words into one argument, and `""` yields a genuinely empty argument. Quoting only: no variable expansion, globbing, pipes or other shell syntax, and mint still runs the binary directly through the runner seam, never through a shell. User approved 2026-10-16. Confidence: high.

Sibling check: ai-model-selection — its specification makes `ai_command` a raw full command string and does not specify how it is split; the whitespace split is as-built only (the transport's `parseCommand` comment), and this decision supersedes that comment when it lands. mint-release-tool's specification defines `ai_command` as an overridable command string, also without a split rule.

---

## Commit AI-Failure Output

### Context

On `mint commit`, any AI generation failure prints only `could not generate a message, opening editor` and opens `$EDITOR` (attended), or fails loud (unattended) — the AI's captured output is dropped (`internal/commit/run.go`, `aiFailWarnMessage` and the `isAITransportFailure` branch). Release, by contrast, renders it verbatim under the ✗ line. The lean default reaches commit (shared shipped default), and the classifier decision named no verb scope. Raised by the background review (review-001 F7), motivated by a claude too old for the lean flags failing every commit with no hint why; the user is not concerned about old claude versions (2026-10-16), but the gap stands for any commit failure — an expired login, a usage limit.

### Journey

The verbatim-diagnostics rule (CLAUDE.md: subprocess failure diagnostics travel verbatim in `StageFailure.Output`/`Warning.Output`) already covers the warning commit emits; commit is the one site that drops the output. The classifier's only distinct reaction is the degradation ladder, which commit does not have — every commit failure already ends in the editor — so a classify call there would only delay the editor by one AI call.

### Decision

**Settled by derivation** — not discussed. Determined by CLAUDE.md's verbatim-diagnostics rule and the research finding that chunking is a release-notes concept, with commit's failure path ending in the editor whatever the cause (review-001 F7).

- `mint commit`'s AI-failure warning carries the AI's captured output verbatim, as release already does.
- The classifier is release-notes only (forward release and regenerate); `mint commit` makes no classify call.

---

## Chunk Boundaries and the Single-File Floor

### Context

Once a release goes to chunking, its files are packed into chunk calls under the per-call size cap. Packing by file cannot shrink a single file: when one file's diff alone exceeds the cap, no chunk can carry it. The live agentic-workflows release carried a 1.9 MB committed design-session transcript — the same pathology as mint's own 867 KB `.workflows/`/`.tick/` artifact case.

### Options Considered

1. **Stand in for it** — the chunk gets a one-line stand-in instead of the diff (e.g. "`sessions/design.jsonl` — 1.9 MB changed, diff omitted (over the size cap)"); the Change Map still lists the file and its size so the notes can mention it if salient; after the notes, mint names the skipped file and suggests `diff_exclude`.
2. **Split it into pieces** — cut the file's diff across several chunks; nothing dropped, but several AI calls spent reading what is almost always generated noise.
3. **Truncate it** to fit — an arbitrary half-read.

### Journey

The lean rested on one claim: a single file bigger than a whole call's budget is almost always a generated artifact whose content summarises to nothing — true of both live cases (the 1.9 MB transcript, mint's own 867 KB workflow artifacts). The user was asked whether they ever commit a single hand-written file that size and expect the notes to describe its contents, and picked the stand-in.

### Decision

**A file whose diff alone exceeds the per-call size cap is represented by a one-line stand-in, not its diff.** The stand-in names the file and how much changed and says the diff was omitted for size; the Change Map still carries it, so the notes can mention it where it matters. After the notes, mint tells the operator which file(s) it stood in for and suggests adding them to `diff_exclude`. User chose this 2026-10-16. Deciding factor: such a file is almost always a generated artifact, so reading it costs AI calls for nothing. Confidence: high.

---

## Notes-File Backdoor

### Context

The user's current workaround for a release mint cannot generate notes for (2026-10-16): in a Claude session, have Claude write the notes to mint's standard and put them in `[release].fallback` (the fixed fallback body shared by `--no-ai` and `on_notes_failure = fallback`), run `mint release --no-ai`, then have Claude remove the body from config again — an extra clean-up commit on main for every such release. `mint release` has no way to take a ready-made body without a config edit (no notes-file or stdin flag in `cmd/mint/usage.go`'s `releaseUsage`).

Folded into this feature 2026-10-16 as the remediation floor: mint takes a finished notes body directly (`--notes-file`), so the workaround becomes "write a file, release with it" with no config edit and no clean-up commit. *(Amended 2026-10-16 — this offered stdin as a route and called the design undiscussed; the Decision below settles a file path only, the gate, the `--no-ai` conflict and bad-file handling. `on_notes_failure` never applies to a supplied body: it governs only an AI failure, and no AI call is made.)*

### Journey

**Where the body comes from** (raised by the background review, review-001 F9). The first real use collides with mint's own gates: an agent writes `notes.md` to the repo root and runs `mint release -y --notes-file notes.md`, and the clean-tree preflight refuses it as an untracked file (`internal/preflight/preflight.go`). With `--autostash`, `git stash push --include-untracked` would sweep the file away before mint reads it (`internal/engine/autostash.go`). And the release path's pre-tag artifact commit stages with `git add -A` (`record.CommitDirtyTree`, `internal/record/commit.go`), so an untracked notes file in the tree would be swept into a release commit — keeping the file out of git is in mint's realm, not only the caller's. A stdin form was weighed and dropped: the review gate reads its keypresses from stdin, and an agent can always write a file.

The alternative — keep the clean-tree rule absolute and require the file to live outside the repo or in an ignored path — was set against the trap it leaves: the obvious first try fails. The user expects an agent will usually write the file to a temp directory anyway, but it still has to work from the repo root (2026-10-16).

### Decision

**The named notes file is excluded from everything mint does** (user, 2026-10-16):
- mint reads it once at startup, before any stash, check or gate, and holds the body in memory.
- It does not count against the clean-tree check, is left out of `--autostash`'s stash, and is never staged or committed by any git add/commit mint runs — the pre-tag artifact commit's `git add -A` included.
- Afterwards it sits in the work tree untouched; cleaning it up is the caller's business, out of mint's scope.
- File path only — no stdin form.

**Release only** (user, 2026-10-16): `mint commit` does not get a message-file backdoor — it keeps its two message sources, the AI or `$EDITOR`. An agent with its own commit message gains little from `mint commit` beyond its staging and push flags, `git commit -F` already serves it, and commit's `-A` staging (`git add -A`) is where an excluded file would be riskiest.

**How a supplied body moves through the release** (raised by the background review, review-001 F10; agreed by the user 2026-10-16):
- **Review gate** — still shown on an attended run so the operator sees what will ship, offering accept, abort and edit; regenerate (`r`) is dropped, since there is no AI output to redo. Under `-y` it auto-accepts as usual (inside the presenter, per the gate seam).
- **Body** — passes into the release and changelog byte-identical, exactly like an AI body; the size cap, chunking and AI are all skipped.
- **With `--no-ai`** — giving both is a usage error: two answers to where the notes come from.
- **Bad file** — an empty or whitespace-only file is refused; a missing or unreadable one fails before any step runs.
- **Regenerate** — no notes-file in this feature: chunking now covers its oversized versions, and its `tag`/`release` sources already rewrite notes from an existing body.

**Precedence over mint's stand-in bodies** — **Settled by derivation** — not discussed. Determined by this section's decision that the supplied body passes into the release byte-identical, and by the purpose of the first-release and degenerate (empty post-exclusion diff) guards in `internal/notes/select.go` — they produce an honest body when there is nothing for an AI to summarise, which a supplied body does not need (review-002 F3). A supplied notes file wins over both guards: whenever the file is given, its body is the release's body — never `Initial release.` or the maintenance stub. (Today those guards run ahead of `--no-ai` and the AI, which is why the `[release].fallback` workaround loses to them.)

---

## Summary

### Key Insights

1. **mint cannot predict fit, so it reacts and caps.** The classify-on-failure trigger responds to the AI's actual outcome; the per-call byte cap is an honest size policy that also gives the chunker its target — neither pretends to know the model's window.
2. **The AI tool's own overhead was the dominant cost, and it is trimmable.** The lean claude command cuts it ~190× on the subscription login; the live agentic-workflows overflow would have fitted in one pass. The cheapest lever turned out to sit outside chunking entirely.
3. **mint owns the prompt.** Dropping CLAUDE.md and Claude Code's system prompt is a feature, not a cost: a raw ask carrying only mint's instructions is what lets mint control its output.

### Open Threads

- **Parked on the roadmap under Next (2026-10-16): headline-first release notes.** The default prompt's category-first format and its "NO TL;DR, NO summary paragraph, NO preamble" rule (`internal/notes/prompt.go`) leave no place to declare what a release is about, so a release's main feature can land as one bullet among sideline changes. Separate from big-release handling; the chunked merge pass inherits whatever format exists when it ships.
- **Parked on the roadmap under Next (2026-10-16): agent-authored release notes.** Claude can already run `mint release -y` from a session, with mint's own AI call writing the notes; what is missing is a way for the outer agent to write notes to mint's standard — mint handing it the instructions and Change Map, or taking a one-line statement of what the release is about (the session that built the feature knows the headline that mint's diff-only view misses). The receiving half — taking a finished body — is this feature's notes-file backdoor.

### Current State

- Decided: the detection trigger in full — reactive classify-on-failure as primary, a per-call byte cap as the proactive guard, and the classifier's edges (codes, failures, empty output, hardening, retry).
- Decided: the shipped default AI command becomes the lean claude form.
- Decided: `max_diff_lines` becomes a per-call byte cap; over it, releases chunk and commit drops to the editor.
- Decided: a single file too big for any one call is represented by a stand-in line, with a `diff_exclude` hint afterwards.
- Decided: a release-only `--notes-file` backdoor; the named file is excluded from every git step mint runs.
- Decided: the classifier's codes are SIZE / OTHER; its result renders alongside the AI's verbatim error; off-list answers map to OTHER.
