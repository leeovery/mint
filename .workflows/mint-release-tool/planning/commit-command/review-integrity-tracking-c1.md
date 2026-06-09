---
status: in-progress
created: 2026-06-09
cycle: 1
phase: Plan Integrity Review
topic: Commit Command
---

# Review Tracking: Commit Command - Integrity

## Summary

The plan is strong and implementation-ready overall: 5 phases, 25 tasks, each carrying the full canonical task template (Problem / Solution / Outcome / Do / Acceptance Criteria / Tests / Edge Cases / Context / Spec Reference). Tasks are single TDD cycles, vertically sliced, self-contained, with concrete pass/fail acceptance criteria. Phase ordering (Foundation → Core → Edge → Refinement) is sound, and the consumed-external-dependency seams are explicitly named per task. The tick store (topic `tick-82909f`) matches the task files exactly (5 phases + 25 tasks, all priority 2, no dependency edges — consistent with the stated natural-order design; no convergence point needs an explicit edge because the cross-task reuse is all backward-pointing within natural creation order).

The findings below are the only gaps worth raising. One is Important (a foundational editor-roundtrip mechanism the whole `$EDITOR` family consumes is never concretely stated); the rest are Minor.

## Findings

### 1. Foundational editor open/save-roundtrip mechanism is never concretely specified

**Severity**: Important
**Plan Reference**: Phase 3 / commit-command-3-2 (`--no-ai` drops to the editor with save-as-accept) — the foundation task that 3-3, 3-4, 4-1, 4-2, 4-3, 4-5 all consume.
**Category**: Task Self-Containment / Scope and Granularity
**Change Type**: add-to-task

**Details**:
3-2 is the task that *establishes* how mint opens an editor itself (rather than delegating to `git commit`) and treats the save as the accept event — a routine every other editor-touching task explicitly reuses ("the editor save-as-accept routine established in 3-2," "open it from mint pre-filled with the current message," etc.). But 3-2's **Do** only says "open it directly from mint (via the consumed `CommandRunner`)" and "treat the save as the accept event." It never states the concrete file-roundtrip mechanism the implementer must build: write the buffer (empty/template for the fallback path, the current message for `e`) to a temp file, launch the resolved editor argv against that file, wait for exit, then read the saved file back as the message. Because 3-1 returns an *editor command* (`git var GIT_EDITOR` output, which can be a shell word-list like `code --wait`), the implementer must also know to invoke it as `<editor-argv> <tempfile>` (not pipe via stdin), and that `e` (4-1) requires the *same* routine to support a **pre-filled** initial buffer — a capability the fallback path (empty buffer) does not exercise. Leaving this to the implementer risks a stdin-based or `git commit -e`-delegated design that 4-1's pre-fill requirement then cannot reuse, breaking the "no parallel editor logic" intent. The mechanism is spec-supported (the spec says mint opens the editor itself because staging is deferred to save) — this is pulling the existing decision into a concrete, reusable instruction, not adding new scope.

**Current**:
```
- Route to the editor fallback: resolve the editor via 3-1 (`ResolveEditor`), then **open it directly from mint** (via the consumed `CommandRunner`) against the real, unstaged state with an **empty/template** buffer. Do **not** delegate to `git commit` to open the editor — mint opens it itself because staging is deferred to the save-as-accept event. Insert **no synthetic stub** message.
```

**Proposed**:
```
- Route to the editor fallback: resolve the editor via 3-1 (`ResolveEditor`), then **open it directly from mint** (via the consumed `CommandRunner`) against the real, unstaged state with an **empty/template** buffer. Do **not** delegate to `git commit` to open the editor — mint opens it itself because staging is deferred to the save-as-accept event. Insert **no synthetic stub** message.
- Build the editor open as a **reusable file-roundtrip routine** (the routine 3-3/3-4 reuse and 4-1's `e` pre-fills): (1) write the **initial buffer** to a temp file — empty/template here, but the routine must accept a caller-supplied initial message so 4-1's `e` can pre-fill the current message into the same routine; (2) invoke the **resolved editor argv from 3-1** against the temp-file path (`<editor-argv> <tempfile>` — 3-1 may return a multi-word command such as `code --wait`, so split/launch it as an argv with the path appended, do **not** feed the message via stdin); (3) wait for the editor to exit; (4) read the saved temp file back as the resulting buffer. The routine returns the saved buffer (and whether the editor exited normally) for the save-as-accept / empty-save decision below; it does not itself stage or commit.
```

**Resolution**: Pending
**Notes**:

---

### 2. 3-2's "empty buffer" emptiness rule references comment-stripping but the buffer carries no template/comments

**Severity**: Minor
**Plan Reference**: Phase 3 / commit-command-3-2 — the emptiness determination that 4-2 explicitly consumes ("Reuse the 3-2 editor-contract emptiness rule").
**Category**: Acceptance Criteria Quality / Task Self-Containment
**Change Type**: update-task

**Details**:
3-2 defines "empty" as "a buffer with no non-comment, non-whitespace content is empty" (mirroring `git commit`'s comment-stripping). But the same task is emphatic the buffer opens **empty/template with no synthetic stub** — i.e. there are no comment lines (`#`-prefixed) inserted by mint, so the "non-comment" qualifier describes content that, on the `--no-ai` path, mint never produces. 4-2 then consumes this rule for the `e` path, where mint *also* pre-fills only the real message (no comment lines). The "non-comment" wording is therefore inert at best and, at worst, invites the implementer to start inserting git-style comment scaffolding (a synthetic template) that 3-2 elsewhere forbids. The load-bearing rule is simply **whitespace-only (or no content) ⇒ empty**. Tightening the wording removes the contradiction without changing behaviour.

**Current**:
```
- Determine "empty" the way git does for the editor flow: a buffer with no non-comment, non-whitespace content is empty ⇒ abort. (Mirror plain `git commit`'s empty-message-aborts behaviour.)
```

**Proposed**:
```
- Determine "empty" the way git does for the editor flow: a buffer whose content is **only whitespace** (or has no content at all) is empty ⇒ abort. (Mirror plain `git commit`'s empty-message-aborts behaviour. Because the buffer opens with no synthetic stub or comment scaffolding, there are no `#`-comment lines to strip here — emptiness is purely whitespace-only/no-content; downstream tasks (4-2) reuse this same whitespace-only rule.)
```

**Resolution**: Pending
**Notes**:

---

### 3. 3-5's "non-TTY stdin" forbidden-combo trigger is stated only as consumed, never pinned to which stream gates the editor

**Severity**: Minor
**Plan Reference**: Phase 3 / commit-command-3-5 (Fail loud when the fallback has no message source).
**Category**: Acceptance Criteria Quality
**Category note**: Edge-case criteria specificity.
**Change Type**: update-task

**Details**:
3-5 fires fail-loud when "stdin is non-TTY" and says to reuse "the consumed forbidden-combo condition the Presenter already exposes (per 1-5)." For the **gate**, the relevant non-TTY stream is the one the prompt reads from (stdin). For the **editor fallback**, an interactive editor needs a controlling **terminal** (typically stdin *and* stdout attached to a tty) — a run with non-TTY stdin but a tty stdout is exactly the unattended case the rule targets, and keying purely on "the Presenter's stdin check" is the right call, but the task never states that the editor reuses the *same stdin-based* determination rather than introducing a stdout/`/dev/tty` check. Since 3-5's whole point is "reuse, don't re-derive isatty," making explicit that the editor's interactivity is gated on the **same Presenter stdin determination** (not a new stdout/controlling-terminal probe) removes an implementer judgement call. Pure clarification of the existing decision.

**Current**:
```
  - **Non-TTY stdin** (no `-y`) → fail loud — reuse the **consumed** forbidden-combo condition the Presenter already exposes (per 1-5); do NOT re-implement isatty detection.
```

**Proposed**:
```
  - **Non-TTY stdin** (no `-y`) → fail loud — reuse the **consumed** forbidden-combo condition the Presenter already exposes (per 1-5); do NOT re-implement isatty detection. Gate the editor's interactivity on the **same Presenter stdin determination** the gate uses — do NOT introduce a separate stdout/controlling-terminal (`/dev/tty`) probe for the editor path. A run the Presenter classifies as non-interactive (non-TTY stdin, no `-y`) is the no-message-source case here regardless of stdout.
```

**Resolution**: Pending
**Notes**:

---

### 4. 5-4 leaves the command exit status on push failure ambiguous ("may exit non-zero")

**Severity**: Minor
**Plan Reference**: Phase 5 / commit-command-5-4 (Warn-don't-unwind on push failure).
**Category**: Acceptance Criteria Quality
**Change Type**: update-task

**Details**:
5-4's **Do** says "The overall command **may** exit with a non-zero/failure status to signal the push failed, but the commit stays in place." "May" is a non-pass/fail directive in an otherwise crisply-specified task — an implementer cannot write a deterministic test against "may." The surrounding intent is clear (the commit succeeded and is forward-only; only the push failed, and it is repeatable), but the exit code is a real, testable observable (`mint commit -Apy` in CI must signal whether the push landed). The plan should pick one: since the commit *succeeded* and the push is a best-effort final step whose failure is *reported not repaired*, a non-zero exit (push failed) is the defensible choice for scriptability — but whichever is chosen, it must be definite, not "may." Spec-supported either way; this only removes the ambiguity.

**Current**:
```
- The push remains **repeatable**: mint reports failure but performs no cleanup, so re-running the push by hand is the documented fix. The overall command may exit with a non-zero/failure status to signal the push failed, but the **commit stays in place**.
```

**Proposed**:
```
- The push remains **repeatable**: mint reports failure but performs no cleanup, so re-running the push by hand is the documented fix. On a push failure the overall command **exits non-zero** (the push step failed) so scripted/CI callers can detect it — but the **commit stays in place** (forward-only, never unwound); the non-zero status signals only the failed push, not a failed commit. Add an acceptance criterion and test asserting the deterministic exit status.
```

**Resolution**: Pending
**Notes**: Pairs with a new acceptance-criterion/test bullet — see proposed addition below if approved.

---

### 5. 1-2 prompt-composition order is asserted ("prompt then diff") without a stated rationale or override interaction

**Severity**: Minor
**Plan Reference**: Phase 1 / commit-command-1-2 (Assemble Conventional Commits prompt).
**Category**: Acceptance Criteria Quality
**Change Type**: update-task

**Details**:
1-2 asserts the composed input is "prompt + staged diff content, **in that order**, and nothing else," with an acceptance criterion and test locking the ordering. That ordering is fine, but the task never says how the **override** (`[commit].prompt`) interacts with it — specifically that the override **replaces the prompt segment only**, with mint still appending the diff in the *same* position (the task says this in prose under the second knob, but the "in that order … and nothing else" criterion/test does not state it holds under override). An implementer reading just the acceptance criteria could place the diff before an override's contents, or treat the override as the whole input (dropping the diff). The override-still-appends-diff rule exists in the Do/Outcome; lifting it into the ordering criterion closes the gap so the locked-order test covers both default and override.

**Current**:
```
- [ ] The composed input is: prompt + staged diff content, in that order, and nothing else.
```

**Proposed**:
```
- [ ] The composed input is: prompt + staged diff content, in that order, and nothing else — and this ordering holds under the `[commit].prompt` override too (the override replaces the **prompt** segment only; mint still appends the diff in the same trailing position, never dropping or re-ordering it).
```

**Resolution**: Pending
**Notes**:

---
