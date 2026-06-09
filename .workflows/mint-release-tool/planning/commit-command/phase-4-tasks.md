---
phase: 4
phase_name: Interactive Gate Actions — Edit and Regenerate
total: 5
---

## commit-command-4-1 | approved

### Task commit-command-4-1: Add the `e` edit action with loop-back to the gate

**Problem**: The Phase 1 `Continue?` gate (1-5) offers only `y`/`n` (Enter ⇒ accept). The spec's gate is `y`/`n`/`e`/`r`, and `e` (edit) lets the user refine the AI-generated message before it sticks. Crucially, `e` is **not** save-as-accept — it opens the editor pre-filled with the **current** message, and on a non-empty save it **returns to the `Continue?` gate** (the cli-presentation loop-back contract) with the edited message shown and **used verbatim — no AI reprocessing**. Only the Phase 3 *fallback* editor is save-as-accept; the gate's `e` is a refinement loop. Without `e`, the user must abort and re-run (or hand-edit after committing) to fix a minted message they otherwise like.

**Solution**: Extend the commit gate (1-5) to offer `e` as a choice. When the user presses `e`, resolve the editor via the consumed 3-1 `ResolveEditor` (no parallel resolver), open it from mint pre-filled with the **current** message body, and on a **non-empty** save replace the current message with the edited text **verbatim** (no L2/AI call, no reprocessing) and **loop back** — re-render the `Continue?` gate with the edited message, with `y`/`n`/`e`/`r` all still available. From the re-rendered gate the user may accept (`y`), abort (`n`), edit again (`e`), or regenerate (`r`). This task owns the non-empty-save loop-back; empty-save handling is 4-2 and not-launchable graceful-degrade is 4-3.

**Solution note**: The editor **resolution** is consumed from 3-1 (`ResolveEditor`) — do NOT build a parallel resolver. The gate **rendering**, the line-read input model, and the choice dispatch are consumed from the Presenter (the same seam 1-5 integrates) — `e` is added to the declared choice set the gate offers. This is the **loop-back** contract, explicitly distinct from the Phase 3 fallback's **save-as-accept**: an `e` non-empty save does NOT stage, does NOT commit, and does NOT push — it only updates the in-memory message and re-renders the gate. The edited message is used **verbatim**: do NOT pass it back through `ComposePrompt`/L2 — there is no AI reprocessing of an edit. Empty-save discard-and-preserve is 4-2; the not-launchable-editor graceful-degrade is 4-3; the `r` action is 4-4/4-5. `e` is interactive-only (moot under `-y`/non-TTY, which auto-accept or fail loud per 1-5) — do not make it reachable on those paths.

**Outcome**: At an interactive `Continue?` gate showing an AI-generated commit message, pressing `e` opens the editor (resolved via 3-1) pre-filled with that **current** message. On a **non-empty save**, mint takes the edited text **verbatim** as the new candidate message — **no AI call, no reprocessing** — and **re-renders the `Continue?` gate** with the edited message shown and `y`/`n`/`e`/`r` still offered. A **multi-line** edited body (subject + blank line + wrapped body) is preserved intact through the loop-back. The `e` save does **not** stage, commit, or push (it is loop-back, not save-as-accept); only a subsequent `y` accepts. Editing again (`e` from the re-rendered gate) opens the editor pre-filled with the *now-edited* message.

**Do**:
- Extend the commit gate (the 1-5 `Prompt(gate)` integration in `internal/commit` `Run`) so the declared gate choice set includes `e` alongside `y`/`n`. Branch on the returned choice; this task implements the `e` branch (non-empty-save case).
- On `e`: resolve the editor via the consumed **3-1 `ResolveEditor`** (no parallel resolver). [The not-launchable signal is handled by 4-3 — this task assumes a launchable editor.]
- Open the editor **from mint** (via the consumed `CommandRunner`) pre-filled with the **current** message body — the message currently shown at the gate (initially the L3-generated body; after a prior `e`/`r`, the latest candidate). Pre-fill with the real message, NOT an empty/template buffer (this is edit, not the fallback path).
- On a **non-empty save**: take the saved buffer as the new candidate message **verbatim** — do NOT call `ComposePrompt`/L2/the AI; there is no reprocessing of an edit. Preserve the full multi-line body exactly as saved.
- **Loop back**: re-render the `Continue?` gate with the edited message shown, offering `y`/`n`/`e`/`r` again. The `e` save is NOT an accept — do NOT stage, commit, or push here. Only a subsequent `y` proceeds to the (consumed) commit path.
- Implement the gate as a loop so `e` (and `r`) can re-enter it any number of times; `y` exits to accept, `n` exits to abort.
- Do NOT implement empty-save discard (4-2), not-launchable graceful-degrade (4-3), or the `r` action (4-4/4-5). Do NOT make `e` reachable under `-y`/non-TTY (interactive-only).

**Acceptance Criteria**:
- [ ] The gate offers `e` alongside `y`/`n` on an interactive AI-path run.
- [ ] `e` opens the editor pre-filled with the **current** message (not an empty/template buffer).
- [ ] The editor is resolved via the consumed **3-1 `ResolveEditor`** — no parallel resolver is introduced.
- [ ] A **non-empty save** loops back to the `Continue?` gate — it is **not** save-as-accept (no staging, no commit, no push).
- [ ] The edited message is used **verbatim** — no L2/AI call and no reprocessing of the edit.
- [ ] A **multi-line** edited body is preserved intact through the loop-back.
- [ ] The re-rendered gate still offers `y`/`n`/`e`/`r`; editing again pre-fills with the now-edited message.
- [ ] `e` is interactive-only — it is not reachable under `-y` or non-TTY.

**Tests**:
- `"the gate offers e alongside y/n on an interactive AI-path run"`
- `"pressing e opens the editor pre-filled with the current message"`
- `"e resolves the editor via 3-1 ResolveEditor (no parallel resolver)"`
- `"a non-empty save loops back to the gate, not save-as-accept (no staging/commit/push)"`
- `"the edited message is used verbatim with no AI reprocessing (no L2 call)"`
- `"a multi-line edited body is preserved through the loop-back"`
- `"the re-rendered gate still offers y/n/e/r"`
- `"editing again pre-fills the editor with the now-edited message"`

**Edge Cases**:
- Non-empty save loops back to the gate, not save-as-accept.
- Edited message used verbatim with no AI reprocessing.
- Multi-line edited body preserved.
- Gate re-renders with the edited message and y/n/e/r still available.
- Reuses 3-1 editor resolution (no parallel resolver).

**Context**:
> Interactive Review Gate → Choice mapping: "`e` / edit → open the editor (resolved via the shared chain under Fallback Semantics → Editor resolution) pre-filled with the current message; on save, **return to the `Continue?` gate** with the edited message shown, used verbatim (no AI reprocessing). This follows the cli-presentation seam's loop-back contract — `e` re-renders the gate, it is *not* save-as-accept (only the fallback editor is save-as-accept). From the re-rendered gate the user may accept (`y`), abort (`n`), edit again (`e`), or regenerate (`r`)." $EDITOR Fallback — Path Semantics → Editor resolution: "applies to *every* editor mint opens — both this fallback path and the gate's `e` action" — so `e` consumes the same 3-1 resolver. The empty-save discard, not-launchable graceful-degrade, and `r` action are the sibling Phase 4 tasks; `e` is interactive-only (moot under `-y`/non-TTY per 1-5).

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Interactive Review Gate → Choice mapping (`e` / edit)", "$EDITOR Fallback — Path Semantics → Editor resolution".

## commit-command-4-2 | approved

### Task commit-command-4-2: `e` empty-save discards the edit; gate re-renders with the prior message preserved

**Problem**: `e` is a **refinement step, never a message source** — so it must never be able to produce an empty commit. When the user opens the editor via `e` but saves an **empty** (or whitespace-only) buffer — or quits without writing anything — mint must **discard the edit** and **re-render the `Continue?` gate with the PRIOR message preserved** (the message that was shown before `e`), not commit an empty message and not lose the existing candidate. This is the opposite of the Phase 3 *fallback* editor, where an empty save is a true no-op abort — under `e` a message already exists, so an empty save simply means "I didn't change anything." Without this, an accidental empty save under `e` could blank out a good minted message.

**Solution**: In the `e` branch of the gate (4-1), when the editor save is **empty** — buffer empty, whitespace-only, or quit/abort with no content — **discard** the edited buffer (do NOT adopt it as the new message) and **loop back** to the `Continue?` gate with the **prior** message (the candidate shown before `e`) preserved unchanged, `y`/`n`/`e`/`r` still offered. "Empty" is determined per the editor contract established in 3-2 (no non-whitespace/non-comment content ⇒ empty). Because `e` only ever *replaces* an existing message on a non-empty save and otherwise preserves it, `e` can never yield an empty commit.

**Solution note**: This task is the empty-save counterpart to 4-1's non-empty-save loop-back; it shares the same `e` branch and the same gate loop. The "empty buffer" determination is **consumed** from the editor-contract semantics 3-2 already established (whitespace-only treated as empty) — reuse it, do NOT invent a second emptiness rule. The key distinction from Phase 3: on the **fallback path** an empty save = abort/no-op (no message exists yet); under the **`e` gate action** an empty save = **discard + preserve the prior message** (a message already exists). `e` is never a message source, so it can never produce an empty commit — there is no path where an empty `e` save commits anything. Repeated `e` then empty-save must still preserve the message that was current before that `e` (i.e. the last non-empty candidate), including back to the original generated message. `e` is interactive-only (moot under `-y`/non-TTY).

**Outcome**: At the gate, pressing `e` and then saving an **empty** buffer (or whitespace-only, or quitting with no content) **discards** the edit and **re-renders the `Continue?` gate with the prior message preserved** exactly as it was, `y`/`n`/`e`/`r` still available. A **whitespace-only** save is treated as empty (per the 3-2 editor contract) and discarded the same way. **Repeated `e` then empty-save** still preserves the message that was current before that `e` — e.g. `e` (non-empty edit → message becomes X), then `e` again (empty save) leaves X; an `e` empty-save on the very first edit leaves the original generated message. `e` **never** produces an empty commit — there is no empty-`e`-save path that commits.

**Do**:
- In the `e` branch of the gate loop (4-1), detect an **empty** save: buffer empty, **whitespace-only**, or quit/abort with no written content. Reuse the **3-2 editor-contract emptiness rule** (no non-whitespace/non-comment content ⇒ empty) — do NOT introduce a second emptiness definition.
- On an empty save: **discard** the edited buffer (do NOT adopt it) and **re-render** the `Continue?` gate with the **prior** message — the candidate that was current before this `e` — preserved unchanged. Offer `y`/`n`/`e`/`r` again.
- Ensure the preserved message is the **last non-empty candidate**: after a prior non-empty `e` (message X), an empty `e` save leaves X; an empty `e` save on the first edit leaves the original generated message. (The gate loop carries the current candidate forward; an empty `e` save simply does not mutate it.)
- Make explicit that `e` is **never a message source**: there is no code path where an empty `e` save proceeds to commit — the only way out of the gate to a commit is `y` on a non-empty candidate. So `e` can never produce an empty commit.
- Do NOT treat an empty `e` save as an abort/no-op (that is the Phase 3 fallback semantics, not the gate's `e`). The run is not abandoned — the gate simply re-renders.
- Do NOT make `e` reachable under `-y`/non-TTY (interactive-only). Do NOT implement not-launchable graceful-degrade (4-3) or the `r` action (4-4/4-5) here.

**Acceptance Criteria**:
- [ ] An **empty** `e` save discards the edit and re-renders the gate with the **prior** message preserved unchanged.
- [ ] A **whitespace-only** `e` save is treated as empty (per the 3-2 editor contract) and discarded the same way.
- [ ] A **quit/abort with no content** under `e` is treated as empty and preserves the prior message.
- [ ] **Repeated `e` then empty-save** preserves the message current before that `e` (the last non-empty candidate, back to the original generated message).
- [ ] `e` is **never a message source** — no empty-`e`-save path commits; `e` can never produce an empty commit.
- [ ] An empty `e` save is NOT treated as an abort/no-op (distinct from the Phase 3 fallback) — the gate re-renders, the run continues.
- [ ] The emptiness rule is **consumed** from 3-2, not re-defined.

**Tests**:
- `"an empty e save discards the edit and re-renders the gate with the prior message"`
- `"a whitespace-only e save is treated as empty and preserves the prior message"`
- `"quitting the e editor with no content preserves the prior message"`
- `"repeated e then empty-save preserves the last non-empty candidate"`
- `"an empty e save on the first edit preserves the original generated message"`
- `"an empty e save never commits (e is never a message source)"`
- `"an empty e save is not an abort — the gate re-renders and the run continues"`

**Edge Cases**:
- Empty save discards the edit and preserves the prior message.
- Whitespace-only save treated as empty per the editor contract.
- Repeated `e` then empty-save still preserves the original message.
- `e` is never a message source, so it can never produce an empty commit.

**Context**:
> Interactive Review Gate → Choice mapping (`e`): "An empty save under `e` discards the edit and re-renders the gate with the **prior message preserved** — `e` is a refinement step, never a message source, so it can never produce an empty commit." $EDITOR Fallback — Path Semantics → Editor resolution: "Consistent with '`e` is a refinement step that can never produce an empty commit.'" Contrast with the fallback path where "A non-empty save = accept; quit/empty = abort" — under the gate's `e` a message already exists, so an empty save is discard-and-preserve, not abort. The whitespace-only/empty determination reuses the 3-2 editor contract; `e` is interactive-only (moot under `-y`/non-TTY).

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Interactive Review Gate → Choice mapping (`e` / edit, empty save)".

## commit-command-4-3 | approved

### Task commit-command-4-3: `e` graceful-degrade when no editor is launchable

**Problem**: When the user presses `e` at the gate but **no editor in the chain resolves to a launchable program** (the 3-1 not-launchable signal), mint must **not** fail loud — because, unlike the Phase 3 *fallback* path, **a message already exists** (the AI-generated candidate at the gate). The spec's rule: behaviour at a not-launchable editor depends on whether a message candidate exists. On the **fallback path (no message yet)** → fail loud (3-5). On the **`e` gate action (a message exists)** → **graceful degrade**: warn that the editor could not be launched and **re-render the `Continue?` gate with the UNEDITED message preserved** (treat `e` as a no-op). This keeps the run usable — the user can still accept the existing message — instead of killing it for a missing editor. Without this branch, a not-launchable editor under `e` would either crash or wrongly fail loud.

**Solution**: In the `e` branch of the gate (4-1), branch on the **not-launchable** signal returned by the consumed 3-1 `ResolveEditor`. When it fires, do NOT attempt to open an editor and do NOT fail loud: emit a **warning** (via the consumed Presenter) that the editor could not be launched, then **re-render the `Continue?` gate with the unedited message preserved verbatim**, `y`/`n`/`e`/`r` still offered — treating `e` as a **no-op**. This is explicitly **distinct from the 3-5 fallback fail-loud**, which fires only when there is no message to fall back to; here a message exists, so the gate stays usable.

**Solution note**: The **not-launchable** signal is **consumed** from 3-1 — do NOT re-derive launchability. The decision of *fail-loud vs graceful-degrade* on a not-launchable editor is the consumer's (per 3-1's design); this task implements the **graceful-degrade** consumer for the `e` gate action, the mirror of 3-5's fail-loud consumer for the fallback path. The discriminator is "does a message candidate already exist?": at the `e` gate it always does (the gate is showing one), so graceful-degrade. Do NOT reuse 3-5's fail-loud here — that path has no message. After the warn, the gate must remain fully usable (`y`/`n`/`e`/`r`): the user can accept the existing message, abort, try `e` again, or `r`. `e` is interactive-only (moot under `-y`/non-TTY).

**Outcome**: At the gate, pressing `e` when **no editor in the chain is launchable** (3-1 not-launchable signal) does **not** open an editor and does **not** fail loud. mint **warns** that the editor could not be launched and **re-renders the `Continue?` gate with the unedited message preserved verbatim**, still offering `y`/`n`/`e`/`r` — so the user can accept (`y`) the existing message, abort (`n`), retry `e`, or `r`. This is **distinct from the 3-5 fallback fail-loud**: the fallback fails because no message exists; the `e` action degrades gracefully because a message already exists. `e` is treated as a **no-op** in this case (the message is unchanged).

**Do**:
- In the `e` branch of the gate loop (4-1), call the consumed **3-1 `ResolveEditor`** and branch on its **not-launchable** signal.
- On not-launchable: do NOT open an editor, do NOT fail loud. Emit a **warning** via the consumed Presenter — the editor could not be launched (e.g. *"could not launch an editor — keeping the current message"*) — surfaced consistently with mint's warn rendering.
- **Re-render the `Continue?` gate** with the **unedited** message preserved **verbatim** (treat `e` as a **no-op** — the candidate is unchanged), still offering `y`/`n`/`e`/`r`. The gate must remain fully usable: `y` accepts the existing message, `n` aborts, `e` retries, `r` regenerates.
- Make explicit the distinction from **3-5**: 3-5 fails loud on a not-launchable editor on the **fallback** path (no message to fall back to); this task degrades gracefully on the **`e` gate** path (a message already exists). Do NOT route the `e` not-launchable case into the 3-5 fail-loud.
- Do NOT make `e` reachable under `-y`/non-TTY (interactive-only). Do NOT implement the `r` action (4-4/4-5) here.
- Tests use the fake runner / 3-1 not-launchable signal + recording presenter: assert `e` under not-launchable warns (not fails loud), re-renders the gate with the unedited message, no editor is launched, and the gate remains usable (a subsequent `y` commits the unchanged message).

**Acceptance Criteria**:
- [ ] A **not-launchable** signal from 3-1 under `e` triggers a **warn + re-render**, **not** fail-loud.
- [ ] The **unedited** message is preserved **verbatim** through the re-render (`e` treated as a no-op).
- [ ] No editor is launched when the signal is not-launchable.
- [ ] The behaviour is **distinct from the 3-5 fallback fail-loud** — `e` degrades gracefully because a message already exists.
- [ ] The gate remains **usable** after the warn — `y`/`n`/`e`/`r` are all still offered (a subsequent `y` commits the unchanged message).
- [ ] The not-launchable signal is **consumed** from 3-1, not re-derived.
- [ ] `e` is interactive-only — this path is not reachable under `-y`/non-TTY.

**Tests**:
- `"a not-launchable editor under e warns and re-renders the gate (not fail-loud)"`
- `"the unedited message is preserved verbatim when e cannot launch an editor"`
- `"no editor is launched when the not-launchable signal fires under e"`
- `"the e not-launchable graceful-degrade is distinct from the 3-5 fallback fail-loud"`
- `"the gate remains usable (y/n/e/r) after the not-launchable warn"`
- `"a subsequent y after the warn commits the unchanged message"`

**Edge Cases**:
- Not-launchable signal from 3-1 triggers warn + re-render, not fail-loud.
- Unedited message preserved verbatim.
- Distinct from the 3-5 fallback fail-loud because a message already exists.
- Gate remains usable (y/n/e/r) after the warn.

**Context**:
> $EDITOR Fallback — Path Semantics → Editor resolution: "When **no editor in the chain resolves to a launchable program**, behaviour depends on whether a message candidate already exists: Fallback path (no message yet) → fail loud, same as the no-TTY/`-y` case above — there is no message to fall back to. `e` gate action (a message already exists) → graceful degrade: warn that the editor could not be launched and re-render the `Continue?` gate with the **unedited** message preserved (treat `e` as a no-op). Consistent with '`e` is a refinement step that can never produce an empty commit.'" The not-launchable signal is consumed from 3-1; 3-5 owns the fallback-path fail-loud (no message); this task owns the `e`-action graceful-degrade (message exists). `e` is interactive-only (moot under `-y`/non-TTY).

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "$EDITOR Fallback — Path Semantics → Editor resolution (no launchable program → `e` gate action graceful-degrade)".

## commit-command-4-4 | approved

### Task commit-command-4-4: Add the `r` regenerate-with-context action (line-read + one-time injection)

**Problem**: The Phase 1 gate has no way to re-roll a message the user dislikes. The spec's `r` (regenerate-with-context) action — the "context injection" affordance from the user's original commit shell function — lets the user re-run the AI with a **one-time** free-text context line. After `r`, mint prompts for a single context line via the Presenter's line-read (Enter submits), **injects it one-time** into the regeneration prompt, re-runs the engine, and returns the regenerated message to the gate. The injected line is **not persisted** (not to config, not to subsequent re-rolls), and an **empty line** regenerates with **no injected context** (a plain re-roll). Without `r`, the only options for a poor message are abort-and-retry or hand-edit via `e` — neither lets the user *guide* a fresh AI generation.

**Solution**: Extend the commit gate (4-1's loop) to offer `r`. When the user presses `r`, prompt for a **single free-text context line** via the consumed Presenter line-read (the same input model as the gate; Enter submits). Inject that line **one-time** into the regeneration prompt — assembled via the consumed L3 `ComposePrompt`/`Generate` path (1-2/1-3) augmented with the one-time context — and re-run the engine (consumed L2, including its normal one retry). On success, set the regenerated body as the new candidate and **loop back** to the `Continue?` gate with it shown, `y`/`n`/`e`/`r` still offered. An **empty** context line regenerates with **no** injected context (plain re-roll). The injected line is **not persisted** to `[commit].context` or carried into any later re-roll. Regeneration *failure* routing is 4-5.

**Solution note**: The line-read is **consumed** from the Presenter (the same seam the gate uses) — do NOT build a parallel input reader. Prompt assembly and the L1→compose→L2 generate path are **consumed** from 1-2/1-3; this task adds only the **one-time context** augmentation and the gate wiring. The one-time context is distinct from `[commit].context` (the persisted config knob, 1-1): the `r` line is injected for **this regeneration only** and **never persisted** — not written to config, not merged into the next `r`'s prompt. The engine's normal **one retry** is consumed (1-3/L2) — do NOT re-implement it. On regeneration **success**, loop back to the gate (like `e`'s loop-back, but the body came from the AI, not a hand-edit). Regeneration **failure** after the one retry routes to the `$EDITOR` fallback — that is **4-5**, do NOT build it here. `r` is **interactive-only** (moot under `-y`/non-TTY, which auto-accept or fail loud per 1-5) — do not make it reachable on those paths.

**Outcome**: At an interactive gate, pressing `r` prompts for a **single free-text context line** via the Presenter's line-read; **Enter submits**. A **non-empty** line is injected **one-time** into the regeneration prompt, the engine re-runs (consumed L2 + its one retry), and the **regenerated** message returns to the `Continue?` gate (shown, `y`/`n`/`e`/`r` still offered). An **empty** line regenerates with **no** injected context — a plain re-roll. The injected context is **not persisted**: it is never written to `[commit].context`, and a **subsequent** `r` starts fresh (the prior line is not carried forward). `r` is interactive-only — it is never reached under `-y`/non-TTY.

**Do**:
- Extend the commit gate (the 4-1 loop) so the declared choice set includes `r` alongside `y`/`n`/`e`. Branch on the returned choice; this task implements the `r` branch (success path).
- On `r`: prompt for a **single free-text context line** using the consumed **Presenter line-read** (the same input model as the gate; **Enter submits**). Do NOT build a parallel input reader.
- Inject the line as **one-time** context into the regeneration prompt: re-run the consumed L3 generate path (1-3 `Generate` over the same diff, with `ComposePrompt` from 1-2) augmented so the user's line is added as one-time context **in addition to** any `[commit].context`. The engine's normal **one retry** is consumed — do NOT re-implement it.
- **Empty line** ⇒ regenerate with **no** injected context (a plain re-roll) — the prompt is the default/configured prompt with no extra one-time line.
- **Not persisted**: the injected line must NOT be written to `[commit].context` or any config, and must NOT be carried into a subsequent `r` — each `r` starts from a fresh, empty one-time context. Assert a second `r` does not include the first `r`'s line.
- On regeneration **success**: set the regenerated body as the new candidate and **loop back** to the `Continue?` gate with it shown, offering `y`/`n`/`e`/`r`. The `r` is NOT an accept — do NOT stage/commit/push; only a subsequent `y` proceeds.
- Do NOT implement the regeneration-**failure** → `$EDITOR` fallback routing (that is **4-5**). Do NOT make `r` reachable under `-y`/non-TTY (interactive-only).
- Tests use the recording presenter (scripted line-read input) + fake runner (scripted regeneration): assert `r` reads a line via the Presenter, a non-empty line is injected one-time into the regeneration prompt, an empty line injects nothing, the regenerated message returns to the gate, the line is not persisted to config, and a second `r` does not carry the first line.

**Acceptance Criteria**:
- [ ] The gate offers `r` alongside `y`/`n`/`e` on an interactive AI-path run.
- [ ] `r` prompts for a **single free-text context line** via the consumed **Presenter line-read**; **Enter submits**.
- [ ] A **non-empty** line is injected **one-time** into the regeneration prompt (in addition to any `[commit].context`).
- [ ] An **empty** line regenerates with **no** injected context (a plain re-roll).
- [ ] The injected context is **not persisted** — not written to `[commit].context`/config, and not carried into a subsequent `r`.
- [ ] Regeneration runs the engine's **one retry** (consumed) — not re-implemented.
- [ ] The **regenerated** message returns to the `Continue?` gate (shown, `y`/`n`/`e`/`r` still offered); `r` is not an accept (no staging/commit/push).
- [ ] `r` is **interactive-only** — not reachable under `-y`/non-TTY.
- [ ] Regeneration-failure routing (4-5) is **not** implemented here.

**Tests**:
- `"the gate offers r alongside y/n/e on an interactive AI-path run"`
- `"r prompts for a single context line via the Presenter line-read (Enter submits)"`
- `"a non-empty line is injected one-time into the regeneration prompt"`
- `"an empty line regenerates with no injected context (plain re-roll)"`
- `"the injected context is not persisted to [commit].context/config"`
- `"a subsequent r does not carry the prior r's line (one-time, fresh each time)"`
- `"regeneration consumes the engine one retry (not re-implemented)"`
- `"the regenerated message returns to the gate (not an accept; no staging/commit)"`

**Edge Cases**:
- Non-empty line injected one-time into the regeneration prompt.
- Injected context not persisted to config or subsequent re-rolls.
- Empty line regenerates with no injected context (plain re-roll).
- Enter submits via the Presenter's line-read.
- Regenerated message returns to the gate.
- Moot under `-y`/non-TTY (interactive-only action).

**Context**:
> Interactive Review Gate → Choice mapping (`r`): "`r` / regenerate with context → re-run the AI with a one-time context line. This *is* the 'context injection' affordance from the user's original commit shell function. After `r`, mint prompts for a single free-text context line (rendered via the Presenter's line-read — the same input model as the gate); Enter submits. The line is injected as one-time context into the regeneration prompt and is **not persisted**. An empty line regenerates with no injected context (a plain re-roll). Regeneration runs the engine's normal one retry; failure routes to the `$EDITOR` fallback (per Fallback Semantics)." CLI Surface → Resolved (dropped): "No `--context` one-time-context flag … Interactive `r` (regenerate-with-context) at the gate plus the `[commit].context` config cover the need." The one-time `r` line is distinct from the persisted `[commit].context` (1-1). Regeneration-failure routing is 4-5; `r` is interactive-only (moot under `-y`/non-TTY).

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Interactive Review Gate → Choice mapping (`r` / regenerate with context)".

## commit-command-4-5 | approved

### Task commit-command-4-5: Route `r` regeneration-failure to the `$EDITOR` fallback

**Problem**: When the user presses `r` and the **regeneration fails after its one retry** (transport error / unusable output), mint must treat it as **any other AI failure** → the `$EDITOR` fallback — the same path 3-3 (AI-generation failure) already established. One consistent rule: **any failed AI generation lands at the editor**. There is **no special "re-show the prior message" path** — the prior (pre-`r`) message is not re-shown; the run drops to the editor exactly as a first-generation failure would. Without this, an `r` failure would have no defined outcome (or would invent a bespoke handler), breaking the single-rule invariant.

**Solution**: In the `r` branch (4-4), when the consumed L3 generate path surfaces a **generation-failure** outcome after the engine's one retry, route to the **same editor fallback entry point as 3-3** — reuse 3-3's failure-routing entry point (no parallel failure handler), which opens the editor with the established save-as-accept semantics (3-2): editor resolved via 3-1, empty/template buffer (no synthetic stub, **no re-show of the prior message**), non-empty save ⇒ stage-then-commit, empty/abort ⇒ true no-op. The save-as-accept semantics are **unchanged**. This is the single consistent rule applied to the `r` failure.

**Solution note**: The failure-routing **entry point** is **consumed** from 3-3 (which was authored to expose a shared entry point Phase 4's `r`-failure reuses) — do NOT build a parallel failure handler. The save-as-accept routine (3-2), editor resolution (3-1), and the engine's one-retry (1-3/L2) are all consumed unchanged. There is **no special re-show-prior-message path**: the spec is explicit — "No special 're-show the prior message' path." The editor opens empty/template exactly as on the 3-3 path; mint does NOT pre-fill the pre-`r` message into the fallback editor. The `-y`/non-TTY fail-loud (3-5) is **moot** here because `r` is an interactive-only gate action — an unattended run never reaches `r` (per 1-5). Do NOT add an `r`-specific re-show or a bespoke failure branch.

**Outcome**: When an `r` regeneration **fails after its one retry**, mint routes to the **3-3 editor fallback** (the same entry point, no parallel handler): the editor opens (resolved via 3-1) with an **empty/template** buffer — **no synthetic stub and no re-show of the pre-`r` message** — and the **unchanged** save-as-accept semantics apply (non-empty save ⇒ stage-then-commit per mode; empty/abort ⇒ true no-op). This is the single consistent rule: any failed AI generation lands at the editor. Because `r` is interactive-only, the `-y`/non-TTY fail-loud is moot — the run is already on a TTY.

**Do**:
- In the `r` branch (4-4), branch on the consumed L3 generate **generation-failure** outcome (transport failed / unusable output **after** the engine's one retry — consumed from 1-3/L2; do NOT re-implement the retry).
- On failure, route to the **same failure-routing entry point as 3-3** (reuse it; **no parallel failure handler**). This opens the editor fallback with the **3-2 save-as-accept** semantics unchanged: editor resolved via 3-1, **empty/template** buffer, non-empty save ⇒ apply the mode's deferred staging then `git_safe` commit, empty/abort ⇒ true no-op.
- **No special re-show-prior-message path**: do NOT pre-fill the pre-`r` (or any prior) message into the fallback editor; it opens empty/template exactly as the 3-3 path does. (The spec explicitly forbids a re-show path.)
- Keep the **save-as-accept semantics unchanged** — this task adds only the *routing* of the `r`-failure into the shared 3-3 entry point; it does not alter save/abort behaviour.
- Note that the `-y`/non-TTY fail-loud (3-5) is **moot** for `r`: `r` is interactive-only and unreachable under `-y`/non-TTY (per 1-5), so an `r`-failure is always on a TTY. Do NOT add a separate unattended branch for `r`.
- Do NOT build a bespoke `r`-failure handler, a re-show path, or any deviation from the 3-3 fallback.
- Tests use the fake runner scripting an `r` regeneration failure after retry: assert the run routes to the 3-3 editor fallback (same entry point), the editor opens empty/template (no re-show of the prior message, no stub), save-as-accept behaves unchanged (non-empty save stages+commits, empty/abort no-ops), and no parallel failure handler is introduced.

**Acceptance Criteria**:
- [ ] An `r` regeneration **failure after the engine's one retry** routes to the **3-3 editor fallback**.
- [ ] It **reuses the 3-3 entry point** — **no parallel failure handler** is introduced.
- [ ] There is **no special re-show-prior-message path** — the fallback editor opens **empty/template** (no stub, no pre-`r` message pre-filled).
- [ ] The **save-as-accept semantics are unchanged** (non-empty save ⇒ stage-then-commit per mode; empty/abort ⇒ true no-op).
- [ ] The engine's **one retry** is consumed (1-3/L2), not re-implemented.
- [ ] The `-y`/non-TTY fail-loud (3-5) is **moot** — `r` is interactive-only; no separate unattended branch is added.

**Tests**:
- `"an r regeneration failure after the one retry routes to the 3-3 editor fallback"`
- `"the r-failure reuses the 3-3 entry point (no parallel failure handler)"`
- `"the fallback editor opens empty/template — no re-show of the prior message, no stub"`
- `"save-as-accept semantics are unchanged on the r-failure path (non-empty save stages then commits)"`
- `"an empty/aborted editor on the r-failure path is a true no-op"`
- `"the engine one-retry is consumed on the r-failure path (not re-implemented)"`

**Edge Cases**:
- Failure after the engine's one retry routes to the 3-3 editor fallback.
- Reuses the 3-3 entry point (no parallel failure handler).
- No special re-show-prior-message path.
- Fallback save-as-accept semantics unchanged.
- Moot under `-y`/non-TTY (interactive-only action).

**Context**:
> $EDITOR Fallback — Path Semantics → Regeneration failure routes here too: "If the user presses `r` (regenerate-with-context) at the gate and the regeneration fails after its one retry, mint treats it as any other AI failure → the `$EDITOR` fallback. One consistent rule: any failed AI generation lands at the editor. No special 're-show the prior message' path. (Under `-y`/non-TTY this is moot — `r` is an interactive-only gate action.)" Interactive Review Gate → Choice mapping (`r`): "Regeneration runs the engine's normal one retry; failure routes to the `$EDITOR` fallback (per Fallback Semantics)." The failure-routing entry point, save-as-accept routine, and editor resolution are consumed from 3-3/3-2/3-1; the engine's one retry is consumed from 1-3/L2. The `-y`/non-TTY fail-loud (3-5) is moot for `r` (interactive-only).

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "$EDITOR Fallback — Path Semantics → Regeneration failure routes here too", "Interactive Review Gate → Choice mapping (`r`)".
