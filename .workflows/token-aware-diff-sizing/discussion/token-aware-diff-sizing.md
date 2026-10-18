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
- **Response: a graceful-degradation ladder** — full single-pass → **map-reduce** (parallel map + ONE serial reduce, *not* refine) → concat partials → existing `on_notes_failure` floor. Change Map is the size-independent **spine** handed to every map + the reduce to preserve salience. *(Amended 2026-10-17 — per When the Merge Fails there is no concat rung: a failed merge saves the per-chunk notes and falls to `on_notes_failure`. The Change Map is not strictly size-independent — it lists renames and removals per path; see Stitching and Salience.)*
- **Config: auto-degrade silently by default.** A new `[release]`-scoped axis (`oversize = chunk|off`, name TBD), orthogonal to `on_notes_failure`. *(Amended 2026-10-18 — per the Oversize Knob revision there is no setting: chunking is always on, and the auto-degrade is unconditional.)*
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

**What OTHER stops** — **Settled by derivation** — not discussed. Determined by Classify Call Failure (only a `context.Canceled` is exempted from `on_notes_failure`, an exception that only reads as one if the OTHER path does fall to it), the ladder ending at the existing `on_notes_failure` floor, the new oversize knob being orthogonal to `on_notes_failure` (the knob has since been dropped — Oversize Knob, revised 2026-10-18; the call stands on the other grounds), and CLAUDE.md seam 5 (review-002 F6). OTHER ends the degradation, not the run's notes policy: no split, then `on_notes_failure` applies exactly as today — `abort` renders the ✗ line with OTHER's cause phrase and the AI's verbatim output; `fallback` ships the fallback body. "Stops" above means stops degrading.

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

- **Release (forward and single regenerate):** a prompt over the cap skips the single-pass call and goes straight to chunking; every chunk call is sized to fit under the cap. (A single file whose diff is too big to fit in one chunk call alongside that call's instructions, Change Map and reminder is split into pieces across several chunk calls — see Chunk Boundaries and the Single-File Floor. *Amended 2026-10-17 — this read "whose diff alone exceeds the cap"; the cap bounds the whole composed prompt, per this Decision, and every call carries the full Change Map, per Stitching and Salience. Amended again 2026-10-17 — this read "is stood in for"; per the Chunk Boundaries revision there is no stand-in.*)
- **`regenerate --all`:** inherits the same behaviour; its per-version skip remains only as the floor when chunking is exhausted.
- **`mint commit`:** over the cap drops to `$EDITOR` as today — commit has no chunking.

The cap remains a policy, never a fit prediction: a prompt under it can still overflow, which is the classifier's case. User agreed 2026-10-16. Confidence: high on the shape; the default value needs choosing in specification with the lean command's small overhead in mind.

**One call after stand-ins** — **Settled by derivation** — not discussed. Determined by the merge's purpose in Stitching and Salience (ranking across chunks that each read only a slice — a lone call reads the whole release), Map-Reduce's parallel-first steer (no serial chain of AI calls), and this Decision skipping the single-pass call only because the prompt is over the cap (review-003 F4). When a release over the cap fits under it once its oversized files are stood in — the agentic-workflows case, over the cap only because of one 1.9 MB transcript — mint makes that one call as an ordinary single-pass call with the stand-ins in place: no chunk prompt and no merge. The stand-in hint still follows the notes, and the notes are single-pass notes, so `r` regenerates them as on any single-pass release. *(Amended 2026-10-17 — per the Chunk Boundaries revision there are no stand-ins. The point now falls out of the order of steps: the size check runs after the exclusion check (Exclusion Suggestions Before the Notes), so a release brought under the cap by its exclusions — the transcript excluded at the pause — is measured as under the cap and gets an ordinary single-pass call.)*

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

## Oversize Knob

### Context

In research the user asked for a setting deciding how a too-big release is handled — "break it up" (chunk) versus not — defaulting to breaking it up, with the name left open ("we can give it a better name"). Research shaped it as a new axis beside `on_notes_failure`: `on_notes_failure = abort | fallback` is a failure *response*, chunking is failure *avoidance* that still aims at real AI notes, and after chunking bottoms out a response is still needed — so they compose rather than merge.

### Journey

**Naming.** The user's own word for the non-default value was "fail". But switching chunking off does not decide failure — `on_notes_failure` still does — and a value called "fail" would make two settings look like they both decide it. "off" says exactly what it does: no chunking, defer to `on_notes_failure`.

**Default.** Research recorded the user's call (2026-06-26) to auto-degrade silently by default — "it just handles it" — rather than stop and ask for consent to the extra AI calls. "Silent" means no consent prompt; how a chunked run is narrated on screen is the progress-narration subtopic's ground.

### Decision

#### 2026-10-18 — revised
*Trigger: review finding — what `off` switches off once the ladder had grown (over-cap chunking, pieces, halving, the merge, the classify call, the exclusion check) (review-003 F14); defining it led the user to ask whether chunking needs a setting at all.*

**No setting: chunking is always on.** The `oversize` key is dropped. Deciding factors: chunking only happens when a release cannot go in one call, so the one thing `off` changes is that such releases get no AI notes — not an outcome anyone wants as a standing policy, and anyone wanting something other than AI notes for a given release already has `--no-ai` and `--notes-file`. And a key is cheap to add and costly to remove: each costs a schema entry, README and config-metadata rows and a setup-guide line, and under strict config a repo carrying the key would fail to load if it were later removed — whereas adding it if a real case appears (an API-key setup where the extra calls cost money, say) is additive. The per-call size cap stays: it is the target every call is sized to, and the setting a smaller-window model needs. What changes from the entry below: no `oversize` key, no `off` path, and the classifier-under-off reasoning no longer applies; the classify call runs on every release-notes failure as decided. User agreed 2026-10-18. Confidence: high.

#### 2026-10-17
**The setting is `oversize`, with values `"chunk"` and `"off"`; `"chunk"` is the default.** User agreed 2026-10-17, confirming chunk as the default. Confidence: high.

**Where it lives and what off does** — **Settled by derivation** — not discussed. Determined by the user's research request, research's response-vs-avoidance framing, and Commit AI-Failure Output (commit has no chunking; regenerate rides `[release]`). `oversize` is a new `[release]` key, separate from `on_notes_failure`: it decides whether mint degrades, not what happens once degradation ends. Under `"off"` behaviour is today's — a prompt over the per-call cap, or a SIZE answer from the classifier, goes straight to `on_notes_failure` with no chunking. As a new key it takes the strict-config path (schema, friendly type error, config-metadata row, README tables).

**The classifier under off** — **Settled by derivation** — not discussed. Determined by the classifier's two jobs — control flow and the ✗ line's cause phrase (Classifier Result Alongside the Verbatim Error). With `oversize = "off"` the classify call still runs (a few hundred tokens under the lean command): a SIZE answer cannot start chunking, but it gives the ✗ line the true cause and lets mint's guidance say chunking is switched off.

---

## Remediation at the Floor

### Context

Facet 1 of the seed: when notes cannot be AI-generated, mint dies with the cause and no way forward — the escape hatches (`diff_exclude`, `on_notes_failure = fallback`, `--no-ai`, and now `--notes-file` and `oversize`) all exist, but nothing points at them, and an attended operator has to re-run the whole release with a different flag. With the lean default and chunking, the floor is reached far less often — when chunking is exhausted, the login has expired or a usage limit hit, or `oversize = "off"` — but it is still reached. *(Amended 2026-10-18 — per the Oversize Knob revision there is no `oversize` setting, so neither the escape hatch nor the `off` route to the floor exists.)* In research the user asked for something actionable: either name the next moves, or offer choices inline ("run without AI?" → write it in an editor, or fall back to commit messages). Research also found the commit-message route already exists: `--no-ai`, or `on_notes_failure = fallback` with an empty `fallback`, builds the body from the commit-subject list (`internal/notes/noai.go`). Release's editor seam only edits an already-produced body (the review gate's `e`); there is no write-from-scratch-on-failure path today.

### Options Considered

**1. Guidance, then stop** — the ✗ line and the AI's verbatim message, then a short cause-fitted list of next moves (for size: `diff_exclude` for generated paths, `--notes-file`, `--no-ai`); the operator re-runs with one.
- Pros: no new release route; works identically attended or not.
- Cons: every floor hit at a terminal repeats the whole prepared release to change one input.

**2. Ask on the spot** — the same message, then a one-keypress choice: `c` commit-subject notes, `e` write the notes in the editor (opened on the saved per-chunk notes when a merge failed, empty otherwise), `a` abort; the release finishes in the same run.
- Pros: the prepared release is not thrown away.
- Cons: a new release route — the editor opening to write from scratch.

### Journey

Attended versus unattended governs it (CLAUDE.md: fail loud, never hang): under `-y` or a non-TTY nobody can answer, so option 2 can only apply at a terminal. The deciding reason for option 2 there: by the time notes fail, the release is fully prepared, and re-running repeats every step to change one input. The saved per-chunk notes from a failed merge make `e` especially cheap — the operator tidies rather than writes.

### Decision

**At a terminal, the floor asks; everywhere else it gives guidance and stops.** User agreed 2026-10-17. Confidence: high.

- **Attended, `on_notes_failure = abort`:** after the ✗ line and the AI's verbatim message, a one-keypress choice — `c` commit-subject notes (the existing `--no-ai` body), `e` write the notes in `$EDITOR` (opened on the saved per-chunk notes when a merge or a chunk failed, empty otherwise — a new write-from-scratch route), `a` abort. The chosen body then continues through the release.
- **Unattended (`-y` or non-TTY), `on_notes_failure = abort`:** the ✗ line, the AI's verbatim message and a short cause-fitted list of next moves, then the run stops — never a prompt.
- **`on_notes_failure = fallback`:** no choice is offered; the fallback body ships, as the operator chose.
- The choice goes through the presenter's gate seam like every other gate.

**Commit-message fallback reuse** — the `c` choice is the existing commit-subject body (`internal/notes/noai.go`), surfaced as a choice rather than built anew.

**`c` is always the commit subjects** (raised by the background review, review-004 F7; user agreed 2026-10-18). As built, the `--no-ai` body returns a configured `[release].fallback` verbatim and builds the commit-subject list only when none is set (`resolveFallbackBody`, `internal/notes/resolve.go`), so a repo with a fixed fallback under `abort` would get that string from a key labelled commit-subject notes. `c` gives this release's commit subjects whether or not a fallback is set — it is chosen in the moment, and the subjects describe this release where a fixed string does not. The configured fallback stays what `--no-ai` and `on_notes_failure = fallback` ship. Accepted cost: a configured fallback body is not among the floor's choices.

---

## Scope Across the Three Consumers

### Context

The size guard is shared by three consumers — forward release, regenerate (single and `--all`), and `mint commit`. Research read each consumer's too-big handling from code: chunking is a release-notes concept (a commit message is one tight message with a fast `$EDITOR` fallback); forward release and single regenerate take chunking cleanly through the shared generator; `regenerate --all` inherits it uniformly — the user's steer (2026-06-26) being that only the odd version in a batch is oversized, so no special-casing — with its skip-and-continue kept only as its floor. Commit's share of this feature is its failure output (Commit AI-Failure Output); its size behaviour is set by the Proactive Size Ceiling decision (over the cap → `$EDITOR`). Two consumer edges remained: the dry-run notes cache and `--all`'s skip reason.

### Journey

Research flagged that non-deterministic chunked output might make a real run miss the cache a dry run filled. The cache keys on inputs, not output — `notescache.Key(diff, version, instructions)` hashes the post-exclusion diff, the version and the instructions (`internal/notescache/cache.go`) — so a dry run's chunked body is stored and offered back exactly like a single-pass body.

### Decision

**Settled by derivation** — not discussed. Determined by the notes cache's input-keyed design (`internal/notescache/cache.go`, `Key`).

A dry run's chunked notes are cached and reused by the real run the same as single-pass notes, sparing the whole chunk wave on the real run.

**The cache keeps the chunk notes too** (raised by the background review, review-003 F9; user agreed 2026-10-17). The cache stored only the final body, so a real run reusing a dry run's chunked notes had no partial notes in hand: `r` there would re-run the whole chunk wave plus the merge, the cost Regenerate on Chunked Notes set out to avoid. A chunked body is now cached together with its per-chunk notes, so `r` on reused chunked notes re-runs only the merge, one call, as on a live run. The partial notes share the entry's lifecycle and need no cleanup of their own: the entry sits outside the repo, is pruned to the latest per project on every write (`Prune`, `internal/notescache/cache.go`), goes stale after the ~1h TTL, and once the release is tagged its key — which includes the version — cannot match again. In a live run the partial notes live only in memory and end with the run; only a failure's saved notes file persists, by design, for `e` or `--notes-file`.

**`--all`'s skip reason** — **Settled by derivation** — not discussed. Determined by the classifier decisions (the cause phrase comes from the classifier's result) and `--all`'s skip-and-continue remaining as its floor. With the size cap an oversized version chunks rather than skipping, so `--all` skips only at the floor, for whatever ended the ladder; each skipped version — in its warning and in the end-of-run summary — names that real cause rather than "diff too large".

**`--all`'s saved notes** — **Settled by derivation** — not discussed. Determined by When the Merge Fails and When a Chunk Fails (the finished chunk notes are saved on every such failure), the skip reason above, and regenerate's `--notes-file` answering one version (Notes-File Backdoor). Under `regenerate --all`, each version whose notes fail saves its own chunk-notes file, named in that version's skip warning and in the end-of-run summary; each is fed back with its own single-version `regenerate --notes-file`. *(Amended 2026-10-18 — per When a Chunk Fails' "No file when no chunk finished", only a version with at least one chunk's notes saves and names a file; a version whose single-pass call failed, or whose wave produced nothing, names none.)*

Sibling check: no overlap found — the cache is used as built.

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

**Malformed quoting** — **Settled by derivation** — not discussed. Determined by CLAUDE.md seam 6 (config is strict) with its precedent of rejecting bad values at load (`validateOnNotesFailure`, `validateHooks`: `invalid .mint.toml: …`), and fail loud, never hang (review-002 F5). An `ai_command` that cannot be parsed — an unterminated quote, at any level (top-level, `[release]`, `[commit]`) — is rejected when config loads, before any step runs, with `invalid .mint.toml: …` naming the key. It is never left to fail at call time, where it would read as an AI failure, reach the classifier (which runs the same command), take OTHER, and end in the fallback body or the editor.

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

**Unattended commit** — **Settled by derivation** — not discussed. Determined by CLAUDE.md's verbatim-diagnostics rule (both `StageFailure.Output` and `Warning.Output`) and this decision's own motivation — an expired login or a usage limit — where the unattended run is the one with nobody present to re-run it (review-002 F4). When an unattended `mint commit` (`-y` or non-TTY) fails loud because the AI failed, the failure also carries the AI's captured output verbatim beneath the no-message-source line. The other triggers into that guard — `--no-ai`, oversized, all-excluded — have no AI output and are unchanged.

---

## Overhead-Dominated Failures

### Context

Research held overhead-dominated failures — the AI tool's own system prompt, tools and attachments alone near the window — as not mint-fixable: chunking shrinks mint's prompt but every chunk re-pays the overhead, so mint could only surface and advise. The Lean Default AI Command decision removed this failure for the shipped command (~105k → ~561 tokens), narrowing the case to repos whose own `ai_command` carries the overhead.

### Journey

For those repos the reasoning still holds: no chunk is small enough, and the classify call — running the same command — fails the same way, so Classify Call Failure already routes the run down the OTHER path with the AI's verbatim message (claude's own overflow text names the system prompt and tool definitions). The one thing left is what mint's guidance says: pointing at the documented lean command keeps claude's flags in the documentation rather than in mint's code, consistent with mint holding no per-provider knowledge.

### Decision

**Settled by derivation** — not discussed. Determined by Lean Default AI Command, Classify Call Failure and Classifier Result Alongside the Verbatim Error.

An overhead-dominated failure on an explicit `ai_command` takes the OTHER path: no split, the AI's verbatim message under the ✗ line, then `on_notes_failure`. mint's guidance points at the documented lean command rather than naming claude flags itself.

Sibling check: no overlap found — this topic's own ground.

---

## Under-Cap Overflow: Halving

### Context

A prompt under the per-call size cap can still overflow the model — an explicit `ai_command` pointing at a smaller-window model (a local model, say), overhead an explicit command carries, or a cap default set above the real window. The classifier answers SIZE and the ladder fires, but chunks are sized "under the cap" and the failed prompt already was, so the chunker has nothing smaller to aim at. The same gap opens when a single chunk call comes back SIZE. Raised by the background review (review-002 F7).

### Options Considered

**Halve** — after a SIZE on a call under the cap, aim at half the size that just failed; a chunk that fails again is halved again, a bounded number of times, then the run falls to `on_notes_failure`.
- Pros: still produces notes without mint knowing the window.
- Cons: extra serial rounds — but only on a setup whose cap is wrong.

**Treat it as misconfiguration and stop**
- Pros: no extra rounds; makes a mis-set cap loud.
- Cons: fails a release that could have had notes, over a setting the operator may not know matters.

### Journey

Halving is the reactive bisection the Proactive Size Ceiling decision called wasteful as the *primary* way to size chunks; here it is only the fallback for a cap that is wrong for the model, so its cost lands where the misconfiguration is. Its serial rounds are a deliberate exception to the parallel-first shape, accepted because the path is rare when the cap fits the model. Pairing it with a post-run hint turns the quiet absorption into something the operator can fix.

### Decision

**On a SIZE answer for a diff-carrying call under the cap — the single-pass call or a chunk call — mint halves.** *(Amended 2026-10-17 — this read "a call under the cap", which reached the merge call too; halving a merge is the recursive-collapse reduce ruled out in When the Merge Fails, so a SIZE on the merge call takes that section's path instead.)* The chunks aim at half the size that just failed; a chunk that fails SIZE again is halved again, a few times at most (bound is specification detail), then the run falls to `on_notes_failure` as usual. After the notes, mint tells the operator the size cap is set higher than this model accepts and suggests lowering it. User agreed 2026-10-17. Confidence: high.

**When halving runs out** (raised by the background review, review-003 F13; user agreed 2026-10-17). An explicit `ai_command` whose own overhead leaves room for only a sliver of release sends every halving round into failure, and a lower-the-cap hint alone cannot help it. mint cannot tell this case from a model with a small window: in both, even small pieces come back SIZE while the small classify call fits (the classifier Journey's stop catches only overhead that alone overflows the window). So when the halving bound is spent, mint's guidance names both causes — the model accepts less than the cap (lower it), or the command carries a large overhead of its own (the documented lean command, per Overhead-Dominated Failures) — rather than the cap alone. The halving bound already keeps the wasted rounds few.

Sibling check: no overlap found — the halving rounds and the lean command are this topic's own decisions.

---

## Timeouts and the Ladder

### Context

A single notes call that runs past the per-attempt timeout returns `ErrTimeout`, which the transport never retries and which never reaches the classifier (no error text to classify), so the release falls straight to `on_notes_failure` — abort by default — without trying chunking. The repo behind this feature has hit it: agentic-workflows' `.mint.toml` carries `timeout = 300` with the note "the v0.6.0 re-platforming diff (75 squashes) timed out twice at 60s". Raised by the background review (review-002 F8).

### Options Considered

**Chunk on a timeout** — treat a timed-out single-pass call as a reason to start the ladder.
- Pros: rescues a slow, oversized call.
- Cons: a timeout is not evidence of size; when the service itself is slow, a parallel wave into it multiplies the wait.

**Keep timeouts out of the ladder and let the size cap absorb them** — choose the cap's default and the release timeout's default together.
- Pros: every call is already held under the cap and an over-cap release chunks before anything is sent, so a release like v0.6.0 chunks instead of running one long call; no wave fired at a slow service.
- Cons: a timeout on a release that sits under any sensible cap is not covered.

### Journey

The cap converts "too slow because too big" into "too big" before the call is made, which is the case the agentic-workflows timeout was. What would change the call: timeouts observed on releases that sit under any sensible cap — then sizing alone would not cover them. How the per-attempt timeout reads across a wave of N calls stays with the pending timeout-semantics subtopic.

### Decision

**A timeout does not start the degradation ladder; it falls to `on_notes_failure` as today.** The size cap's default and the release timeout's default are chosen together in specification, sized so a call at the cap finishes well inside the timeout. User agreed 2026-10-17. Confidence: medium-high.

---

## Timeouts Across a Wave

### Context

The transport's `timeout` is a per-attempt bound on one AI call (`internal/ai/transport.go`; per-verb via `TimeoutFor`). A chunked release makes many calls — the exclusion check, chunk calls run a few at a time, possibly classify calls, then the merge — and the existing setting said nothing about how it reads across them.

### Journey

The per-attempt meaning is the transport's by design, and Timeouts and the Ladder already sized the cap against it: a call at the cap finishes inside the timeout. An overall deadline across the wave would be a second, new timing rule whose right value grows with the release — exactly what the per-call cap exists to avoid having to predict.

### Decision

**Settled by derivation** — not discussed. Determined by the transport's per-attempt timeout and Timeouts and the Ladder (the cap is sized so a call at the cap finishes inside the timeout).

The per-attempt timeout applies to each call on its own — every chunk call, the merge and the classify call. A wave therefore takes roughly as long as its rounds of chunk calls plus the merge; mint sets no overall deadline across the calls.

Sibling check: no overlap found — ai-model-selection's per-verb `timeout` is used as specified, per attempt.

---

## Regenerate on Chunked Notes

### Context

On an attended release, the review gate offers `r` — regenerate with a one-time nudge ("lead with the auth work") — for AI-generated notes (`notes.KindNormalAI`). On a chunked release, `r` as wired would re-run the whole parallel map wave plus the merge: minutes of calls and a draw on the subscription usage window, where single-pass `r` costs one call. Research made the review gate the salience check for stitched notes, so `r` on a chunked body is exactly when an operator rejects a weak stitch. Raised by the background review (review-002 F9).

### Journey

A nudge asks to change emphasis or ordering, and the merge call is where emphasis and ordering are decided; the per-chunk partials do not change with it. Re-running only the merge costs one call, matching `r` on a normal release. The residual it cannot fix — a change a chunk's own partial missed — is rare.

### Decision

**On chunked notes, `r` re-runs only the merge call**, reusing the per-chunk notes already produced and adding the operator's one-time nudge. One call, as `r` costs on a single-pass release. User agreed 2026-10-17. Confidence: high.

**When the merge fails during `r`** — **Settled by derivation** — not discussed. Determined by the as-built rule that a failed `r` aborts whatever `on_notes_failure` says (`regenerateBody` in `internal/engine/release.go`: a Regenerator failure is surfaced and aborts; `ResolveFailure` in `internal/notes/resolve.go` governs only the normal AI path), this Decision modelling chunked `r` on single-pass `r`, and When the Merge Fails' save of the chunk notes (review-003 F5). A merge that fails during `r` aborts the run, as a failed `r` does on a single-pass release. As on any failed merge, mint first saves the per-chunk notes and names the file in the failure; `on_notes_failure` does not apply, so `fallback` never ships its body in place of the re-merge the operator asked for. The abort lands before any tag, so the saved notes can still go out through `--notes-file`.

---

## Map-Reduce, Not Refine

### Context

A chunked release needs a stitch shape. Research named two: **map-reduce** — each chunk produces partial notes in parallel, then one reduce call merges them over the whole release — and **refine** — sequential calls, each given the notes so far plus the next chunk. Naive concatenation alone was rejected as the plan (it breaks salience and dedup).

### Journey

The user set the direction in research (2026-06-26): "the more we can do in parallel, the better", and explicitly not three or four *serial* AI requests. Refine is serial by construction and order-biased (early chunks frame the notes); map-reduce is a parallel wave plus a single serial step. The one serial reduce is therefore the quality-and-risk chokepoint — what happens when it fails is the concat-floor subtopic's ground.

### Decision

**Settled by derivation** — not discussed. Determined by the user's research steer: parallel first, no chain of serial AI requests.

The chunked path is map-reduce: the chunk calls run in parallel, then exactly one merge call produces the final notes. Refine is out.

Sibling check: no overlap found — this topic's own ground.

---

## Stitching and Salience: the Change Map as Spine

### Context

Release notes are defined by cross-cutting salience: `DefaultPrompt` asks for one bullet per notable change (not per file or commit), closely related changes combined, ranked using the Change Map. A chunk sees only its slice, so a feature spanning several chunks risks fragmenting into duplicated bullets, and no chunk knows whether its slice is the headline or a footnote.

### Journey

The Change Map (`internal/notes/changemap.go`) is cheap and bounded by file count, not diff size — two git calls (`--name-status`, `--numstat`) rendered as structural novelty, churn by area and notable files. Handing the same global map to every chunk call and to the merge lets each chunk know the whole release's shape while reading only its slice, and lets the merge rank across all of it. Research called this the strongest reuse argument in the design. (It is not strictly size-independent — it lists renames and removals per path — which matters only for the edge where instructions plus Change Map alone approach the cap.)

### Decision

**Settled by derivation** — not discussed. Determined by `DefaultPrompt`'s salience rules (rank by the Change Map) and the Change Map's file-count-bounded cost.

Every chunk call and the merge call receive the same whole-release Change Map.

Sibling check: no overlap found — the Change Map is mint-release-tool's as-built artifact, used here as built.

**When the Change Map would crowd out the diff** (raised by the background review, review-003 F7; user agreed 2026-10-17). A release that deletes a vendored tree or moves thousands of files gets one `Removed:` or `Renamed:` line per path (`renderNovelty`, `internal/notes/changemap.go`), so instructions plus the Change Map alone can exceed the per-call cap, and no call mint could make would fit. When the map would crowd out the diff, mint condenses it: a long run of removed or renamed files collapses to one line per folder (e.g. "Removed: 4,312 files under vendor/"). The notes still learn the tree went — all they would say about it anyway — and the release still gets notes. A release whose map fits renders exactly as today. Rejected: stopping at the floor with a `diff_exclude` hint, which costs a release that could have had notes. The threshold for "crowds out" is specification detail.

Sibling check: mint-release-tool — its specification's Change Map section makes directory/area rollup the default and calls a flat list of every changed file "mush on big releases"; condensing long removed/renamed runs by folder applies that rollup to the novelty list, so nothing there is contradicted.

**No room left for diff** — **Settled by derivation** — not discussed. Determined by Proactive Size Ceiling (no call ever exceeds the cap), Under-Cap Overflow: Halving (an exhausted ladder falls to `on_notes_failure`), and condensing being the only shrink mint makes to its own fixed part. When a call's fixed part still leaves no room for diff after the map is condensed — an outsized `[release].prompt` or `context`, which is operator text mint does not trim, or a halving round aimed below the fixed part — mint sends no call and the run falls to `on_notes_failure`, with guidance naming which of the two it was.

---

## When the Merge Fails

### Context

The chunked path's one serial step is the merge call. Research's ladder had a rung between merge failure and the `on_notes_failure` floor: concatenate the per-chunk notes as they are. Each chunk writes its own sections, so the glued body repeats headers (`✨ Added … 🐛 Fixed … ✨ Added …`), carries near-duplicate bullets and no ranking — and under `-y` ships to the provider release unreviewed. Deep-dive F8's recursive-collapse reduce (re-merge partials in groups until they fit) was already ruled out by the no-serial-depth steer.

### Options Considered

**Glue the chunk notes together** (research's rung 3)
- Pros: a release always gets AI-written notes of some kind; zero extra calls.
- Cons: a visibly broken body, worst exactly when nobody is reviewing.

**No glue — save the chunk notes, then the floor**
- Pros: never ships an incoherent body; the wave's work is kept and becomes a one-step fix through the notes-file backdoor.
- Cons: the run does not produce AI notes on its own.

### Journey

The user questioned why the merge would fail at all once every chunk has finished — merged changelog notes pushing past the limit is near-impossible (research's magnitude argument: it takes hundreds of chunks). Agreed on size. But the merge can fail for reasons that are not size: it is the *last* call after a parallel wave, so it is the one most likely to meet a usage limit the wave itself drew down, an expired login, a timeout, or an error that survives the transport's retry. That strengthens keeping the chunk notes rather than throwing the wave away. With size off the table as a realistic cause, the glue rung earns nothing a simple fallback does not; the user concluded the same (2026-10-17).

### Decision

**No concatenation rung. A failed merge goes to `on_notes_failure` like any other failure** — after mint saves the per-chunk notes to a file outside the repo and names it in the failure guidance (e.g. "notes couldn't be merged; per-chunk notes saved to `<path>` — tidy them and release with `--notes-file <path>`"). Under `abort`, the ✗ line carries that pointer with the AI's verbatim output; under `fallback`, the fallback body ships and the saved file is still named. The floor stays the last rung, never the plan. User agreed 2026-10-17. Confidence: high.

---

## When a Chunk Fails

### Context

A chunked release sends its chunk calls in parallel, so one can fail while the rest succeed: an error that survives the transport's retry and classifies OTHER, a timeout (which never reaches the classifier — Timeouts and the Ladder), or a chunk whose halving rounds run out (Under-Cap Overflow: Halving). The run then holds notes for most of the release and nothing for one slice of it — say five of six chunks.

### Options Considered

**All or nothing** — the generation counts as failed and goes to the floor; the finished chunks' notes are saved.
- Pros: never ships notes that read as complete while missing part of the release; the saved notes turn the attended floor's `e` into tidying and filling in rather than writing from scratch.
- Cons: a release that had most of its AI notes gets none on its own.

**Merge what came back, flagged** — merge the surviving partial notes and flag the missing slice: at the review gate on a terminal run, as a warning on an unattended one.
- Pros: every such release still gets AI notes.
- Cons: the notes look whole but aren't, and under `-y` they ship unreviewed, warning or not.

### Journey

The missing slice can be the release's headline feature. Notes that read as complete while quietly leaving out the biggest change are the very failure this feature exists to end — big releases getting coherent notes — and the one the parked headline-first roadmap item targets. It is also the line When the Merge Fails drew: no body that only looks right ships unattended. The user agreed (2026-10-17).

### Decision

**One failed chunk fails the generation.** When any chunk call ends in failure — after its retry, its classify call and, on SIZE, its halving rounds — no merge runs. mint saves the notes of the chunks that finished to a file outside the repo, names it in the failure, and the run goes to `on_notes_failure` like any other failure. At a terminal under `abort`, the floor's `e` opens the editor on the saved notes; unattended runs stop with guidance; under `fallback` the fallback body ships and the saved file is still named. User agreed 2026-10-17. Confidence: high.

**No file when no chunk finished** — **Settled by derivation** — not discussed. Determined by this Decision saving "the notes of the chunks that finished", When the Merge Fails keeping the file so the wave's work is kept, the floor's `e` already having an empty branch (Remediation at the Floor), and the missing-slice marker recording a gap in notes that exist (review-004 F9). mint saves and names a chunk-notes file only when at least one chunk's notes exist. When a wave fails before any chunk finishes — a login expiring or a usage limit landing at the start — the failure names no file, the floor's `e` opens empty, and there is no `--notes-file` pointer; a file holding only the marker would otherwise pass `--notes-file`'s empty check and ship as the release's notes.

**Ctrl-C mid-wave** — **Settled by derivation** — not discussed. Determined by CLAUDE.md seam 5 (a `context.Canceled` propagates unchanged and is never routed to a fallback) and mint keeping no resume state (a batch regenerate already re-runs from the top). A Ctrl-C during a chunked run cancels every call in flight and aborts cleanly: no chunk notes are saved and `on_notes_failure` does not apply. An interrupted chunked run behaves like an interrupted single-pass one — it stops, and a re-run starts over.

**The missing slice is marked in the saved file** (raised by the background review, review-003 F11; user agreed 2026-10-17). A saved file from a failed chunk lacks that chunk's part of the release, so "tidy them and release with `--notes-file`" taken at face value would ship notes without it. The saved file opens with a clearly marked line naming the files that have no notes, and the failure guidance says the same. The file is what gets picked up later — by the operator through `e`, or by an agent session releasing with `--notes-file`, possibly after the terminal output is gone — so the gap has to be recorded in the file itself. Accepted cost: the marker line ships if the file is released unread; an attended release shows it at the review gate.

Sibling check: no overlap found — the saved chunk notes and `--notes-file` are this topic's own terms.

---

## When the Whole Wave Fails

### Context

A usage limit landing mid-wave, or a login expiring, fails every chunk call still running at once. As the rules stood, each failed chunk gets the transport's single retry and its own classify call (which carries the retry too), so a wave of N chunks could send up to 4N calls at a service already refusing them before the ✗ line appears — the amplification the classifier was chosen to avoid (its deciding factor: never turn a non-size failure into a wave of N calls). Raised by the background review (review-003 F12).

### Options Considered

**Classify every failed chunk** — each failure is classified and handled on its own.
- Pros: every chunk's cause is known.
- Cons: up to 4N calls into a refusing service; once one chunk has failed for good the generation is already failed (When a Chunk Fails), so the rest of the answers change nothing.

**Stop on the first answer that settles the run, let running calls finish** — the first failure that is not SIZE ends the wave's new work.
- Pros: a wave-wide failure costs a couple of extra calls, not one set per chunk; the chunks already running still produce notes for the saved file.
- Cons: the run stops only when those running calls return.

**Stop and cancel running calls**
- Pros: the quickest stop once the cause is known.
- Cons: throws away notes that were close to done.

### Journey

When a Chunk Fails made the generation all-or-nothing, which means the first chunk failure that is not a size problem decides the run; classifying further failures can only confirm it. Letting calls already in flight finish costs no extra requests — they were already sent — and what they return is what makes the attended floor's `e` cheap. The user agreed (2026-10-17).

### Decision

**The first chunk failure that is not SIZE ends the wave.** After it, mint makes no further classify calls and starts no new chunk calls; calls already running finish, and the notes they produce are saved with the rest. A timeout settles it the same way, with no classify call (Timeouts and the Ladder). A SIZE answer does not end the wave — that chunk halves as decided. The ✗ line renders that first failure's cause phrase and its captured output verbatim. User agreed 2026-10-17. Confidence: high.

Sibling check: no overlap found — the wave, its chunk calls and the classify call are this topic's own terms.

**A bound on parallel calls** — **Settled by derivation** — not discussed. Determined by the parallel-first steer (Map-Reduce, Not Refine) weighed against what unbounded fan-out does: one AI process per chunk started at once puts the whole wave on the machine and on the provider's rate and usage limits simultaneously. The chunk calls run with a concurrency limit — a few at a time — which keeps most of the parallel speed. The bound itself is specification detail.

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

#### 2026-10-17 — revised
*Trigger: user reversal, in the engagement on review-003 F8 — the user asked why mint would judge a big file hand-written or generated at all: that call cannot be made reliably and depends on the project, and the exclusion check has already settled whether each file belongs. A file still in the diff — not in `diff_exclude`, not flagged by the check, or kept by the operator — is wanted, so it is read.*

**No stand-in. A file whose diff is too big to fit in one chunk call alongside that call's instructions, Change Map and reminder is split into pieces** — slices of its diff, each sized to fit a chunk call, sent in parallel with the other chunk calls and stitched by the merge like any other partial notes; never a serial loop (Map-Reduce, Not Refine). "Too big" is measured the same way as everything else, in bytes against a chunk call's room. Pieces are cut at hunk or line boundaries; a single line longer than a whole call (a minified bundle, say) would need cutting mid-line, which is specification detail. Gone with the stand-in: the stand-in line, its after-notes `diff_exclude` hint (the exclusion check's suggestions replace it), and the AI's hand-written/generated verdict from the entry below. The parked deciding factor — big files are almost always generated noise — now lives with the exclusion check, which asks about them before the notes; under `-y` nobody answers it, so a flagged big file is read in pieces and the printed suggestion names the glob. Change from the entries below: everything that stays in the diff is read. User agreed 2026-10-17. Confidence: high.

#### 2026-10-17 — revised (earlier)
**Re-decided 2026-10-17 — the stand-in is for generated files only** (user, in the engagement on review-003 F6). Per Exclusion Suggestions Before the Notes, the AI judges each file too big for one chunk call. One it judges hand-written is read in pieces: its diff is split into slices that each fit a call, sent in parallel with the other chunk calls and stitched by the merge like any other partial notes — never a serial loop, the refine shape ruled out in Map-Reduce, Not Refine. One it judges generated keeps the stand-in, and at a terminal is raised at the exclusion pause first. The earlier deciding factor still holds for most such files; the judgment catches the exception — notably a hand-written file under a cap set low or halved down.

#### 2026-10-16
**A file whose diff is too big to fit in one chunk call alongside that call's instructions, Change Map and reminder is represented by a one-line stand-in, not its diff.** *(Amended 2026-10-17 — this read "whose diff alone exceeds the per-call size cap"; per Proactive Size Ceiling the cap bounds the whole composed prompt, and per Stitching and Salience every call carries the full Change Map, so a chunk call has less room for diff than the cap.)* The stand-in names the file and how much changed and says the diff was omitted for size; the stand-in line is how the file reaches the notes — the chunk call carrying it can mention the file where it matters, and the merge sees what that chunk wrote. *(Corrected 2026-10-17 — this said the Change Map still carries the file. The as-built map names a file individually only when it is new, removed, renamed, a new top-level entry or the single largest by line count (`renderChangeMap`, `internal/notes/changemap.go`), so a long-lined transcript is usually not named; the map is left unchanged (review-003 F6).)* After the notes, mint tells the operator which file(s) it stood in for and suggests adding them to `diff_exclude`. User chose this 2026-10-16. Deciding factor: such a file is almost always a generated artifact, so reading it costs AI calls for nothing. Confidence: high.

**Packing by area** — **Settled by derivation** — not discussed. Determined by the salience goal (a feature seen whole by one map call rather than fragmented across chunks), the Change Map's existing grouping by area, and the parallel-first steer, which rules out a serial ask-the-AI-to-group round-trip. Chunks are packed by area — files from the same part of the codebase stay together — filled up to the per-call size cap.

---

## Exclusion Suggestions Before the Notes

### Context

Files that do not belong in release notes reach them because nothing flags them. The user has shipped releases whose notes documented design files or test changes they did not mean to leave in (2026-10-17). On big releases the same kind of file bloats the diff: the 1.9 MB design-session transcript in the agentic-workflows release, mint's own 867 KB `.workflows/`/`.tick/` artifacts. The single-file stand-in covered only a file too big for any one call, and treated every such file alike. In the engagement on review-003 F6 the user asked whether Claude could read a big file over several calls, and whether it could tell which files should be in `diff_exclude`.

### Options Considered

**Read every oversized file in pieces**
- Pros: nothing dropped.
- Cons: spends calls on generated noise — several calls that each say "updated the transcript".

**The AI judges; mint acts on it only for oversized files** — stand-in or pieces by the AI's verdict, other suggestions printed as a hint after the notes.
- Pros: no file ever leaves the notes on the AI's say alone.
- Cons: the operator learns of an unwanted file only after the notes are written, and has to edit the config themselves.

**The AI judges and excludes files on its own for the run**
- Pros: noise gone with no interaction.
- Cons: a wrong "generated" verdict silently drops a real change from the notes, unreviewed under `-y`.

**The AI judges; at a terminal mint asks about each suggestion and records the answer in config**
- Pros: the slip the user describes is caught before the notes are written, the release carries on in the same run, and each answer is given once.
- Cons: one more AI call before the notes on every release; a pause when something is flagged.

### Journey

The user agreed that the AI should not exclude files on its own, then asked for its judgment to reach the operator — human or agent — before the notes: "are these files meant to be included?" Two conditions came with it: a path answered "yes, include it" must never be asked about again, and a "no" should be added to the config and committed by mint, with the release carrying on rather than the operator backing out to do it by hand. Answers live in `.mint.toml` rather than state private to one machine, so a teammate, CI or an agent session elsewhere is not asked again. The check runs on every release, not only big ones: the user's slips happened on normal-sized releases, which a big-release-only check would have missed; the cost is one small serial call before the notes. Reading a big file in pieces is parallel slices, not a loop. The user agreed the whole shape (2026-10-17).

The user then separated the two verbs' needs: release notes are a public list of what a release holds, while a commit message is for the developers and agents reading that piece of work, and should cover its tests and specs. Today one shared `diff_exclude` serves both — commit's AI diff applies it too (`internal/commit/preflight.go`: diff_exclude "only shapes the AI's L1 diff") — so an exclusion answer written there would hide test or spec folders from commit messages as well. A commit-side check was weighed and dropped: commit messages should describe the tests and specs in the work, and a check there would be asking a different question, closer to "did you mean to stage this?". The user put the per-verb split in this feature (2026-10-17).

### Decision

**On `mint release`, before the notes are written, mint asks the AI which changed paths look like they do not belong in release notes, and at a terminal asks the operator about each one.** User agreed 2026-10-17. Confidence: high.

- **The check** — one AI call through the existing transport carrying the release's file list with names and sizes, no diff. Paths already covered by `diff_exclude` or by the keep list are not raised. It runs on every release whose notes come from the AI *(amended 2026-10-18 — this read "on every release"; see Only when the AI writes the notes below)*.
- **At a terminal** — one pause per flagged path or glob, offering exclude or keep, through the presenter's gate seam:
  - **Exclude** — mint adds the glob to the `[release]` exclusion list in `.mint.toml` (never the shared one, so commit messages still see those files), changing only that entry and leaving the rest of the file as written, commits it, and carries on; this release's notes are written without those files, and the commit goes out with the release.
  - **Keep** — mint records the glob on a new `[release]` keep list in `.mint.toml` (key name is specification detail; a new key takes the strict-config path), committed the same way, and never raises it again.
- **Per-verb exclusion lists** — `diff_exclude` gains a `[release]` and a `[commit]` level beside the shared top-level list. Each verb's AI diff excludes the shared list plus its own: the lists add, unlike `ai_command`/`timeout`, whose per-verb value overrides, because the shared list holds what neither verb should read (lockfiles, build output) and a per-verb list should not have to repeat it. The exclusion check's answers go to `[release]` only. The new keys take the strict-config path (schema, friendly type error, config-metadata row, README tables).
- **`mint commit` gets no check** (user, 2026-10-17).

**mint's own config never reaches the notes** (raised by the background review, review-004 F4; user agreed 2026-10-18). An exclude or keep answer commits `.mint.toml` inside the release it was made for, so the notes diff would show a `.mint.toml` change, a commit-subject body (the floor's `c`, or `fallback` with an empty body) would list mint's own exclusion commit, and a later regenerate of that version would see both. `.mint.toml` joins `CHANGELOG.md` as a built-in always-exclude from every notes diff — forward and regenerate alike, since exclusion is path-based — and the commit-subject list leaves out mint's exclusion commit. Deciding reason: a change to mint's config is about how releases are made, not what a release holds; showing it would put a tooling line into every release that answered the pause.

Sibling check: mint-release-tool — its specification's built-in always-exclude tier holds `CHANGELOG.md` alone (non-configurable, "pure mint output, never meaningful source") and makes exclusion path-based so regenerate reproduces the forward view; `.mint.toml` joins that tier on the same grounds, extending it once this lands, and nothing there is contradicted.

**A file list too big for one call** (raised by the background review, review-004 F2; user agreed 2026-10-18). On a release touching thousands of files, the check's own list of names and sizes can exceed the per-call cap, which no call may do (Proactive Size Ceiling). When it would, the list is condensed the way the Change Map is (Stitching and Salience): a long run of files under one folder becomes one line, e.g. `vendor/: 4,312 files, 38 MB`. The AI suggests folder-level globs in the main, so a folder-grouped list keeps what it needs; skipping the check would lose it on exactly the releases where flagging a folder matters most, and splitting the list across calls would split the judgment. The threshold is specification detail.

**Regenerate runs no check** (raised by the background review, review-004 F5; user agreed 2026-10-18). `mint release regenerate` — single version or `--all`, fresh source included — makes no exclusion check: no pause and no config writes. It applies the `[release]` exclusion and keep lists as they stand, so every answer given on forward releases carries into regenerated notes. Deciding reason: a regenerate that writes only the provider release needs no clean tree and commits nothing (`RegenerateGateSet.CommitsAndPushes`, `internal/engine/regenerate.go`), and a check would make it edit and commit `.mint.toml` mid-run — and could pause `--all` at every version; new releases are where new paths appear and get answered. Accepted cost: noise in an old version that was never asked about is not flagged.

**Only when the AI writes the notes** — **Settled by derivation** — not discussed. Determined by the check's purpose (keeping paths out of AI-written notes; it runs before the notes and the size check runs on what remains), `--no-ai`'s contract of never calling the AI (`internal/notes/noai.go`), Notes-File Backdoor (a supplied body skips the size cap, chunking and AI), the first-release guard winning before any diff is assembled (`SelectBody`, `internal/notes/select.go`), and covered paths never being raised (review-004 F1). The exclusion check runs only when the release's notes would come from the AI. It does not run under `--no-ai`, with `--notes-file`, on a first release, or when no path is left to raise (an empty post-exclusion diff). The Journey's "every release, not only big ones" argued against a size threshold, not for runs that write no AI notes.

**When the check call fails** — **Settled by derivation** — not discussed. Determined by the check's advisory role (it only proposes exclusions), the notes call that follows running the same command through the normal failure path (the classifier, then `on_notes_failure`), and CLAUDE.md seam 5 (a cancel propagates unchanged). If the exclusion check call fails — a generation failure, a timeout, a missing binary — the release carries on with no suggestions; any real AI problem surfaces at the notes call. A Ctrl-C during the check still cancels the run.

**A failed check is said, not silent** (raised by the background review, review-004 F3; user agreed 2026-10-18). When the check fails on its own — a timeout, a one-off error, an answer mint cannot read — and the notes call then succeeds, the operator could not otherwise tell "nothing looked out of place" from "the check never ran", which is how a design file slips into the notes unnoticed. mint prints one warning line (wording to the effect of "couldn't check for files that don't belong in the notes") with the AI's captured output verbatim beneath it, per CLAUDE.md's verbatim-diagnostics rule for `Warning.Output`, then carries on.
- **Unattended (`-y`, non-TTY, an agent run)** — no pause and nothing written; the suggestions print with the notes, for whoever reads the output to act on.
- **Dry run** — the suggestions show; nothing is asked or written.
- **Abort after answering** — the config commit stays: the answer is about the repo, not this release, and unwinding it would bring the same question back next time.
- **Big files** — the check asks about them like any other path and makes no separate verdict on them; whatever stays in the diff is read, a file too big for one chunk call in pieces (Chunk Boundaries and the Single-File Floor, revised). The size check runs after this one, on the diff that remains.

Sibling check: mint-release-tool — its specification defines `diff_exclude` as a shared configurable array of globs for tracked generated artifacts, and its pre-push failure row has mint auto-unwind its own mutations back to the exact clean starting state. This decision writes to `diff_exclude` as defined, adds a keep list beside it, and makes the exclusion-answer commit the one deliberate exception to that unwind row, superseding it for that commit once it lands.

Sibling check: ai-model-selection — its specification keeps `diff_exclude` shared-only "until their own real need appears"; the per-verb split is that need, so the text anticipates it and nothing is owed.

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
- **Regenerate** — takes `--notes-file` for a single version, a source beside fresh, tag and release (re-decided 2026-10-17; raised by the background review, review-003 F10; user agreed). The earlier call here — no notes-file in regenerate, since chunking covers its oversized versions and the tag/release sources already rewrite from an existing body — did not see the saved chunk notes: under `on_notes_failure = fallback` the version ships with the fallback body before the saved notes can be used, and a failed regenerate has no route to use them at all. `mint release regenerate <version> --notes-file <path>` puts that finished work onto a version already out. It behaves as on release — read once at startup, byte-identical, the gate without `r`, a bad file refused — and one file answers one version, so giving it with `--all`, or with another `--source`, is a usage error. Where the version has already shipped (the fallback body, or a failed regenerate), the failure guidance names this command rather than `mint release --notes-file`.

  Sibling check: mint-release-tool — its specification's Regenerate / Backfill Notes defines the sources as a fresh re-diff, the tag annotation and the existing provider release; a supplied file is a new source beside them, nothing there is contradicted, and this decision extends that list once it lands.

**Precedence over mint's stand-in bodies** — **Settled by derivation** — not discussed. Determined by this section's decision that the supplied body passes into the release byte-identical, and by the purpose of the first-release and degenerate (empty post-exclusion diff) guards in `internal/notes/select.go` — they produce an honest body when there is nothing for an AI to summarise, which a supplied body does not need (review-002 F3). A supplied notes file wins over both guards: whenever the file is given, its body is the release's body — never `Initial release.` or the maintenance stub. (Today those guards run ahead of `--no-ai` and the AI, which is why the `[release].fallback` workaround loses to them.)

---

## Progress Narration for a Chunked Run

### Context

Today the notes step is one stage: a spinner in pretty mode (a terse start line in plain), then ✓ or ✗. A chunked release can take minutes — the exclusion check, rounds of chunk calls a few at a time, maybe halving, then the merge — and a spinner that sits that long looks the same whether mint is working or hung. The Oversize Knob's "silent" default meant no consent prompt, leaving how a chunked run is narrated to this subtopic.

### Options Considered

**One spinner, as today** — no new narration.
- Pros: nothing to build.
- Cons: minutes of an unchanging line; nothing tells a working run from a stuck one.

**The one notes line, updated as the run moves** — what is happening and how far along it is.
- Pros: shows the run is alive and roughly how long is left, without adding lines in pretty mode.
- Cons: new progress events through the presenter.

**Per-part detail** — which folders each part covers, each part's own status.
- Pros: the most visibility.
- Cons: noise unless something fails, and on a failure the ✗ line already names it.

### Journey

Proposed with a mock of the pretty line and agreed by the user (2026-10-18):

```text
⠋ Checking for files that don't belong in the notes…
⠋ Writing notes: 3.1 MB release, in 6 parts · 2 of 6 done
⠋ Writing notes: part 4 was too big for the model, retrying it as 2 smaller parts
⠋ Writing notes: merging 6 parts
✓ Notes written (6 parts, merged)
```

### Decision

**A chunked run keeps the single notes line and updates it as the run moves**: the exclusion check, the split (release size and part count), parts done out of the total, any halving retry, then the merge; the ✓ line says the notes were written in parts and merged. In plain mode — CI and agents, no live line — each of those becomes its own line as it happens, one per finished part included, so a long run never goes quiet. No per-part detail beyond the count. Follow-ups (the lower-the-cap hint, the exclusion suggestions under `-y`) print after the notes, as decided in their own sections. Exact wording is specification detail. User agreed 2026-10-18. Confidence: high.

Sibling check: mint-release-tool (cli-presentation) — its specification makes spinners a pretty-only concern spanning `StageStarted` to `StageSucceeded`/`StageFailed`, with plain rendering the same events as terse lines, and the engine never touching spinners or TTY state. The progress updates are new presenter events in that model, rendered by each presenter; nothing there is contradicted.

---

## The Setup Guide's AI-Transport Prose

### Context

`mint setup` prints a version-matched, self-contained guide for an AI agent configuring mint (`internal/setupguide`). Its overview's "The AI transport" paragraph states the shipped default as `claude -p --model sonnet` and describes today's failure behaviour — a bad-output retry, a fatal timeout, then `on_notes_failure` or the editor. This feature changes both. Its config-reference table renders from the config-metadata source of truth, so new and renamed keys reach it with no prose change.

### Journey

The guide's own contract is that it is version-matched so it never goes stale against the installed binary, and CLAUDE.md keeps docs true to the as-built code. Teaching an agent to operate mint's command flags is a different scope — the guide is flag-free by design — and is parked on the roadmap as an agent guide for running releases.

### Decision

**Settled by derivation** — not discussed. Determined by the setup guide's version-matched, self-contained contract (`internal/setupguide` package doc) and CLAUDE.md's rule that docs stay true to as-built.

The guide's AI-transport prose is updated to what this feature changes — the lean shipped default, the per-call size cap, chunking and the floor. New and renamed config keys reach its config reference through the source of truth as built.

**The guide's `diff_exclude` advice** — **Settled by derivation** — not discussed. Determined by this Decision's grounds (version-matched, kept true where this feature makes it untrue) and the per-verb split's rule that the shared list holds what neither verb should read (Exclusion Suggestions Before the Notes) (review-004 F10). The guide's etiquette section ("diff_exclude scope") and its procedure step 4 send `.workflows/`, `.claude/`, agent directories, `docs/` and lockfiles to a plain `diff_exclude`, which after the split would hide them from commit messages too. That advice is updated to name the levels: noise only release notes should skip goes in `[release]`; the shared list keeps only what neither verb should read, such as lockfiles and build output.

Sibling check: interactive-mint-init-setup — the work that built `mint setup`; its guide prose is edited here only where this feature makes it untrue, and the guide's flag-free scope is left as built.

---

## Summary

### Key Insights

1. **mint cannot predict fit, so it reacts and caps.** The classify-on-failure trigger responds to the AI's actual outcome; the per-call byte cap is an honest size policy that also gives the chunker its target — neither pretends to know the model's window.
2. **The AI tool's own overhead was the dominant cost, and it is trimmable.** The lean claude command cuts it ~190× on the subscription login; the live agentic-workflows overflow would have fitted in one pass. The cheapest lever turned out to sit outside chunking entirely.
3. **mint owns the prompt.** Dropping CLAUDE.md and Claude Code's system prompt is a feature, not a cost: a raw ask carrying only mint's instructions is what lets mint control its output.

### Open Threads

- **Parked on the roadmap under Next (2026-10-16): headline-first release notes.** The default prompt's category-first format and its "NO TL;DR, NO summary paragraph, NO preamble" rule (`internal/notes/prompt.go`) leave no place to declare what a release is about, so a release's main feature can land as one bullet among sideline changes. Separate from big-release handling; the chunked merge pass inherits whatever format exists when it ships.
- **Parked on the roadmap under Next (2026-10-16): agent-authored release notes.** Claude can already run `mint release -y` from a session, with mint's own AI call writing the notes; what is missing is a way for the outer agent to write notes to mint's standard — mint handing it the instructions and Change Map, or taking a one-line statement of what the release is about (the session that built the feature knows the headline that mint's diff-only view misses). The receiving half — taking a finished body — is this feature's notes-file backdoor.
- **Parked on the roadmap under Next (2026-10-17): an agent guide for running releases.** The `mint setup` guide teaches an agent to configure mint and deliberately leaves out command flags, so nothing tells an agent session about the surfaces this feature adds — `--notes-file` on release and regenerate, the saved chunk notes and their missing-slice marker, the floor's choices. The user wants an agent to know them without the user having to remember them.

### Current State

- Decided: the detection trigger in full — reactive classify-on-failure as primary, a per-call byte cap as the proactive guard, and the classifier's edges (codes, failures, empty output, hardening, retry).
- Decided: the shipped default AI command becomes the lean claude form.
- Decided: `max_diff_lines` becomes a per-call byte cap; over it, releases chunk and commit drops to the editor.
- Decided: at the floor an attended run offers commit-subject notes, the editor or abort; unattended runs stop with cause-fitted guidance.
- Decided: the chunked path — parallel chunk calls plus one merge, the Change Map in every call, area-packed chunks, halving after an under-cap overflow; a failed merge, or any one failed chunk, fails the generation — the chunk notes that exist are saved and the run falls to `on_notes_failure`.
- Decided: everything left in the diff after exclusions is read; a single file too big for one call is split into pieces read in parallel — no stand-ins.
- Decided: before every release's notes, an AI check flags paths that look out of place; at a terminal mint asks about each, adding a "no" to the `[release]` exclusion list and a "yes" to a keep list, committed, so no path is asked about twice. `diff_exclude` gains `[release]` and `[commit]` levels that add to the shared list; `mint commit` gets no check.
- Decided: a `--notes-file` backdoor on release and on single-version regenerate; the named file is excluded from every git step mint runs.
- Decided: the classifier's codes are SIZE / OTHER; its result renders alongside the AI's verbatim error; off-list answers map to OTHER.
- Decided: chunking is always on — no `oversize` setting; the per-call size cap is the only size setting.
- Decided: a chunked run narrates on its one notes line — the check, the split, parts done, halving, the merge — and as one line per event in plain mode.
- Decided: one failed chunk, or a failed merge, fails the generation; the first non-size failure ends the wave; chunk calls run a few at a time, each under its own per-attempt timeout; Ctrl-C cancels cleanly.
