---
phase: 2
phase_name: AI Release Notes Engine, Change Map & Interactive Review
total: 16
---

## mint-release-tool-2-1 | approved

### Task mint-release-tool-2-1: AI transport layer (content-agnostic)

**Problem**: Notes generation separates two concerns mirroring the engine's other seams. The **AI transport** is the content-agnostic half: given an already-assembled prompt + content and an `ai_command`, it runs the call, validates the result (non-empty / not-an-error / not-a-refusal), retries once on bad content, and returns the body. It knows nothing about git, diffs, tags, or the Change Map — pure "content in, message out." Without this seam the notes engine has no way to invoke the AI in a way that is trivially testable (a string + a fake `ai_command`) and isolated from all the git-aware assembly logic.

**Solution**: A transport package that takes a composed prompt string + an `ai_command` (default `claude -p`), pipes the prompt to the command's stdin via the Phase 1 `CommandRunner` stdin affordance, reads the body from stdout under a ~60s timeout, validates the output as sanity (non-empty / not whitespace-only / not an error-or-refusal), retries exactly once on bad *content*, and returns the body or a typed transport failure. A timeout is NOT retried.

**Solution note**: This is the content-agnostic half only. Do NOT build diff assembly, exclusion, `max_diff_lines`, the Change Map, or the default prompt here — those are tasks 2-2 through 2-5. The transport receives a finished prompt string and returns a body or a failure.

**Outcome**: Given a prompt string and a runnable `ai_command`, the transport returns the AI's stdout body when it is valid. An empty/whitespace-only/error/refusal body triggers exactly one retry; a second bad body returns a distinguishable "notes failure" (consumed by `on_notes_failure` in task 2-7). A call exceeding ~60s returns a timeout failure with no retry. The `ai_command` is overridable (default `claude -p`); the command is split into name + args and run through the `CommandRunner` with the prompt on stdin.

**Do**:
- Create package `internal/ai` (or `internal/notes/transport`) with a `Transport` type holding the `CommandRunner` and configuration (the `ai_command` string, the timeout).
- Implement `Generate(ctx, prompt string) (string, error)`:
  - Parse `ai_command` into a command name + args (default `"claude -p"` → name `claude`, args `["-p"]`). A simple whitespace split is sufficient for the default; document that full shell-quoting of `ai_command` is not required (the value is operator-controlled config).
  - Invoke via the Phase 1 `CommandRunner` stdin path (`RunWith`/stdin affordance from task 1-1), writing `prompt` to the command's stdin and reading the body from stdout.
  - Apply a ~60s timeout via `context.WithTimeout` so a hung call cannot stall a release.
- Implement validation (sanity, not structure): a body is **bad** if it is empty, whitespace-only, or looks like an error/refusal. Keep refusal/error detection light and documented (e.g. a non-zero exit from the command, or a body that is only an error/refusal sentinel) — there is no machine-parseable wrapper to validate.
- Retry policy: on a **bad-content** result (empty / error / refusal that came back as a completed call), retry the call exactly **once**. If the second result is still bad → return a typed notes-failure error. A **timeout** (or command-not-found / missing tool) is NOT retried — return the failure immediately. The single retry bounds worst-case latency at one ~60s timeout.
- Return typed, distinguishable failure causes (timeout, missing-tool, empty/error/refusal-after-retry) so the caller (`on_notes_failure`, task 2-7) can report the cause.
- All invocation goes through the `CommandRunner`; tests use the `FakeRunner` to script stdout/exit/timeout for the `ai_command`.

**Acceptance Criteria**:
- [ ] A valid (non-empty, non-error) stdout body is returned unchanged.
- [ ] An empty or whitespace-only body triggers exactly one retry, then a notes-failure if still bad.
- [ ] An error/refusal body triggers exactly one retry, then a notes-failure if still bad.
- [ ] A timeout is NOT retried and returns a distinguishable timeout failure.
- [ ] A missing AI tool (command-not-found) returns a distinguishable failure and is not retried.
- [ ] The prompt is delivered on the command's stdin; the body is read from stdout.
- [ ] `ai_command` is overridable (default `claude -p`); a custom command is split and invoked correctly.
- [ ] The transport contains no git/diff/Change Map logic (content-agnostic).
- [ ] All AI invocation goes through the `CommandRunner`/`FakeRunner`.

**Tests**:
- `"it returns a valid body unchanged"`
- `"it retries once on an empty/whitespace-only body then fails"`
- `"it retries once on an error/refusal body then fails"`
- `"it succeeds on the second attempt when the first body was bad"`
- `"it does not retry a timeout and returns a timeout failure"`
- `"it does not retry a missing AI tool (command-not-found)"`
- `"it pipes the prompt to stdin and reads the body from stdout"`
- `"it honours an overridden ai_command"`

**Edge Cases**:
- Empty/whitespace body → retry then fail.
- Error/refusal text → retry then fail.
- Timeout → not retried, straight to failure.
- `ai_command` override (non-default binary/args).

**Context**:
> Engine layering: "AI transport (content-agnostic): takes an assembled prompt + content + `ai_command`, runs the call, validates (non-empty / not-an-error / not-a-refusal), retries once, and returns the body. It knows nothing about git, diffs, or tags — pure 'content in, message out.' The boundary keeps the transport trivially testable (a string + a fake `ai_command`)." Engine: "Default `claude -p`. mint composes the prompt, pipes it to the command's stdin, and reads the body from stdout, with a timeout (~60s) so a hung call can't stall a release. Command overridable via `ai_command` (default `claude -p`)." Validation: "Validation is sanity, not structure: non-empty, not an error/refusal/whitespace. On a bad/empty generation → one automatic retry → still bad → notes failure → `on_notes_failure`. A timeout is not retried — it goes straight to `on_notes_failure` … the single retry covers empty/error/refusal content only, bounding worst-case latency at one ~60s timeout." `on_notes_failure` routing is task 2-7; this task surfaces a typed failure.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 4 → Engine layering", "Engine", "Output format & validation".

## mint-release-tool-2-2 | approved

### Task mint-release-tool-2-2: Diff context assembly (last_tag..HEAD, CHANGELOG.md always-excluded)

**Problem**: The notes engine generates from the **release diff and nothing else** — commit messages/history are deliberately not ingested. The git-aware context assembly half must produce the `last_tag..HEAD` diff with the built-in always-exclude of `CHANGELOG.md` applied via git's `:(exclude)` pathspec, so the AI never sees mint's own changelog churn. This is the foundational input the `max_diff_lines` guard, the Change Map, and the prompt all build on; without it there is no content to feed the transport.

**Solution**: A context-assembly function that runs `git diff {last_tag}..HEAD` through the `CommandRunner` with the built-in `CHANGELOG.md` exclusion expressed as a git `:(exclude)` pathspec, returning the post-exclusion diff text. Exclusion is path-based (git does the filtering), never commit-based.

**Solution note**: Phase 2 uses ONLY the built-in always-exclude of `CHANGELOG.md`. Configurable `diff_exclude` globs and the strategy-aware `version_file` exclusion are Phase 3 — do NOT implement them here. The diff base is `last_tag..HEAD` (prior-tag releases); first-release (no prior tag) never reaches this path (it is the fixed-body precedence winner, task 2-10).

**Outcome**: Given a prior tag and HEAD, the assembly returns the `last_tag..HEAD` diff with `CHANGELOG.md` changes excluded. A diff in which the only change is to `CHANGELOG.md` returns empty (feeding the degenerate path, task 2-8). A gitignored-but-force-added (tracked) file still appears in the diff (not special-cased). The exclusion is realised through git's pathspec, not by post-filtering text.

**Do**:
- In `internal/notes` (context-assembly side, distinct from the task 2-1 transport), implement `AssembleDiff(ctx, lastTag string) (string, error)`.
- Build the diff command through the `CommandRunner`: `git diff {lastTag}..HEAD -- . ':(exclude)CHANGELOG.md'` (or the equivalent pathspec form). Let **git** perform the exclusion via `:(exclude)` — do not read the full diff and strip CHANGELOG hunks in Go.
  - `CHANGELOG.md` is the repo-root changelog; express the exclusion so it matches the file mint itself writes (Stage 5). Document that this is the **non-configurable built-in** exclusion, excluded in both forward and regenerate paths.
- The diff base is `last_tag..HEAD`: the previous-release tag (the current "latest", task 1-3) to HEAD. HEAD is the post-hook HEAD in later phases; in Phase 2 there are no hooks, so HEAD is the working HEAD.
- Note in code that exclusion is **path-based, never commit-based**: a git range diff operates on paths/content and cannot subtract commits; path exclusion is the only mechanism.
- Force-added gitignored files: a file that is gitignored yet force-added is nonetheless *tracked*, so it can appear in a commit-to-commit diff. This is deliberate and **not special-cased** — the assembly does nothing to suppress it.
- Return the raw post-exclusion diff text (the Change Map preamble and `max_diff_lines` capping are layered on by other tasks; this task returns the diff body only).

**Acceptance Criteria**:
- [ ] The diff is computed for `last_tag..HEAD` through the `CommandRunner`.
- [ ] `CHANGELOG.md` is excluded via a git `:(exclude)` pathspec (git does the filtering, not Go-side text stripping).
- [ ] A change set whose only modification is `CHANGELOG.md` yields an empty post-exclusion diff.
- [ ] A force-added gitignored (tracked) file still appears in the diff (not suppressed).
- [ ] Exclusion is path-based — no attempt to drop commits from the range.
- [ ] Configurable `diff_exclude` and `version_file` exclusion are NOT present (deferred to Phase 3).
- [ ] All git calls go through the `CommandRunner`/`FakeRunner`.

**Tests**:
- `"it diffs last_tag..HEAD"`
- `"it excludes CHANGELOG.md via a git exclude pathspec"`
- `"a diff that only touches CHANGELOG.md is empty after exclusion"`
- `"a force-added gitignored tracked file still appears in the diff"`
- `"it returns the post-exclusion diff text for downstream layering"`

**Edge Cases**:
- `CHANGELOG.md`-only changes excluded → empty diff (feeds degenerate path).
- Force-added gitignored file still tracked → still appears.
- No source change after exclude → empty diff.

**Context**:
> Source of truth: "mint generates notes from the release diff and nothing else. Commit messages / history are deliberately not ingested." Diff base: "Diff `last_tag..HEAD` (changes since the last release)." Diff exclusion: "The diff sent to the AI is filtered via git's `:(exclude)` pathspec (git does the filtering): Built-in always-exclude — `CHANGELOG.md` (non-configurable). Pure mint output, never meaningful source. Excluded in both forward and regenerate paths." "A release diff is commit-to-commit so it can only contain tracked files; gitignored files never appear. A file that is gitignored yet force-added is nonetheless tracked, so it can still appear in the diff — this edge is deliberate and not special-cased." "Exclusion is path-based, never commit-based." `diff_exclude` globs and strategy-aware `version_file` exclusion are Phase 3.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 4 → Source of truth, Diff base, Diff exclusion".

## mint-release-tool-2-3 | approved

### Task mint-release-tool-2-3: max_diff_lines guard (default 50000)

**Problem**: A huge diff is slow, costly, and summarises to mush — it is a cost + quality problem, not a context-window problem. mint needs a guard at the *large* end of the diff size range: if the post-exclusion diff exceeds `max_diff_lines` (default 50000), generation is a notes failure routed through `on_notes_failure`, rather than sending a giant diff to the AI. Lines are a cheap token proxy. Excluded paths must not count toward the limit (they were never sent). Without this guard, big releases are expensive and produce poor notes.

**Solution**: A line-count guard layered onto the context assembly: count the lines of the **post-exclusion** diff (the output of task 2-2), compare to `max_diff_lines` (default 50000, configurable), and signal a distinguishable "diff too large" notes failure when exceeded so the caller routes it through `on_notes_failure` (task 2-7).

**Solution note**: This guard is the too-big member of the "don't run the AI on a bad-sized diff" family; the too-small/empty member is the degenerate-diff stub (task 2-8). The trimmed-diff escalation lever is explicitly parked (not built in Phase 2).

**Outcome**: A post-exclusion diff of exactly 50000 lines passes; 50001 lines is a notes failure ("diff exceeds max_diff_lines"). The limit is overridable via config. Lines from excluded paths (e.g. `CHANGELOG.md`) never count, because the count runs on the already-excluded diff.

**Do**:
- Extend the context-assembly side (`internal/notes`) with a guard applied to the post-exclusion diff text produced by task 2-2.
- Count lines of the post-exclusion diff (newline count; document whether a trailing partial line counts — be consistent and tested). Because the diff handed in is already post-exclusion, **excluded paths are inherently not counted** — do not re-add them.
- Compare against `max_diff_lines`: default **50000**, read from the shared top-level config key (Phase 2 reads only the keys it needs with defaults; full schema validation is Phase 6). The boundary is inclusive — exactly `max_diff_lines` passes; strictly greater fails.
- On exceed, return a distinguishable **"diff too large"** notes failure (a cause the `on_notes_failure` resolver in task 2-7 can name in its message, e.g. "diff exceeds max_diff_lines (N > 50000)"). Do NOT call the AI when over the limit.
- Do NOT implement the parked escalation (Change Map + *trimmed* diff instead of failing at the cap) — note it as deferred future work in a comment.
- Wire `max_diff_lines` from config with a default of 50000 when absent.

**Acceptance Criteria**:
- [ ] A post-exclusion diff of exactly 50000 lines passes the guard.
- [ ] A post-exclusion diff of 50001 lines is a "diff too large" notes failure and the AI is not called.
- [ ] `max_diff_lines` is configurable; a custom limit is honoured at its own boundary.
- [ ] Excluded-path lines do not count (the count runs on the post-exclusion diff only).
- [ ] The too-large failure is distinguishable so `on_notes_failure` (task 2-7) can route and name it.
- [ ] The trimmed-diff escalation is NOT implemented (deferred).

**Tests**:
- `"a diff of exactly max_diff_lines passes"`
- `"a diff over max_diff_lines is a notes failure and skips the AI"`
- `"a custom max_diff_lines override is honoured at its boundary"`
- `"excluded-path lines do not count toward the limit"`
- `"the too-large failure carries a distinguishable cause"`

**Edge Cases**:
- Exactly 50000 → passes.
- 50001 / over → notes failure.
- Configurable override.
- Excluded paths not counted.

**Context**:
> `max_diff_lines` guard: "Default 50000. Not a context limit but a cost + quality guard — a huge diff is slow, costly, and summarises to mush. Lines are a cheap token proxy (~10–20 tokens/line). Excluded paths don't count toward it. Exceeding it = a notes failure → abort-or-fallback per `on_notes_failure`. Fully overridable." Degenerate release: "One coherent family of 'don't run the AI on a bad-sized diff' guards: too-big → fallback/abort per `on_notes_failure`; too-small/empty → stub, no AI." Big-diff handling: "An intermediate lever (Change Map + a trimmed diff rather than falling back at the cap) is noted for the same future. Revisit only on observed need." Config: `max_diff_lines = 50000` is a shared top-level engine key. Full fail-loud config validation is Phase 6.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 4 → max_diff_lines guard, Degenerate release, Big-diff handling".

## mint-release-tool-2-4 | approved

### Task mint-release-tool-2-4: Change Map salience preamble

**Problem**: The motivating failure — "glosses over the big feature on big releases" — is a **salience** problem (misallocated attention), not a missing-data problem; feeding more raw diff makes it worse. mint needs a computed **Change Map** that tells the AI what to prioritize: structural novelty (new/removed/renamed paths, especially new directories/packages) weighted *above* raw magnitude, with directory/area rollup and individually-notable files called out. It is prepended to the AI input. Without it, the AI has no salience signal and large releases summarise to mush.

**Solution**: A Change Map builder that runs cheap git commands (name-status / numstat over `last_tag..HEAD`, after the same `CHANGELOG.md` exclusion) to derive structural novelty and per-area magnitude, then renders a compact preamble: novelty signals first (new directories/packages, renamed/removed paths), magnitude as supporting context, rolled up by directory/area with notable files (new top-level entries, the single largest file) called out. Computed **after** exclusion.

**Solution note**: The Change Map is salience *metadata*, not content. It must be computed after the `CHANGELOG.md` exclusion (task 2-2), so bulk noise is already removed and post-exclude magnitude is trustworthy. In Phase 2, exclusion is only `CHANGELOG.md`; `diff_exclude`/`version_file` exclusion arrives in Phase 3 but the Change Map already runs after whatever exclusion exists.

**Outcome**: For a diff that introduces a whole new `auth/` package, the Change Map headlines that new directory **above** a larger-line-count refactor of an existing area. Renamed and removed paths are reported as structural changes. Per-area churn is ranked as supporting magnitude ("400 lines here, 3 there"). The single largest file and new top-level entries are called out. When all changes sit in one existing area, the map rolls them up by that area with no spurious novelty headline. The rendered preamble is suitable to prepend ahead of the diff.

**Do**:
- In `internal/notes` (assembly side), implement `BuildChangeMap(ctx, lastTag string) (string, error)` (or fold the computation into the assembler), using cheap git commands through the `CommandRunner`:
  - `git diff --name-status {lastTag}..HEAD -- . ':(exclude)CHANGELOG.md'` for added (`A`)/modified (`M`)/deleted (`D`)/renamed (`R`) path status — the structural-novelty signal.
  - `git diff --numstat {lastTag}..HEAD -- . ':(exclude)CHANGELOG.md'` for per-file added/removed line counts — the magnitude signal.
- Derive **structural novelty (primary)**: new paths, removed paths, renamed paths — *especially new directories/packages appearing* (a path under a top-level directory that had no prior entries). Weight novelty **above** magnitude in both ordering and the rendered emphasis.
- Derive **magnitude (secondary)**: roll per-file churn up to **directory/area** granularity and rank areas by churn; present as supporting context, not the headline.
- Call out **individually-notable files**: new top-level entries and the single largest file (by churn). A flat per-file list is itself mush on big releases — rollup is the salience-preserving form.
- Render a compact preamble string with novelty first, then magnitude rollup, then notable files. Keep it salience metadata (concise), not a diff restatement.
- Compute **after exclusion** — the same `:(exclude)CHANGELOG.md` (Phase 2's only exclusion) — so the map reflects what the AI actually sees.
- This task produces the Change Map string; *prepending* it to the diff to form the full AI input is wired in task 2-5/2-6. The prompt discipline ("rank with the map, describe from the diff") is authored in task 2-5.

**Acceptance Criteria**:
- [ ] A new directory/package is headlined as structural novelty **above** a larger-magnitude change to an existing area.
- [ ] Renamed and removed paths are reported as structural changes.
- [ ] Per-area churn is ranked and presented as supporting magnitude context.
- [ ] The single largest file and new top-level entries are called out individually.
- [ ] When all changes are within one existing area, the map rolls them up by area with no false novelty headline.
- [ ] The map is computed after the `CHANGELOG.md` exclusion (excluded churn never appears).
- [ ] Output is a compact preamble suitable to prepend before the diff.
- [ ] All git calls go through the `CommandRunner`/`FakeRunner`.

**Tests**:
- `"a new directory/package headlines above a larger existing-area change"`
- `"it reports renamed and removed paths as structural changes"`
- `"it ranks per-area churn as supporting magnitude"`
- `"it calls out the single largest file"`
- `"it calls out new top-level entries"`
- `"all-in-one-existing-area changes roll up with no false novelty headline"`
- `"it computes the map after CHANGELOG.md exclusion"`

**Edge Cases**:
- New directory/package headline above magnitude.
- Renamed/removed paths.
- Single largest file called out.
- All changes in one existing area (rollup, no false headline).

**Context**:
> Change Map: "a computed Change Map that mint assembles (cheap git commands) and prepends to the AI input, telling the AI what to prioritize. Structural novelty (primary signal): new / removed / renamed paths — especially new directories or packages appearing. 'A whole new `auth/` package showed up' is the strongest language-agnostic headline signal there is … Weighted above raw magnitude, in both ordering and how the prompt is told to read it. Magnitude (secondary signal): per-area churn ranking, as supporting context ('400 lines here, 3 there'). Granularity — directory/area rollup by default, with individually-notable files called out (new top-level entries, the single largest file). A flat list of every changed file is itself mush on big releases … so rollup is the salience-preserving form. Computed after `diff_exclude` (the map runs after exclusion, never before). Prompt discipline … the prompt says rank importance using the Change Map but describe changes from the diff. The map is salience metadata, not content." "The AI input is therefore: the Change Map preamble, then the post-`diff_exclude` (and `max_diff_lines`-capped) diff — nothing else." In Phase 2 the only exclusion is `CHANGELOG.md`.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 4 → Change Map".

## mint-release-tool-2-5 | approved

### Task mint-release-tool-2-5: Default notes prompt & Keep-a-Changelog emoji-skin format

**Problem**: The AI must return notes **directly in presentation format** (no machine labels) following mint's default format: a TL;DR one-liner over emoji-headed Keep a Changelog sections, empty sections omitted, version-number bumps ignored, no preamble/meta-commentary. mint also exposes two prompt knobs — `context` (inject project guidance) and `prompt` (full override file). Without the default prompt and these knobs, the transport has nothing meaningful to send and the format/quality bar cannot be met.

**Solution**: A prompt composer that builds the full AI input from: mint's default Keep-a-Changelog emoji-skin prompt (carrying the format rules and the Change-Map salience discipline), the Change Map preamble (task 2-4), and the post-exclusion/`max_diff_lines`-capped diff (tasks 2-2/2-3). It injects `[release].context` into the default prompt when set, and fully replaces the default prompt with a `[release].prompt` file when set (mint still supplies the Change Map + diff).

**Solution note**: This task owns prompt *composition* and the default prompt text. The actual AI call is the transport (task 2-1); the normal-path wiring that threads assembly → prompt → transport → body is task 2-6. The exact emoji↔category mapping and prompt wording are explicitly "ship-and-refine" — pick sensible defaults and keep them tunable.

**Outcome**: The default prompt instructs the AI to emit a **TL;DR one-liner** (may be multi-line) over **emoji-headed Keep a Changelog sections** (`Added / Changed / Deprecated / Removed / Fixed / Security`, e.g. `✨ Added`, `🔧 Changed`, `🐛 Fixed`, `🗑️ Removed`), to **omit empty sections**, to **bold + describe notable features**, to treat the unit of entry as the **notable item** (not file/hunk/commit), to **ignore version-number bumps**, to emit **no preamble/meta-commentary**, to **rank with the Change Map but describe from the diff**, and to emit `Deprecated`/`Security` **only on an explicit textual marker**. A configured `context` string/file is injected into the default prompt; a configured `prompt` file fully overrides the default while mint still supplies the Change Map + diff.

**Do**:
- In `internal/notes`, implement `ComposePrompt(changeMap, diff string, cfg) string` that assembles the full AI input: prompt instructions + the Change Map preamble + the post-exclusion (capped) diff — "nothing else."
- Author the **default prompt** text encoding all format rules:
  - A **TL;DR one-liner** at the top (a unified cross-change narrative synthesized from the whole diff; may be multi-line), sitting above the categorized sections.
  - **Emoji-headed sections** keyed to the Keep a Changelog taxonomy `Added / Changed / Deprecated / Removed / Fixed / Security`, with emoji headers (e.g. `✨ Added`, `🔧 Changed`, `🐛 Fixed`, `🗑️ Removed`). **Omit empty sections entirely.**
  - **Unit of entry = the notable item** (a change that adds a feature and fixes a bug yields two items in two sections).
  - **Notable features bolded + described.**
  - **Ignore version-number bumps** and trivial bookkeeping churn.
  - **Strict "no preamble, no meta-commentary"** so prompt artifacts can never leak.
  - **Salience discipline:** "rank importance using the Change Map, but describe changes from the diff" — the map is metadata, never narrate a file as a feature merely because it's large or new.
  - **Diff-inferability tiers:** `Added / Changed / Fixed / Removed` are diff-readable; `Deprecated` and `Security` are **opportunistic** — emit only on an *explicit textual marker* (a `@deprecated`/deprecation annotation; an obvious security surface — auth/crypto/input-validation, a CVE-referencing dependency bump). Light guidance, expected empty most releases.
- Implement the two prompt knobs:
  - **`[release].context`** (string or file) — **inject** the value into the default prompt (does not replace it). The common case.
  - **`[release].prompt`** (file path) — **full override**: replace the default prompt text with the file's contents; mint still appends the Change Map + diff. A "theme/variant" is just a `prompt` override — no theme enum.
- The AI returns notes directly in presentation format — there is no machine-parseable wrapper, so this task produces only the *input*; no output parsing exists.
- Read only the `[release]` keys needed (`context`, `prompt`) with defaults (absent = default prompt, no injection); full schema validation is Phase 6.

**Acceptance Criteria**:
- [ ] The composed input is: prompt + Change Map preamble + post-exclusion (capped) diff, in that order, and nothing else.
- [ ] The default prompt instructs: TL;DR one-liner, emoji-headed KaC sections, omit empty sections, bold+describe notable features, notable-item unit, ignore version bumps, no preamble/meta-commentary.
- [ ] The default prompt carries the salience discipline (rank with Change Map, describe from diff).
- [ ] `Deprecated`/`Security` are instructed as opportunistic — only on an explicit textual marker.
- [ ] `[release].context` (string or file) is injected into the default prompt without replacing it.
- [ ] `[release].prompt` file fully overrides the default prompt while mint still supplies the Change Map + diff.
- [ ] No machine-parseable output wrapper is requested (presentation format directly).

**Tests**:
- `"the composed input is prompt + change map + capped diff in order"`
- `"the default prompt requests a TL;DR one-liner over emoji-headed KaC sections"`
- `"the default prompt instructs omit-empty-sections and ignore-version-bumps"`
- `"the default prompt carries the no-preamble/no-meta-commentary rule"`
- `"the default prompt carries the rank-with-map-describe-from-diff discipline"`
- `"Deprecated/Security are instructed only on an explicit marker"`
- `"a configured context string is injected into the default prompt"`
- `"a configured prompt file fully overrides the default prompt"`

**Edge Cases**:
- `context` inject (string or file) appended, not replacing.
- `prompt` full-override file used verbatim with Change Map + diff still supplied.
- `Deprecated`/`Security` only on explicit marker.

**Context**:
> Default notes format: "anchors on the Keep a Changelog convention … rendered in mint's emoji skin. A TL;DR one-liner at the top (may be multi-line) … sitting above the categorized sections. Emoji-headed sections keyed to the Keep a Changelog taxonomy — the canonical bucket set is `Added / Changed / Deprecated / Removed / Fixed / Security`, rendered with emoji headers (e.g. `✨ Added`, `🔧 Changed`, `🐛 Fixed`, `🗑️ Removed`). Empty sections are omitted entirely. Unit of entry = the notable item, not the file / hunk / commit … Notable features bolded + described. Diff-inferability tiers the categories. `Added / Changed / Fixed / Removed` are readable from a diff. `Deprecated` and `Security` are intent-laden … kept in the vocabulary but treated as opportunistic: emit only on an explicit textual marker … Strict 'no preamble, no meta-commentary' rule. Default prompt instructs the AI to ignore version-number bumps." Prompt control: "`[release].context` (string or file) — injects project-specific guidance into mint's default prompt … `[release].prompt` (file path) — full override of the prompt; mint still supplies the diff. A 'theme/variant' is not a separate feature — it's just a `[release].prompt` override file." Output: "The AI returns the notes directly in presentation format — no machine-parseable wrapper labels." Source confidence: "the exact emoji↔category mapping and prompt wording are explicitly ship-and-refine."

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 4 → Default notes format, Prompt control, Output format & validation, Change Map (prompt discipline)".

## mint-release-tool-2-6 | approved

### Task mint-release-tool-2-6: Normal AI notes path wiring (prior-tag release)

**Problem**: The assembly (diff, `max_diff_lines`, Change Map — tasks 2-2/2-3/2-4), the prompt composer (task 2-5), and the transport (task 2-1) exist as separate pieces but nothing threads them into the single "normal AI path" that takes a prior-tag release from `last_tag` to a validated notes body. This is the engine's core value-add for the common case (a release with a prior tag and a real diff); without the wiring there is no end-to-end notes body to distribute or review.

**Solution**: A notes-generation entry point for the normal AI path that orchestrates: assemble the post-exclusion diff → apply the `max_diff_lines` guard → build the Change Map → compose the prompt (Change Map + capped diff + default/overridden prompt, with `context` injected) → call the transport → return the validated body **whole**. No parsing, no splitting, no per-sink reassembly — the body the AI returns is the body mint uses.

**Solution note**: This task is the *normal AI path* only. Notes-path precedence (first-release / degenerate / `--no-ai` winning before this path runs) is task 2-10; `on_notes_failure` routing of this path's failures is task 2-7. This task produces the body and surfaces failures; it does not decide abort-vs-fallback.

**Outcome**: For a prior-tag release with a real post-exclusion diff, the engine returns the AI body exactly as generated (used whole). A valid generation passes through unchanged (no transformation). A transport/guard failure surfaces as a typed notes failure for the caller to route. The body is a single string suitable for all three sinks.

**Do**:
- In `internal/notes`, implement `Generate(ctx, lastTag string, cfg) (string, error)` for the normal AI path, orchestrating the pieces in order:
  1. `AssembleDiff(lastTag)` (task 2-2) → post-exclusion diff.
  2. Apply the `max_diff_lines` guard (task 2-3) → on exceed, return the "diff too large" notes failure (do not call the AI).
  3. `BuildChangeMap(lastTag)` (task 2-4) → preamble.
  4. `ComposePrompt(changeMap, diff, cfg)` (task 2-5) → full AI input (with `context` injection / `prompt` override applied).
  5. `transport.Generate(prompt)` (task 2-1) → validated body or a typed notes failure (after the one retry / timeout rules).
- Return the body **whole** — no parsing, no splitting, no label extraction, no per-sink reassembly. The same string flows to every sink (task 2-11).
- Surface a typed notes failure (carrying the cause: too-large, timeout, empty/error/refusal-after-retry, missing tool) for the caller; do NOT decide abort vs fallback here (that is `on_notes_failure`, task 2-7).
- The degenerate-diff short-circuit (empty/all-excluded → stub, no AI) is task 2-8 and sits *in front* of this path via precedence (task 2-10); this task assumes a non-degenerate diff when invoked. (It is acceptable to leave the empty-diff branch to the precedence resolver; document the assumption.)

**Acceptance Criteria**:
- [ ] For a prior-tag release with a real diff, the engine returns a validated AI body.
- [ ] The body is returned **whole** — no parsing/splitting/reassembly.
- [ ] A valid generation passes through unchanged.
- [ ] The pieces are invoked in order: assemble diff → max_diff_lines guard → Change Map → compose prompt → transport.
- [ ] A guard or transport failure surfaces as a typed notes failure (cause preserved) without deciding abort/fallback.
- [ ] The AI input is exactly Change Map + capped diff + prompt (no extra data such as commit messages).
- [ ] All external calls go through the `CommandRunner`/`FakeRunner`.

**Tests**:
- `"it returns a validated AI body for a prior-tag release with a real diff"`
- `"it uses the body whole with no parsing or splitting"`
- `"a valid generation passes through unchanged"`
- `"it invokes assemble → guard → change map → compose → transport in order"`
- `"a too-large diff surfaces a notes failure without calling the AI"`
- `"a transport failure surfaces a typed notes failure with the cause preserved"`

**Edge Cases**:
- Body used whole (no parsing).
- Valid generation passes through unchanged.

**Context**:
> Stage 4 intro: "Generate a release-notes body from the diff since the last release. The same body is reused for every output surface (tag annotation, CHANGELOG, provider release) — generate once, use everywhere." Output: "mint uses the body whole for every sink; no parsing, no splitting, no per-sink reassembly." Change Map: "The AI input is therefore: the Change Map preamble, then the post-`diff_exclude` (and `max_diff_lines`-capped) diff — nothing else." Engine layering separates assembly (git-aware) from transport (content-agnostic). Notes-path precedence and `on_notes_failure` routing are tasks 2-10 and 2-7.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 4 → intro, Engine layering, Change Map, Output format & validation".

## mint-release-tool-2-7 | approved

### Task mint-release-tool-2-7: on_notes_failure resolution (abort default / fallback)

**Problem**: When the normal AI path can't produce a body (missing tool, timeout, error, diff exceeds `max_diff_lines`, or a bad/empty generation surviving one retry), mint must decide what to do — and it must fail loud by default. Notes generate before the tag, so aborting leaves nothing tagged; an empty/garbage release is worse than a failed command. The `on_notes_failure` setting (default `abort`, opt-in `fallback`) governs **only the normal AI path**; it needs a resolver. Without it, a notes failure has no defined consequence.

**Solution**: An `on_notes_failure` resolver that takes a normal-path notes failure (from task 2-6) and the config setting, and either aborts the release loudly (default, tagging nothing) or proceeds with a non-AI **fallback body** (commit-subject list since the last tag, or a fixed configurable string). It governs only the normal path — the first-release / degenerate / `--no-ai` paths never reach it.

**Outcome**: With `on_notes_failure = abort` (default), any normal-path notes failure aborts loudly before the tag, with a message naming the cause (e.g. timeout, diff too large). With `on_notes_failure = fallback`, the same failure proceeds with the fallback body: by default the **commit-subject list** since the last tag, or a **fixed configurable string** if one is set. Varied causes (timeout, missing tool, empty-after-retry, too-large) all route the same way.

**Do**:
- In `internal/notes` (or a small resolver alongside the orchestrator), implement `ResolveFailure(failure, cfg) (body string, abort error)`:
  - Read `on_notes_failure` from `[release]` config; default **`abort`**.
  - **`abort`** (default): return an abort error that names the failure cause, so the run stops before the tag (nothing tagged/pushed). The Presenter surfaces it (wired in task 2-16).
  - **`fallback`**: return a fallback body and no abort. The fallback body defaults to the **commit-subject list since the last tag** (`git log --format=%s {last_tag}..HEAD` through the `CommandRunner`); if a fixed fallback string is configured, use that instead.
- Build the commit-subject-list fallback body via the `CommandRunner` (this is metadata about commits used only as a fallback record — it is NOT the AI input; the AI never ingests commit messages).
- Scope strictly to the **normal AI path**: this resolver is invoked only when task 2-6 surfaces a failure. The first-release fixed body, degenerate stub, and `--no-ai` fallback never invoke it (they never call the AI) — document this and ensure the precedence resolver (task 2-10) does not route them here.
- Share the fallback-body construction with the `--no-ai` path (task 2-9), which also uses a commit-subject list / fixed string — factor the fallback-body builder so both consume it.

**Acceptance Criteria**:
- [ ] `on_notes_failure` defaults to `abort`: a normal-path failure aborts before the tag with the cause named, tagging nothing.
- [ ] `on_notes_failure = fallback`: a normal-path failure proceeds with the fallback body.
- [ ] The fallback body defaults to the commit-subject list since the last tag.
- [ ] A configured fixed fallback string is used instead of the commit-subject list when set.
- [ ] Varied failure causes (timeout, missing tool, empty-after-retry, diff too large) all route through the same resolution.
- [ ] The resolver governs only the normal AI path (never invoked for first-release/degenerate/`--no-ai`).
- [ ] The commit-subject list is built via the `CommandRunner`/`FakeRunner` and is never fed to the AI.

**Tests**:
- `"abort (default) aborts before the tag and names the cause"`
- `"fallback proceeds with the commit-subject list body"`
- `"fallback uses a fixed configurable string when set"`
- `"a timeout, missing tool, empty-after-retry, and too-large all route through resolution"`
- `"the resolver is not invoked for first-release/degenerate/--no-ai paths"`

**Edge Cases**:
- abort default → tags nothing.
- fallback → commit-subject list.
- fallback → fixed configurable string.
- varied failure causes (timeout, missing tool, empty-after-retry, too-large).

**Context**:
> Failure behaviour: "Notes generate at Stage 4, before the tag (Stage 6), so aborting leaves nothing tagged/pushed — which is why blocking is safe. `on_notes_failure`, default `abort` — if the AI can't produce a body (missing tool, timeout, error, diff exceeds `max_diff_lines`, or a bad/empty generation that survives one retry), mint fails loudly and tags nothing. An empty/garbage release is worse than a failed command. `fallback` mode (opt-in) — proceed with a non-AI body instead of aborting. Fallback body defaults to the commit-subject list since the last tag; can be a fixed configurable string." Notes-path precedence: "`on_notes_failure` governs only the normal AI path — steps 1–3 never call the AI, so they can't trigger it." Source of truth: commit messages are never the AI input — the commit-subject list is only a fallback *record*.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 4 → Failure behaviour, Notes-path precedence".

## mint-release-tool-2-8 | approved

### Task mint-release-tool-2-8: Degenerate-diff stub path

**Problem**: An empty or all-excluded diff is the one input the AI will reliably hallucinate on. A re-tag with no source change, a release where every changed file fell under exclusion, or pure churn with nothing notable must NOT be sent to the AI. mint needs the guard at the *small* end of the diff-size family: if the post-exclusion diff is empty or whitespace-only, write a minimal honest stub entry and never call the AI. Without it, a no-op release either hallucinates a fake feature or errors out.

**Solution**: A degenerate-diff detector that inspects the post-exclusion diff (task 2-2): if it is empty or whitespace-only, it produces a minimal, honest stub body (version header context + a short stub line such as "Maintenance release — no notable source changes") and short-circuits — the AI is never invoked, no hard error, no skipped entry.

**Solution note**: This is the too-small/empty member of the "don't run the AI on a bad-sized diff" family (the too-big member is `max_diff_lines`, task 2-3). It is precedence step 2 (above `--no-ai`, below first-release); the precedence wiring is task 2-10. In Phase 2 "all-excluded" means only `CHANGELOG.md` exclusion; `diff_exclude` arrives in Phase 3 but this detector already runs on whatever post-exclusion diff exists.

**Outcome**: When the post-exclusion diff is empty (only `CHANGELOG.md` changed), all-excluded, or whitespace-only, mint writes a minimal stub entry (version header + a short honest stub line) with **no `claude` invocation recorded**. A real release still produces a truthful record; there is no hallucination and no hard error.

**Do**:
- In `internal/notes`, implement `IsDegenerate(diff string) bool` (empty or whitespace-only post-exclusion diff) and a `StubBody()` returning the minimal honest stub line (e.g. "Maintenance release — no notable source changes"). Keep the exact wording configurable-in-code/tunable but ship a sensible default.
- The detector consumes the **post-exclusion** diff from task 2-2, so "all changed files fell under exclusion" and "only `CHANGELOG.md` changed" both reduce to an empty post-exclusion diff — handle them uniformly.
- On degenerate, **short-circuit**: do NOT call the transport (task 2-1) and do NOT build a Change Map for the AI. The stub body flows to the sinks like any other body (task 2-11), under the version header in the changelog (`## [x.y.z] - date` from Phase 1's writer).
- Whitespace-only diffs count as degenerate (pure churn with nothing notable).
- This path offers only `y`/`n`/`e` at the review gate (no `r`, no AI to nudge) — the gate variant is task 2-14; this task just produces the stub body and the no-AI signal.

**Acceptance Criteria**:
- [ ] An empty post-exclusion diff produces the stub body with no AI invocation.
- [ ] An all-excluded diff (every changed file excluded; in Phase 2 = only `CHANGELOG.md` changed) is treated as degenerate.
- [ ] A whitespace-only diff is treated as degenerate.
- [ ] The stub is a minimal honest entry (version header + a short stub line), not a hard error and not a skipped entry.
- [ ] The AI/`claude` is never invoked on the degenerate path.
- [ ] The stub body is a normal body that flows to the sinks (task 2-11).

**Tests**:
- `"an empty post-exclusion diff produces a stub with no AI call"`
- `"an all-excluded diff is degenerate (only CHANGELOG.md changed)"`
- `"a whitespace-only diff is degenerate"`
- `"the stub is a minimal honest entry, not an error or a skipped entry"`
- `"the AI is never invoked on the degenerate path"`

**Edge Cases**:
- All files fell under exclusion.
- Whitespace-only diff.
- No notable source change.
- AI never invoked.

**Context**:
> Degenerate release: "If the post-`diff_exclude` diff is empty or whitespace-only (a re-tag with no source change; a release where every changed file fell under `diff_exclude`; or pure churn with nothing notable), mint does not call the AI — an empty diff is the one input it will reliably hallucinate on. It writes a minimal, honest entry: the version header + a short stub line (e.g. 'Maintenance release — no notable source changes'). No hallucination, no hard error, no skipped entry — a no-op release still produces a truthful record. One coherent family of 'don't run the AI on a bad-sized diff' guards: too-big → fallback/abort per `on_notes_failure`; too-small/empty → stub, no AI." Notes-path precedence (2): "Degenerate diff (empty / all-excluded post-`diff_exclude`) → stub entry, no AI." In Phase 2 the only exclusion is `CHANGELOG.md`.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 4 → Degenerate release, Notes-path precedence".

## mint-release-tool-2-9 | approved

### Task mint-release-tool-2-9: --no-ai fallback path

**Problem**: A user may deliberately skip the AI with `--no-ai`. This is a *deliberate skip, not a failure*, so it must **always use the fallback body and never abort** — even when the AI would have failed. Without this path, there is no way to cut a release without the AI, and the tag must never be empty.

**Solution**: A `--no-ai` path that bypasses the AI entirely and produces the fallback body — the commit-subject list since the last tag by default, or a fixed configurable string — and never aborts. It reuses the same fallback-body builder as `on_notes_failure = fallback` (task 2-7).

**Solution note**: `--no-ai` is precedence step 3 (below first-release and degenerate, above the normal AI path); the precedence wiring is task 2-10. Because it never calls the AI, `on_notes_failure` does not govern it (it cannot fail in the AI sense) and the review gate omits `r` (task 2-14).

**Outcome**: With `--no-ai`, mint produces the fallback body (commit-subject list since the last tag, or the fixed configurable string) with no `claude` invocation, and the release proceeds — it **never aborts**, even in conditions where the normal AI path would have failed. The tag is never empty.

**Do**:
- In `internal/notes`, implement the `--no-ai` body provider: return the **commit-subject list** since the last tag (`git log --format=%s {last_tag}..HEAD` via the `CommandRunner`), or a **fixed configurable string** when one is set (same configurable fallback string as `on_notes_failure`'s fixed-string option).
- **Reuse** the fallback-body builder shared with task 2-7 — both `--no-ai` and `on_notes_failure = fallback` produce the same kind of non-AI body.
- **Never abort:** the `--no-ai` path has no failure mode that aborts — it does not call the AI, so `on_notes_failure` is irrelevant to it. If the last-tag log is empty (no commits since the tag) the path still yields a non-empty body; ensure the tag is never empty (fall back to the fixed string or a minimal record if the subject list is empty — coordinate with the degenerate stub, which precedence already places above `--no-ai`).
- Wire the `--no-ai` CLI flag to select this path (the flag exists in the CLI surface; behaviour lands here, full end-to-end wiring in task 2-16 / precedence in task 2-10).
- This path offers only `y`/`n`/`e` at the gate (no `r`) — gate variant is task 2-14.

**Acceptance Criteria**:
- [ ] `--no-ai` produces the commit-subject list body with no AI invocation.
- [ ] A fixed configurable fallback string is used instead when set.
- [ ] The `--no-ai` path never aborts — even when the normal AI path would have failed.
- [ ] The tag body is never empty on the `--no-ai` path.
- [ ] The fallback-body builder is shared with `on_notes_failure = fallback` (task 2-7).
- [ ] `on_notes_failure` does not govern the `--no-ai` path.

**Tests**:
- `"--no-ai produces the commit-subject list body with no AI call"`
- `"--no-ai uses a fixed configurable fallback string when set"`
- `"--no-ai never aborts even when the AI would have failed"`
- `"--no-ai yields a non-empty body when there are no commits since the last tag"`
- `"--no-ai shares the fallback-body builder with on_notes_failure"`

**Edge Cases**:
- Never aborts even when AI would fail.
- Commit-subject list body.
- Fixed-string fallback config.

**Context**:
> Failure behaviour: "`--no-ai` is a deliberate skip, not a failure → always uses the fallback body, never aborts." Notes-path precedence (3): "`--no-ai` (deliberate skip) → fallback body (commit-subject list), never aborts." Optionality stack: "AI notes — Optional — `--no-ai` / no AI → tag body falls back to a commit-subject / changed-files list, so the tag is never empty." Fallback body: "defaults to the commit-subject list since the last tag; can be a fixed configurable string" — shared with `on_notes_failure = fallback`.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 4 → Failure behaviour, Notes-path precedence", "Body Distribution → Optionality stack".

## mint-release-tool-2-10 | approved

### Task mint-release-tool-2-10: Notes-path precedence resolution

**Problem**: Multiple notes guards can apply to a single run, and which body mint uses depends on a strict precedence. mint must resolve, in order: (1) first-release fixed body, (2) degenerate stub, (3) `--no-ai` fallback, (4) normal AI path. Getting precedence wrong means, e.g., `--no-ai` triggering the AI, or a first-release run being routed through `on_notes_failure`. This resolver is the single decision point that selects which of the four paths produces the body.

**Solution**: A precedence resolver that, given the run state (prior tag present?, `--no-ai`?, post-exclusion diff) and config, selects exactly one notes path in the fixed order: first-release > degenerate > `--no-ai` > normal AI. It routes each run to the right body provider (tasks 1-9, 2-8, 2-9, 2-6) and ensures `on_notes_failure` (task 2-7) governs only the normal AI path.

**Outcome**: A first-release run (no prior tag) always uses `"Initial release."` and wins over `--no-ai` and a degenerate diff — no AI, no `on_notes_failure`. A degenerate diff wins over `--no-ai` (stub, no AI). `--no-ai` wins over the normal AI path (fallback body, no AI, never aborts). Only when none of the first three apply does the normal AI path run, with its failures routed through `on_notes_failure`.

**Do**:
- In `internal/notes`, implement `SelectBody(ctx, state, cfg) (body string, path Kind, err error)` resolving precedence in strict order:
  1. **First release** (no prior tag) → fixed body `"Initial release."` (task 1-9), no AI. Wins over everything below — there is no diff base, so `--no-ai`, `on_notes_failure`, and the degenerate check do not apply.
  2. **Degenerate diff** (empty / all-excluded post-exclusion diff — task 2-8) → stub, no AI.
  3. **`--no-ai`** (deliberate skip — task 2-9) → fallback body, never aborts.
  4. **Normal AI path** (task 2-6) → generate + validate; on failure route to `on_notes_failure` (task 2-7).
- Crucially, evaluate first-release **before** consulting `--no-ai` or the degenerate check, and the degenerate check **before** `--no-ai`. (For first-release, the degenerate check must not even run — there is no diff base.)
- Ensure `on_notes_failure` is consulted **only** in branch 4. Branches 1–3 never call the AI and never route through `on_notes_failure`.
- Return which path was taken (a `Kind`) so the review gate (task 2-14) knows whether to offer `r` (only on the normal AI path) and the wiring (task 2-16) can report it.
- This is the single decision point invoked by the end-to-end wiring (task 2-16); it composes the existing body providers rather than reimplementing them.

**Acceptance Criteria**:
- [ ] First-release (no prior tag) selects `"Initial release."` and wins over `--no-ai` and a degenerate diff.
- [ ] First-release never runs the degenerate check or `on_notes_failure` (no diff base).
- [ ] A degenerate diff wins over `--no-ai` (stub, no AI).
- [ ] `--no-ai` wins over the normal AI path (fallback, no AI, never aborts).
- [ ] The normal AI path runs only when none of the first three apply.
- [ ] `on_notes_failure` governs only the normal AI path (branches 1–3 never reach it).
- [ ] The resolver reports which path was taken (for gate `r`-omission and reporting).

**Tests**:
- `"first-release wins over --no-ai and a degenerate diff"`
- `"first-release does not run the degenerate check or on_notes_failure"`
- `"a degenerate diff wins over --no-ai"`
- `"--no-ai wins over the normal AI path"`
- `"the normal AI path runs only when no earlier guard applies"`
- `"on_notes_failure governs only the normal AI path"`
- `"the resolver reports the selected path kind"`

**Edge Cases**:
- First-release wins over `--no-ai` and degenerate.
- Degenerate wins over `--no-ai`.
- `on_notes_failure` only governs the normal path.

**Context**:
> Notes-path precedence: "When multiple guards could apply to one run, mint resolves them in this order: 1. First release (no prior tag) → fixed body 'Initial release.', no AI. Wins over everything below — there is no diff base, so `--no-ai`, `on_notes_failure`, and the degenerate check don't apply. 2. Degenerate diff (empty / all-excluded post-`diff_exclude`) → stub entry, no AI. 3. `--no-ai` (deliberate skip) → fallback body (commit-subject list), never aborts. 4. Normal AI path → generate + validate; failures route to `on_notes_failure`. `on_notes_failure` governs only the normal AI path — steps 1–3 never call the AI, so they can't trigger it."

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 4 → Notes-path precedence".

## mint-release-tool-2-11 | approved

### Task mint-release-tool-2-11: Single-body distribution to all sinks

**Problem**: The single notes body feeds three surfaces — the annotated tag, `CHANGELOG.md`, and the provider release — but mint **writes** all three and **reads** only one (the tag annotation). The body must be used **whole** for every sink with no parsing or per-sink reassembly, and the optional sinks must honour their toggles: `changelog = false` skips the CHANGELOG (the tag still carries the full body), `publish = false` skips the provider. Without this distribution step the generated body has nowhere to go and the optionality stack isn't enforced.

**Solution**: A distribution step that takes the selected body (whatever path produced it — task 2-10) and threads it whole into the existing sinks: the annotated tag (Phase 1's tag writer — the sole read source), the CHANGELOG projection (Phase 1's writer, skipped when `changelog = false`), and the provider release (Phase 1's `Publisher.CreateRelease`, skipped when `publish = false`). The identical body string reaches every active sink.

**Solution note**: The annotated tag (task 1-10), the changelog writer (task 1-9), and the GitHub `Publisher` (task 1-8) already exist from Phase 1. This task **generalises the body** from the fixed `"Initial release."` to any Phase 2 body and enforces the toggles — it does not rebuild the sinks.

**Outcome**: The same body string is written verbatim to the annotated tag (subject `{commit_prefix} Release {tag}` + the full body), the CHANGELOG `## [x.y.z] - date` section, and the provider release. With `changelog = false`, the CHANGELOG is skipped but the tag still carries the full body (nothing durable lost). With `publish = false`, no provider release is created. The body is identical across all active sinks — no parsing, no splitting, no reassembly.

**Do**:
- In the release orchestration (extending Phase 1's record/tag/publish wiring), thread the selected body (from task 2-10) into the three sinks instead of the hardcoded first-release body:
  - **Annotated tag** (task 1-10): subject `{commit_prefix} Release {tag}` + the full body. This is the **single source mint ever reads**.
  - **CHANGELOG.md** (task 1-9 writer): the full body under the `## [x.y.z] - YYYY-MM-DD` header. **Skip entirely when `changelog = false`.**
  - **Provider release** (task 1-8 `CreateRelease`): the full body. **Skip when `publish = false`.**
- Read the `changelog` (default `true`) and `publish` (default `true`) toggles from `[release]` config (Phase 1 already loads `publish`; add the `changelog` toggle read here with default `true`). Full schema validation is Phase 6.
- Use the body **whole** for every sink — no parsing, no splitting, no label extraction, no per-sink reassembly. Assert the identical string reaches each active sink.
- With `changelog = false`, ensure nothing durable is lost: the tag still holds the full body (the floor / source of truth).
- Provider auto-detection and the unknown-provider/no-driver downgrade are Phase 4 — assume the Phase 1 GitHub `Publisher` when `publish = true`.

**Acceptance Criteria**:
- [ ] The same body string is written to the annotated tag, the CHANGELOG section, and the provider release.
- [ ] The body is used whole — no parsing/splitting/reassembly anywhere.
- [ ] `changelog = false` skips the CHANGELOG projection; the tag still carries the full body.
- [ ] `publish = false` skips the provider release.
- [ ] The annotated tag is always written and always carries the body (mandatory floor / sole read source).
- [ ] The toggles are read from config with defaults (`changelog`/`publish` default `true`).

**Tests**:
- `"the identical body is written to tag, changelog, and provider release"`
- `"the body is used whole with no parsing or per-sink reassembly"`
- `"changelog = false skips the CHANGELOG but the tag still carries the body"`
- `"publish = false skips the provider release"`
- `"the annotated tag is always written with the full body"`

**Edge Cases**:
- `changelog = false` → skip CHANGELOG (tag still carries body).
- `publish = false` → skip provider.
- Identical body across sinks.

**Context**:
> Body Distribution: "The single notes body feeds three surfaces. mint writes all three but reads only one — the tag annotation. Tag annotation = subject `{commit_prefix} Release {tag}` + the FULL notes body … This is the single source mint ever reads. CHANGELOG.md = a write-only projection of the full body … Provider release (GitHub today) = a write-only projection of the full body." Optionality stack: "Annotated tag — Mandatory — always created, always carries a body — the floor and source of truth. Provider release — Optional — `publish` (default `true`). CHANGELOG.md — Optional — `changelog` (default `true`)." "With `changelog = false` nothing durable is lost — the tag still holds the full notes." Output: "mint uses the body whole for every sink; no parsing, no splitting, no per-sink reassembly." Provider auto-detection/downgrade is Phase 4.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Body Distribution: Tag vs Changelog vs Provider Release".

## mint-release-tool-2-12 | approved

### Task mint-release-tool-2-12: Interactive review gate semantics (y/n/e)

**Problem**: The biggest live pain with the legacy script is that release notes go out *unseen*. Notes generate before any mutation, so there is a zero-risk window to review them. mint must drive the interactive review gate's **semantics** — `y` accept (bare Enter), `n` abort, `e` edit verbatim — through the Phase 1 Presenter decision seam, before any mutation, with `-y` skipping the gate entirely. (`r` regenerate is task 2-14; abort auto-unwind is task 2-15; editor resolution is task 2-13.) Without this the generated/reviewed-body promise isn't delivered.

**Solution**: A review-gate driver that, after the plan + notes are shown via the Presenter, requests the user's choice through the Phase 1 Presenter decision seam and maps the three core semantic choices: `y` (accept — the default, a bare Enter) proceeds to Record; `n` (abort) stops the run pre-mutation; `e` (edit) replaces the body with the user-edited text used **verbatim** (no re-validate). `-y` bypasses the gate using the notes as generated. The gate runs **before any mutation**.

**Solution note**: This task owns the gate's `y`/`n`/`e` semantics and the `-y` bypass, driving the Phase 1 Presenter decision seam. Gate **rendering** (the default-yes `Continue?` prompt, menu layout, line-read input) is the CLI Presentation spec's concern — do NOT implement rendering. `r` (task 2-14), editor resolution (task 2-13), and the full abort auto-unwind (task 2-15) are separate tasks; here `n` surfaces the abort intent.

**Outcome**: After the plan + notes are shown, the gate requests a choice via the Presenter seam. A bare Enter / `y` accepts and proceeds to Record. `n` aborts before any mutation (no tag/commit). `e` swaps in the user-edited body, used verbatim with no re-validation, then proceeds. `-y` skips the gate entirely and uses the notes as generated. The gate always runs before the first mutation.

**Do**:
- In the release orchestration, after showing the plan + the generated/selected body via the Presenter (task 1-7), invoke the **decision seam** on the Presenter to obtain the review choice. The engine owns the *semantics*; the Presenter owns the *rendering* (cross-spec boundary — documented on the Phase 1 interface).
- Map the three core choices:
  - **`y` accept** (default; bare Enter): proceed to Record → tag → push (Phase 1 spine).
  - **`n` abort**: stop before any mutation. Surface the abort; the full auto-unwind (when a `pre_tag` hook-artifact commit already exists) is task 2-15 — in Phase 2's hook-free forward path, abort = surface and stop with nothing mutated. Wire `n` to the abort path here.
  - **`e` edit**: obtain the edited text (editor launch/resolution is task 2-13) and **use it verbatim** — no re-parse, no validation. A human edit is trusted; structural validation only ever applies to untrusted AI output. Replace the body, then proceed.
- **`-y` / `--yes`**: skip the whole gate, using the notes as generated. No decision is requested.
- Ensure the gate is positioned **before any mutation** (before Record). Notes generate before the point of no return, so review is zero-risk.
- Test via the `RecordingPresenter` scripting each choice and asserting: accept proceeds, abort stops pre-mutation, edit uses verbatim text, `-y` requests no decision.

**Acceptance Criteria**:
- [ ] A bare Enter / `y` accepts and proceeds to Record.
- [ ] `n` aborts before any mutation (no tag, no commit).
- [ ] `e` replaces the body with the user-edited text used **verbatim** — no re-parse, no validation — then proceeds.
- [ ] `-y` / `--yes` skips the gate entirely and uses the notes as generated (no decision requested).
- [ ] The gate runs before any mutation (before Record).
- [ ] The gate drives the Phase 1 Presenter decision seam; rendering is not implemented here.

**Tests**:
- `"a bare Enter accepts and proceeds to Record"`
- `"y accepts and proceeds"`
- `"n aborts before any mutation"`
- `"e uses the saved text verbatim with no re-validation"`
- `"-y skips the gate and uses the notes as generated"`
- `"the gate runs before any mutation"`

**Edge Cases**:
- Bare Enter → accept.
- `e` saved text verbatim (no re-validate).
- `-y` skips entirely.
- Gate before any mutation.

**Context**:
> Interactive review: "Notes are generated at Stage 4 (before any mutation / the point of no return), so there is a natural zero-risk window to review them. Gate (Continue?): [y] accept (default — Enter) [n] abort [e] edit [r] regenerate with context. `y` accept (default; a bare Enter accepts) → proceed to Record → tag → push. `n` abort → full auto-unwind … `e` edit → opens the notes in the user's editor for real manual editing. The saved text is used verbatim — no re-parse, no validation. A human edit is trusted; structural validation only ever applied to untrusted AI output." Non-interactive: "`-y` / `--yes` skips the whole gate (uses notes as generated) for scripted/CI use." "Exact gate rendering … is owned by the CLI Presentation specification … this section owns the four semantic choices and their effects." `r` is task 2-14; editor resolution is task 2-13; abort auto-unwind is task 2-15.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Interactive Confirmation & Notes Review".

## mint-release-tool-2-13 | approved

### Task mint-release-tool-2-13: Editor resolution for `e`

**Problem**: The `e` edit choice opens the notes in the user's editor for real manual editing. mint must resolve which editor to launch — `$VISUAL`, then `$EDITOR`, then a sensible default (`vi`) — and, critically, if no editor can be launched, **report the problem and return to the gate** rather than crashing. Without robust resolution, `e` is fragile and can strand or crash a release at the review step.

**Solution**: An editor-resolution + launch helper that resolves the editor via `$VISUAL` → `$EDITOR` → `vi`, writes the current body to a temp file, launches the resolved editor on it, and returns the saved text. If no editor can be launched, it reports the problem and signals "return to the gate" so the caller re-presents the gate instead of crashing.

**Solution note**: This task owns editor resolution and launch for the `e` choice; the gate semantics that invoke it (and use the result verbatim) are task 2-12. The launched editor runs through the `CommandRunner` so it can be faked in tests.

**Outcome**: With `$VISUAL` set, that editor is launched; else `$EDITOR`; else `vi`. The current body is presented for editing and the saved text is returned (used verbatim by task 2-12). If none of the resolved editors can be launched, mint reports the problem and returns to the gate (the run does not crash and nothing is mutated).

**Do**:
- Implement `ResolveEditor() string`: return `$VISUAL` if set and non-empty; else `$EDITOR` if set and non-empty; else the default `vi`.
- Implement an edit launch helper: write the current body to a temp file, launch the resolved editor on that file via the `CommandRunner` (interactive — the editor takes over the terminal), then read the saved file back as the new body. The saved text is returned to the caller (task 2-12) and used **verbatim** (no validation here either).
- **No launchable editor:** if launching the resolved editor fails (e.g. command-not-found for the resolved binary — reuse the task 1-1 command-not-found signal), do NOT crash: report the problem via the Presenter and signal the caller to **return to the gate** (re-present the choices). Nothing is mutated.
- Clean up the temp file after reading it back.
- Test editor resolution by setting/unsetting `$VISUAL`/`$EDITOR` and asserting which binary the `FakeRunner` was asked to launch; test the no-launchable-editor path returns the "back to gate" signal without crashing.

**Acceptance Criteria**:
- [ ] `$VISUAL` (set) is preferred for the editor.
- [ ] With `$VISUAL` unset and `$EDITOR` set, `$EDITOR` is used.
- [ ] With neither set, `vi` is the default.
- [ ] The current body is written to a temp file, the editor is launched on it, and the saved text is returned.
- [ ] If no resolved editor can be launched, mint reports the problem and returns to the gate (no crash, no mutation).
- [ ] The temp file is cleaned up.
- [ ] The editor launch goes through the `CommandRunner`/`FakeRunner`.

**Tests**:
- `"it prefers $VISUAL when set"`
- `"it uses $EDITOR when only $EDITOR is set"`
- `"it falls back to vi when neither is set"`
- `"it returns the saved text from the temp file"`
- `"no launchable editor reports the problem and returns to the gate without crashing"`

**Edge Cases**:
- `$VISUAL` set.
- Only `$EDITOR` set.
- Neither → `vi`.
- No launchable editor → report and return to gate.

**Context**:
> Interactive review: "`e` edit → opens the notes in the user's editor for real manual editing. The saved text is used verbatim — no re-parse, no validation … Editor resolution: `$VISUAL` then `$EDITOR`, falling back to a sensible default (`vi`); if no editor can be launched, mint reports the problem and returns to the gate rather than crashing." The verbatim use of the result is task 2-12.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Interactive Confirmation & Notes Review (`e` edit)".

## mint-release-tool-2-14 | approved

### Task mint-release-tool-2-14: `r` regenerate-with-context (loop) & no-AI gate variant

**Problem**: The `r` choice is the "nudge it just this once" affordance: mint asks for a one-time context line, appends it to the prompt, re-runs the AI, and shows the result again, looping until the user is happy — without permanently editing `[release].context`. But on the no-AI notes paths (first-release fixed body, degenerate stub, `--no-ai` fallback) there is no AI invocation to nudge, so `r` must be **omitted** from the gate. Without this, the AI path lacks its iterative refinement and the no-AI paths would offer a meaningless (or `--no-ai`-contradicting) choice.

**Solution**: Extend the review gate (task 2-12) with the `r` choice on the normal AI path: prompt for a one-time context line, append it to the prompt, re-run the normal AI path (task 2-6), re-show the result, and loop (re-presenting the gate) until the user picks `y`/`n`/`e`. On the no-AI paths (identified by the precedence `Kind` from task 2-10), the gate offers only `y`/`n`/`e` — `r` is omitted. The one-time context line is **not** persisted to config.

**Solution note**: This is the in-gate one-time-context nudge ONLY — not the standalone `regenerate`/`--all` command (that is Phase 5). The context line lives for this run's loop only and never touches `[release].context`.

**Outcome**: On the normal AI path, `r` asks for a context line, appends it to the prompt, regenerates via the AI, and re-shows the gate; repeated `r` loops keep nudging (each appends its line for that regeneration). The one-time line is never written to `[release].context`. On first-release, degenerate, and `--no-ai` paths, the gate omits `r` (only `y`/`n`/`e`). Choosing `y`/`n`/`e` exits the loop.

**Do**:
- Extend the gate driver (task 2-12) with the **`r` regenerate** choice, available **only when the precedence `Kind` (task 2-10) is the normal AI path**:
  - Ask for a one-time context line (via the Presenter input seam).
  - Append it to the prompt for this regeneration (compose alongside the default/overridden prompt + Change Map + diff — task 2-5/2-6), re-run the normal AI path (task 2-6), and re-show the result.
  - **Loop:** re-present the gate; the user may `r` again (another nudge), or settle on `y`/`n`/`e`. Each `r` regenerates and re-shows.
- The one-time context line is **NOT persisted** to `[release].context` (a transient nudge for this run only). Assert it never mutates config.
- **No-AI gate variant:** when the `Kind` is first-release / degenerate / `--no-ai`, the gate offers only `y`/`n`/`e` — `r` is omitted (no AI to nudge; offering it under `--no-ai` would contradict the flag). `y`/`n` are body-agnostic and `e` edits verbatim regardless of source, so only `r` is affected. To regenerate with the AI on a `--no-ai` run, the user re-runs without `--no-ai` (documented) — `r` never silently overrides the flag.
- Test multiple `r` loops, omission of `r` on each no-AI path, and that the context line does not reach config; drive the `RecordingPresenter` + `FakeRunner` (scripting successive AI bodies per regeneration).

**Acceptance Criteria**:
- [ ] On the normal AI path, `r` asks for a one-time context line, appends it to the prompt, regenerates via the AI, and re-shows the gate.
- [ ] Multiple `r` loops are supported; each regenerates and re-shows.
- [ ] The one-time context line is not persisted to `[release].context`.
- [ ] On first-release, degenerate, and `--no-ai` paths, the gate omits `r` (only `y`/`n`/`e`).
- [ ] `r` never overrides `--no-ai` (the user re-runs without the flag to use the AI).
- [ ] Choosing `y`/`n`/`e` exits the regenerate loop.
- [ ] AI regeneration goes through the existing normal AI path (task 2-6) and the `CommandRunner`/`FakeRunner`.

**Tests**:
- `"r appends a one-time context line and regenerates via the AI"`
- `"multiple r loops each regenerate and re-show the gate"`
- `"the one-time context line is not persisted to config"`
- `"r is omitted on the first-release path"`
- `"r is omitted on the degenerate path"`
- `"r is omitted on the --no-ai path"`
- `"choosing y/n/e exits the regenerate loop"`

**Edge Cases**:
- `r` omitted on first-release/degenerate/`--no-ai` paths.
- Multiple `r` loops.
- Context line not persisted to config.

**Context**:
> Interactive review: "`r` regenerate with context → mint asks for a one-time context line, appends it to the prompt, re-runs the AI, and shows the result again (loops until happy). The 'nudge it just this once' affordance — without permanently editing `[release].context`." "On the no-AI notes paths (first-release fixed body, degenerate stub, or `--no-ai` fallback …), the gate offers only `y` / `n` / `e`: the AI-dependent `r` regenerate is omitted, since there is no AI invocation to nudge (and offering it under `--no-ai` would contradict the flag). To regenerate with the AI on a `--no-ai` run, the user re-runs without `--no-ai` rather than `r` silently overriding it. `y`/`n` are body-agnostic and `e` edits verbatim regardless of source, so only `r` is affected." The standalone `regenerate`/`--all` command is Phase 5 — this is the in-gate nudge only.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Interactive Confirmation & Notes Review (`r` regenerate, no-AI variant)".

## mint-release-tool-2-15 | approved

### Task mint-release-tool-2-15: Abort auto-unwind from the gate (`n`)

**Problem**: Answering `n` at the review gate must roll back **everything mint made this run**, returning the repo to the exact clean starting state — identical to the pre-push failure path. A user-abort and a pre-push git failure are treated identically: nothing mint did this run survives unless the release completes. Without wiring `n` to the unwind, an abort could leave a stray release commit behind.

**Solution**: Wire the gate's `n` choice to the auto-unwind: reset any commit(s) mint created this run (in Phase 2's hook-free forward path this is the release-bookkeeping commit, if Record has run) and return to the exact clean starting state, reusing the Phase 1 spine's reset. The same path serves a user-abort and a pre-push failure.

**Solution note**: Phase 2 keeps `n` to the **gate-abort behaviour** reusing the Phase 1 spine reset. The hardened, surgical, lock-resilient auto-unwind (precise N-commits-+-tag deletion, `--autostash` restore ordering) is **Phase 4** — do not build that here; note it as deferred. In Phase 2's forward path the gate runs **before any mutation** in the common case, so abort frequently has nothing to undo; the wiring must still correctly reset if Record-stage commits exist (and will reset the hook-artifact commit too once hooks land in Phase 3).

**Outcome**: Answering `n` returns the repo to the exact clean state it started from — no release commit, no tag survives. The abort path is identical to the pre-push failure path. Because the gate sits before any mutation in Phase 2's forward path, abort typically has nothing to undo; if any mint commit exists at abort time, it is reset.

**Do**:
- Wire the gate's `n` choice (from task 2-12) to the **auto-unwind / clean-reset** path:
  - Reset any commit(s) mint created this run back to the recorded clean starting point (the HEAD mint captured before mutating). Reuse the Phase 1 spine's reset mechanism.
  - No tag is created until after the gate in the spine, so in the common case there is no tag to delete at gate-abort; if a tag were created this run, it would be removed (the surgical version is Phase 4).
- Treat a **user-abort and a pre-push git failure identically** — route both to the same clean-reset path (one mental model).
- Capture the clean starting state (e.g. the starting HEAD) at run start so the unwind target is unambiguous.
- Report what was undone via the Presenter (e.g. "aborted — repo restored to clean state"; if nothing was mutated, say so).
- Do NOT implement the Phase 4 hardening: surgical precise-count unwind, lock-resilient git, `--autostash` stash-restore ordering. Keep to the gate-abort reset using the Phase 1 spine; note the hardening is deferred to Phase 4.

**Acceptance Criteria**:
- [ ] Answering `n` returns the repo to the exact clean starting state.
- [ ] No release commit and no tag survive an abort.
- [ ] The abort path is identical to the pre-push failure path (same reset).
- [ ] If the gate is reached before any mutation, abort reports nothing to undo (and nothing is changed).
- [ ] If a mint commit exists at abort time, it is reset to the clean starting point.
- [ ] The Phase 4 surgical/lock-resilient/autostash hardening is NOT implemented (deferred).
- [ ] All git operations go through the `CommandRunner`/`FakeRunner`.

**Tests**:
- `"answering n returns the repo to the clean starting state"`
- `"no commit or tag survives an abort"`
- `"the abort path matches the pre-push failure path"`
- `"abort before any mutation reports nothing to undo"`
- `"abort after a Record commit resets that commit"`

**Edge Cases**:
- Unwind back to clean state.
- Identical to pre-push failure path.
- No tag/commit survives.

**Context**:
> Interactive review: "`n` abort → full auto-unwind: identical to the pre-push failure path — mint rolls back everything it made this run, including any `pre_tag` hook-artifact commit, returning to the exact clean starting state. The hook re-runs next time (idempotent build). A user-abort and a pre-push git failure are treated identically." Failure model (before the push): "Everything mint did is local-only. mint auto-unwinds its own mutations — deletes the tag it made, resets the release commit(s) — returning the repo to the exact clean starting state. mint knows precisely what it created (N commits + 1 tag), so the unwind is surgical … Not configurable (YAGNI)." Invariants: "nothing mint did this run survives unless the release completes." The surgical/lock-resilient/autostash-restore hardening is Phase 4; the hook-artifact commit lands in Phase 3 — Phase 2's `n` reuses the Phase 1 spine reset for the gate-abort behaviour.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Interactive Confirmation & Notes Review (`n` abort)", "Stages 6–7 → Failure model", "Release Lifecycle → Invariants".

## mint-release-tool-2-16 | approved

### Task mint-release-tool-2-16: End-to-end prior-tag release wiring

**Problem**: The Phase 2 pieces — context assembly, `max_diff_lines` guard, Change Map, default prompt, transport, the four notes paths + precedence, single-body distribution, and the `y`/`n`/`e`/`r` review gate — exist independently, but nothing threads them into a runnable prior-tag `mint release` that takes a repo *with* a prior tag to a published release whose AI-generated body flows to all three sinks and is gated on review. This vertical seam proves the engine end-to-end and completes the Presenter cross-spec seam by exercising all four review choices.

**Solution**: Extend the Phase 1 release orchestrator so that a release **with a prior tag** runs the full Phase 2 notes flow: select the body via precedence (task 2-10) → show plan + notes → run the `y`/`n`/`e`/`r` gate (tasks 2-12/2-13/2-14, unless `-y`) → on accept, distribute the single body whole to the tag, CHANGELOG, and provider (task 2-11) through the Phase 1 Record → tag → push → publish spine; on abort, auto-unwind to clean (task 2-15).

**Outcome**: In a repo with a prior tag, `mint release` generates the notes body via the appropriate notes path, shows it at the gate, and — on accept (or `-y`) — records it, tags, atomic-pushes, and publishes, with the **identical body in all three sinks**. Accept proceeds to Record; abort (`n`) leaves the repo clean. The `--no-ai`, normal-AI, and degenerate paths all run end-to-end; the PONR asymmetry from Phase 1 (warn-only post-push) is preserved.

**Do**:
- Extend the Phase 1 orchestrator (`cmd/mint` / release orchestration) so the **notes stage** for a prior-tag release uses the Phase 2 precedence resolver (task 2-10) instead of the hardcoded first-release body. First-release (no prior tag) continues to use `"Initial release."` via the same resolver (precedence step 1) — Phase 1's path is now one branch of the unified selector.
- Spine order for a prior-tag release:
  1. Version → preflight (Phase 1) → determine `last_tag` (the current latest, task 1-3).
  2. **Notes:** `SelectBody` (task 2-10) → body + path `Kind` (handles first-release / degenerate / `--no-ai` / normal AI, with `on_notes_failure` routing on the normal path).
  3. Show plan + notes via the Presenter; run the review gate (tasks 2-12/2-13/2-14) unless `-y`. `r` available only on the normal AI path.
  4. On **accept** (or `-y`): Record (changelog + bookkeeping commit) → gh gate (if publishing, before tag) → annotated tag + `git push --atomic` → publish — distributing the single body whole to all sinks (task 2-11).
  5. On **abort** (`n`): auto-unwind to clean (task 2-15).
- Preserve the PONR asymmetry: pre-push failures abort cleanly; a publish failure after a successful push warns only (Phase 1 behaviour, unchanged).
- Wire `--no-ai` (selects the fallback path via precedence) into the release command flags' behaviour for this end-to-end path. (`-d/--dry-run`, `--set-version`, `--autostash`, `--any-branch` remain later phases — do not implement their behaviour.)
- Test end-to-end with `FakeRunner` + `RecordingPresenter`: assert the generated body flows identically to all three sinks; gate-accept proceeds to Record; gate-abort leaves the repo clean; exercise the normal-AI, `--no-ai`, and degenerate paths.

**Acceptance Criteria**:
- [ ] A prior-tag `mint release` generates the body via the precedence resolver and gates on review.
- [ ] On accept (or `-y`), the identical body is recorded, tagged, atomic-pushed, and published — same body in all three sinks.
- [ ] Gate accept proceeds to Record; gate abort (`n`) leaves the repo clean.
- [ ] The normal-AI, `--no-ai`, and degenerate paths each run end-to-end.
- [ ] `last_tag..HEAD` is the diff base for the AI path; first-release still resolves to `"Initial release."` via the same selector.
- [ ] The PONR asymmetry is preserved (pre-push abort; post-push publish failure warns only).
- [ ] The whole flow is exercised with `FakeRunner` + `RecordingPresenter`; no real git/gh/claude in tests.

**Tests**:
- `"a prior-tag release generates a body and flows it identically to all three sinks"`
- `"gate accept proceeds to Record, tag, push, and publish"`
- `"gate abort leaves the repo clean"`
- `"the --no-ai path runs end-to-end with a fallback body"`
- `"a degenerate diff runs end-to-end with the stub body and no AI call"`
- `"a publish failure after a successful push warns only (post-PONR)"`

**Edge Cases**:
- Generated body flows to all three sinks.
- Gate accept proceeds to record.
- Gate abort leaves repo clean.

**Context**:
> Phase 2 goal: "Releases with a prior tag generate a notes body from the `last_tag..HEAD` diff via the layered AI engine … prepend a computed Change Map, distribute the single body whole to the tag annotation, CHANGELOG.md, and provider release, and gate on the interactive `y`/`n`/`e`/`r` notes review." Stage 4: "generate once, use everywhere." Release Lifecycle spine: Version → Preflight → (Project prep) → Release notes → Record → Make official (tag + `git push --atomic`) → Publish. Invariants & Failure model (Phase 1, preserved): pre-push local-only and recoverable; `git push --atomic` is the single PONR; post-PONR publish failure warns only. Notes-path precedence unifies first-release / degenerate / `--no-ai` / normal AI under one selector. `--dry-run`/`--set-version`/`--autostash`/`--any-branch` are later phases.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Release Lifecycle (the spine)", "Stage 4 — AI Release Notes", "Body Distribution", "Interactive Confirmation & Notes Review", "Stages 6–7 → Failure model".
