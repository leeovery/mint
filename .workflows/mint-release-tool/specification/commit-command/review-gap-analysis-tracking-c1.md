---
status: in-progress
created: 2026-06-09
cycle: 1
phase: Gap Analysis
topic: commit-command
---

# Review Tracking: commit-command - Gap Analysis

## Findings

### 1. `e` (edit) at the AI-path gate: save-as-accept vs. loop-back is unspecified

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Interactive Review Gate; `$EDITOR` Fallback — Path Semantics

**Details**:
Two different `$EDITOR`-save semantics coexist in the spec without reconciling the overlap.

- The **fallback path** (`--no-ai`, AI failure, oversized diff) defines "the editor save *is* the accept event" — non-empty save commits immediately, no further gate.
- The **gate `e` choice** says: "edit the message in `$EDITOR`, used verbatim." It does not say what happens *after* the editor closes. The cli-presentation seam this gate is built on is explicit that on `e` the engine "re-calls `ShowNotes` with the refreshed body and `Prompt` again, looping until `y`/`n`" — i.e. `e` returns to the gate, it is NOT save-as-accept.

So the same physical act (editing in `$EDITOR` and saving) means "accept and commit" on the fallback path but "return to the gate for another `Continue?`" on the `e` path. An implementer cannot tell whether commit's `e` should follow cli-presentation's loop-back contract or the commit spec's save-as-accept rule. The spec needs to state explicitly that `e` loops back to the gate (inheriting the seam) while only the fallback editor is save-as-accept — or whichever is intended.

A secondary sub-question falls out: if `e`'s editor is loop-back, and the user *empties* the message and saves under `e`, what happens? (Re-prompt? Abort? Keep prior body?) The fallback path defines empty-save = abort, but `e` has no stated empty-save behaviour.

**Proposed Addition**:
{leave blank until discussed}

**Resolution**: Pending
**Notes**:

---

### 2. `r` (regenerate-with-context): how the one-time context line is collected is unspecified

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Interactive Review Gate (`r` choice)

**Details**:
The spec defines `r` as "re-run the AI with a one-time context line" and calls this "the 'context injection' affordance from the user's original commit shell function." But it never specifies *how the user supplies that context line* after pressing `r`: is there a follow-up free-text prompt (type the context, press Enter)? Is it read through the same Presenter line-read model used for the gate? What happens on an empty context line (treat as plain regenerate, or re-prompt)?

The cli-presentation seam describes `r` only as "re-running generation via `claude`" and does not itself define a context-entry prompt — that input step is commit-specific (release's `r` is a plain regenerate with no context line). Because the `--context` flag was consciously dropped in favour of "interactive `r` covers the need," the interactive context-entry mechanism is the *only* surviving path for one-time context, yet its UX is undefined. An implementer would have to invent the prompt, its rendering, and its empty-input behaviour.

**Proposed Addition**:
{leave blank until discussed}

**Resolution**: Pending
**Notes**:

---

### 3. Oversized-diff (`max_diff_lines`) detection ordering relative to the AI call is implied but not stated

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Commit Message Format (`$EDITOR` fallback case 3); Commit Flow / Lifecycle (stages 2–3)

**Details**:
Case 3 of the fallback ("`max_diff_lines` exceeded → fall back to `$EDITOR`") must be detected *before* the L2 AI call (you don't generate, then fall back). The lifecycle lists "Build context (L1)" then "Generate (L2)" but never states that L1 surfaces the over-limit condition and short-circuits L2 into the fallback. Compounding this, the fallback section says the over-limit case opens the editor "with a clear note," whereas `--no-ai` opens with "an empty/template message" — so the oversized case is a *generate-skip*, not a *generate-fail*, and that distinction (caught at L1, never reaching L2) is left implicit.

This matters because the `-y`/non-TTY fail-loud rule keys off "a fallback fires." An implementer needs to know the over-limit check runs at L1, decides the fallback, and *then* the TTY/`-y` forbidden-combo check applies — the same as `--no-ai`. Without stating the ordering, an implementer could wire the limit check after the AI call (wasting the call) or miss that the oversized case must also fail loud under `-y`.

(Note: the line-counting mechanics of `max_diff_lines` themselves — excluded paths don't count, ~lines-as-token-proxy — are settled in the release/engine spec and reused; this finding is only about *commit's* branch ordering, which is commit-specific because commit falls back rather than aborts.)

**Proposed Addition**:
{leave blank until discussed}

**Resolution**: Pending
**Notes**:

---

### 4. `e` / `r` / `$EDITOR` references but no statement of behaviour when `$EDITOR` is unset

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: `$EDITOR` Fallback — Path Semantics; Interactive Review Gate (`e` choice)

**Details**:
Every fallback path and the gate's `e` choice depend on `$EDITOR` being launchable. The spec carefully handles the *no-TTY* case (fail loud) but never addresses the *no-`$EDITOR`* case: what happens on an interactive TTY when the `$EDITOR` environment variable is unset/empty or points to a non-executable? This is a real, common boundary on fresh machines/CI shells that still have a TTY.

Git itself has a defined fallback chain (`GIT_EDITOR` → `core.editor` → `VISUAL` → `EDITOR` → built-in default like `vi`). The commit spec, which positions the fallback as "behaving like plain `git commit`," should state whether mint defers to git's editor resolution (so `git commit` opens whatever git would open), resolves `$EDITOR` itself, or fails loud when nothing is set. As written, an implementer must guess the resolution chain and the unset-variable behaviour.

**Proposed Addition**:
{leave blank until discussed}

**Resolution**: Pending
**Notes**:

---

### 5. `-a` and `-A` passed together: precedence/conflict behaviour undefined

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Staging Model; CLI Surface & Flags

**Details**:
`-a` (`--all`, tracked-only) and `-A` (`--add-all`, includes untracked) are mutually distinct staging modes, but the spec never says what happens if both are supplied (`mint commit -aA`). Options an implementer would have to choose between: error out as a conflicting-flags case; let `-A` win (superset); let `-a` win; or last-flag-wins. Since the two flags are deliberately separated to resolve the tracked-vs-untracked tension, the both-supplied case is a plausible user slip that needs a defined resolution rather than an implementer coin-flip.

**Proposed Addition**:
{leave blank until discussed}

**Resolution**: Pending
**Notes**:

---

### 6. Push under non-default upstream remotes / branch-tracking is delegated to git, but the "no upstream" UX message overlaps the warn-clearly rule ambiguously

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Auto-push Behaviour

**Details**:
The Auto-push section says, for no upstream, surface "git's own failure" via the warn-clearly rule with the example *"commit is in place; set an upstream and push"*. But it also says mint "adds no special upstream logic" and runs "a normal `git push`." A normal `git push` with no upstream on modern git prints a hint and a non-zero exit — but whether mint should pass through git's verbatim stderr, replace it with mint's own *"commit is in place; set an upstream…"* line, or both, is not pinned down. The other push-failure causes (rejected, remote moved, network) are bundled under the same "report clearly with the fix (re-run the push)" rule, but only the no-upstream case gets a bespoke message example — leaving the *other* failure causes' exact user-facing text unspecified (do they all get "re-run the push," or cause-specific guidance?). An implementer needs to know whether mint classifies push-failure causes (and emits tailored fixes) or emits one generic warn for all of them with git's output attached.

**Proposed Addition**:
{leave blank until discussed}

**Resolution**: Pending
**Notes**:

---

### 7. `-A`/`-a` empty-staging diagnostic: which of the two git-flavoured messages fires is underspecified for the staging-flag case

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Staging Model (Empty-staging handling)

**Details**:
The empty-staging section distinguishes two messages: "nothing to commit, working tree clean" (genuinely clean) vs. the "no changes staged — use `-a`/`-A`/`git add`" guidance (dirty-but-unstaged, bare `mint commit`). It then says "`-A`/`-a` that stage nothing land here too" — but doesn't say *which* of the two messages they get. If a user runs `mint commit -a` on a tree whose only changes are untracked files, `-a` (tracked-only) stages nothing, yet the tree is *not* clean and the "use `-a`/`-A`" guidance would be partly nonsensical (they already passed `-a`; the real fix is `-A`). Likewise `mint commit -A` on a genuinely clean tree should presumably yield "working tree clean." The mapping from {which flag, what's actually present} → {which message} is the part an implementer must get right to "mirror git's messaging," and it is left to interpretation.

**Proposed Addition**:
{leave blank until discussed}

**Resolution**: Pending
**Notes**:

---
