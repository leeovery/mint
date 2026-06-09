---
phase: 1
phase_name: Walking Skeleton — Bare Commit, End-to-End
total: 6
---

## commit-command-1-1 | approved

### Task commit-command-1-1: Read [commit] config table

**Problem**: `mint commit` needs two commit-specific config knobs — `[commit].context` (inject project guidance into the commit prompt) and `[commit].prompt` (a full prompt-override file path) — read from the verb-namespaced `.mint.toml`. Both are optional, typed, and must fail loud on bad input, consistent with the existing config model. Nothing reads the `[commit]` table yet, so prompt assembly (commit-command-1-2) has no values to consume.

**Solution**: Extend the existing config loader (`internal/config`, established by the release plan's minimal loader and consumed verb-namespaced shape) to decode a `[commit]` table into a `Commit` sub-struct holding `Context string` and `Prompt string`. Both keys are optional (absent = empty, meaning "no injection" / "use the default prompt"). A wrong-typed value (e.g. an array where a string is expected) fails loud. When `[commit].prompt` names a file, resolving/reading that file is deferred to assembly (commit-command-1-2) — but a configured-but-unreadable/missing override path must surface as a loud failure when it is used, not silently fall through to the default.

**Solution note**: This task adds only the `[commit]` table read onto the already-restructured verb-namespaced config (a consumed dependency — the shared engine keys `ai_command`/`diff_exclude`/`max_diff_lines` at top level and the per-verb tables already exist). Do NOT re-implement the full schema, the shared engine keys, or `[release]`. Do NOT add a scope toggle or a per-verb `ai_command`/`max_diff_lines` override — the spec explicitly excludes those.

**Outcome**: Given no `.mint.toml` or an absent `[commit]` table, `Context` and `Prompt` are both empty (the bare-commit default: no context injection, default Conventional Commits prompt). Given `[commit].context = "…"`, the string is returned. Given `[commit].prompt = ".mint/commit-prompt.md"`, the path is returned for assembly to read. A non-string value for either key fails loud with a clear typed error. A configured `prompt` path that is missing or unreadable fails loud at the point of use (assembly), naming the path — it never silently degrades to the default prompt.

**Do**:
- In `internal/config`, add a `Commit` sub-struct (e.g. `type Commit struct { Context string; Prompt string }`) and decode the `[commit]` table into it during the existing `Load(root)`.
- Treat both keys as optional: an absent `[commit]` table or absent key yields the empty string. An empty string for `context` means "no injection"; an empty string for `prompt` means "use the default Conventional Commits prompt".
- Typed/fail-loud: a value of the wrong type (e.g. `context = ["a","b"]` or `prompt = 3`) must surface a clear decode error naming the offending key — consistent with the existing fail-loud config posture. (Reuse whatever typed-decode/fail-loud mechanism the consumed config model provides; do not invent a parallel validation path.)
- Provide a small accessor the assembler can call to resolve the override file: e.g. `ResolveCommitPrompt(cfg, root) (string, error)` that, when `cfg.Commit.Prompt != ""`, reads the file at that path (relative to the repo root) and returns its contents; when the path is missing/unreadable it returns a loud error naming the path. When `cfg.Commit.Prompt == ""` it returns an empty string and no error (assembler then uses the default prompt). Keeping the read here keeps the "unreadable override → fail loud" rule in one place; assembly (1-2) consumes the result.
- Do NOT read or default any push-related key — push is flag-only with no config default (spec: "Deliberately NOT added for commit").

**Acceptance Criteria**:
- [ ] Absent `.mint.toml` or absent `[commit]` table → `Context == ""` and `Prompt == ""` (no error).
- [ ] `[commit].context = "…"` is read and returned verbatim.
- [ ] `[commit].prompt = "<path>"` is read and returned as a path for assembly to resolve.
- [ ] A wrong-typed `context` or `prompt` value fails loud with a clear error naming the key.
- [ ] A configured `prompt` path that is missing/unreadable fails loud naming the path (never silently uses the default).
- [ ] An empty `context` (no injection) and an empty `prompt` (default prompt) are both valid, non-error states.
- [ ] No push/scope/per-verb-engine-override keys are introduced for `[commit]`.

**Tests**:
- `"it returns empty context and prompt when the [commit] table is absent"`
- `"it reads [commit].context as a string"`
- `"it reads [commit].prompt as a path"`
- `"a non-string context value fails loud naming the key"`
- `"a non-string prompt value fails loud naming the key"`
- `"a configured prompt-override path that is missing fails loud naming the path"`
- `"a configured prompt-override path that is unreadable fails loud naming the path"`
- `"an empty context and empty prompt are valid (no injection / default prompt)"`

**Edge Cases**:
- Key absent (context and prompt both optional) → empty, default behaviour, no error.
- Wrong type for either key → fail loud.
- Prompt-override path unreadable/missing → fail loud at point of use, naming the path (no silent fall-through to default).

**Context**:
> Config Schema: "With mint now multi-verb, the config shape is verb-namespaced tables + shared engine keys." `[commit]` example: `context = "Conventional Commits; dev-workflow toolkit."` ("inject into the commit prompt") and `prompt = ".mint/commit-prompt.md"` ("full prompt override"). "Commit-specific keys: `[commit].context` (context-inject knob) and `[commit].prompt` (full prompt override). Both optional, typed, fail-loud — consistent with the existing config model." "Deliberately NOT added for commit: No push config — push is flag-only `-p`, never a default … No scope toggle, no per-verb `ai_command`/`max_diff_lines` override." The verb-namespaced restructure itself is a consumed dependency (the release spec's to absorb); this task only adds the `[commit]` table read on top of it.

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Config Schema → Commit's config surface", "Deliberately NOT added for commit".

## commit-command-1-2 | approved

### Task commit-command-1-2: Assemble Conventional Commits prompt (L3)

**Problem**: The shared AI engine's content-agnostic transport (L2) needs a finished prompt. Commit's L3 glue owns prompt assembly — and commit's default is a *different* default from release's: Conventional Commits 1.0.0 (`type(scope): description`), with the AI inferring `type`, scope off by default, no `commit_prefix`/🌿 branding, and the two-knob context-inject + full-override model. Without commit's own prompt assembly there is nothing meaningful to send to L2 for a commit message.

**Solution**: An L3 prompt composer for commit (in commit's package, e.g. `internal/commit`) that builds the full AI input: mint's default Conventional Commits prompt instructions + the staged diff content. It injects `[commit].context` into the default prompt when set, and fully replaces the default prompt with the `[commit].prompt` override file contents when set (mint still supplies the diff). The default prompt instructs the AI to infer `type`, omit scope, write an imperative concise subject (optional blank line + wrapped body for the *why*), and emit a plain conventional-commit message with no mint branding.

**Solution note**: This task owns commit's prompt *composition* and default prompt text only. The actual AI call is L2's transport (consumed: `internal/ai` `Transport.Generate(ctx, prompt)`). Diff assembly + `diff_exclude`/`max_diff_lines` is L1 (commit-command-1-3). This mirrors release's `ComposePrompt` shape (consumed pattern) but with the Conventional Commits default and the `[commit]` knobs. The exact prompt wording is ship-and-refine — pick sensible defaults, keep tunable.

**Outcome**: The default prompt instructs the AI to emit a Conventional Commits message: a `type: description` subject line (imperative, concise) with the **`type` inferred from the diff** (feat/fix/chore/docs/…), **scope omitted by default**, an optional blank line + wrapped body for the *why*, and **no `commit_prefix`/🌿 and no other mint branding** anywhere in the message text. A configured `[commit].context` is injected into the default prompt without replacing it; a configured `[commit].prompt` file fully overrides the default prompt while mint still supplies the diff. The composed input is exactly the prompt + the diff content — nothing else.

**Do**:
- In commit's L3 package (e.g. `internal/commit`), implement `ComposePrompt(diff string, cfg) (string, error)` assembling: prompt instructions + the staged diff content (the L1 output from 1-3), and nothing else.
- Author the **default Conventional Commits prompt** text encoding:
  - **Subject line** `type: description` — imperative mood, concise; follow Conventional Commits 1.0.0 (conventionalcommits.org).
  - **AI infers `type`** (feat/fix/chore/docs/refactor/test/…) from the diff — central to the format and reliably inferable.
  - **Scope omitted by default** — do not emit `(scope)`; the instruction explicitly tells the AI not to guess a scope (project-specific, inconsistently guessed; better omitted than wrong).
  - **Optional body** — a blank line then a wrapped body explaining the *why*, when warranted.
  - **No mint branding** — the message must be plain `type: description …`; explicitly forbid any 🌿/`commit_prefix` or preamble/meta-commentary. `commit_prefix` is a release-only concern and must never appear in a commit message.
- Implement the two prompt knobs (consumed model, mirroring release):
  - **`[commit].context`** (string) — inject the value into the default prompt (does not replace it). The common case.
  - **`[commit].prompt`** (file path) — full override: replace the default prompt text with the override file contents (resolved via the 1-1 accessor `ResolveCommitPrompt`); mint still appends the diff. A configured-but-unreadable override fails loud (per 1-1) — never silently fall back to the default.
- The AI returns the message directly (no machine-parseable wrapper) — this task produces only the *input*; no output parsing.
- Do NOT assemble the diff, apply `diff_exclude`/`max_diff_lines`, or call the transport here — those are 1-3 (L1) and the consumed L2.

**Acceptance Criteria**:
- [ ] The composed input is: prompt + staged diff content, in that order, and nothing else.
- [ ] The default prompt instructs a Conventional Commits `type: description` subject (imperative, concise) with an optional wrapped body for the *why*.
- [ ] The default prompt instructs the AI to **infer the `type`** from the diff.
- [ ] The default prompt instructs **scope omitted by default** (no `(scope)` guessing).
- [ ] The default prompt forbids any `commit_prefix`/🌿/mint branding and any preamble/meta-commentary in the message text.
- [ ] `[commit].context` (when set) is injected into the default prompt without replacing it; absent context = default prompt unchanged.
- [ ] `[commit].prompt` (when set) fully overrides the default prompt while mint still supplies the diff; an unreadable override fails loud (does not fall back to default).
- [ ] No machine-parseable output wrapper is requested (the AI returns the message directly).

**Tests**:
- `"the composed input is the prompt followed by the staged diff and nothing else"`
- `"the default prompt requests a Conventional Commits type: description subject with optional body"`
- `"the default prompt instructs the AI to infer the type from the diff"`
- `"the default prompt instructs scope omitted by default"`
- `"the default prompt forbids commit_prefix / 🌿 / branding in the message"`
- `"a configured [commit].context is injected into the default prompt without replacing it"`
- `"absent context leaves the default prompt unchanged"`
- `"a configured [commit].prompt file fully overrides the default prompt while the diff is still supplied"`
- `"an unreadable prompt-override fails loud and does not fall back to the default"`

**Edge Cases**:
- Context present injects vs absent (no injection, default prompt).
- Prompt-override fully replaces the default (diff still supplied).
- Scope omitted by default.
- No `commit_prefix` / 🌿 anywhere in the prompt-produced output.

**Context**:
> Commit Message Format & Prompt: "L3 owns this prompt; it is simply a different default from release's. Default format = Conventional Commits 1.0.0 … `type(scope): description` subject line — imperative, concise; optional blank line + wrapped body for the *why*. AI infers the `type` (feat/fix/chore/docs/…) from the diff — central to the format and reliably inferable. Scope off by default — scope conventions are project-specific and the AI guesses them inconsistently; better omitted than wrong. Two-knob override, mirroring release: a commit-specific context-inject knob + a full prompt-override knob." "No mint branding in the message text. Commit does NOT use release's `commit_prefix` (🌿) … forcing a 🌿 onto every commit is undesirable. `commit_prefix` stays a release-only concern." Engine: "L3 owns prompt assembly; L2 receives the finished prompt … mint always owns the prompt; `ai_command` is just transport." Layer 3: "supplies the prompt + default format (release notes vs commit message)."

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Commit Message Format & Prompt", "AI Engine — Three-Layer Split → Prompt boundary, Commit's binding to the engine".

## commit-command-1-3 | approved

### Task commit-command-1-3: Bind staged-diff source through L1/L2 (L3 glue)

**Problem**: Commit's L3 glue must pick the L1 *source* (the staged diff, `git diff --cached`) and thread it through the shared, content-agnostic engine — L1's `diff_exclude` + `max_diff_lines` logic and L2's transport with output validation and one retry — without re-implementing any engine internals. Without this binding there is no commit message body: assembly (1-2) has no diff, and L2 is never invoked for commit.

**Solution**: An L3 generate function (in commit's package) that: (1) obtains the **staged diff** from L1's shared context builder parameterised to the `git diff --cached` source with `diff_exclude` globs applied and the `max_diff_lines` guard applied at L1; (2) composes the Conventional Commits prompt over that diff (1-2); (3) calls L2's consumed transport (`Transport.Generate`), which runs `ai_command`, validates the output, and retries once — consumed, not reimplemented; (4) returns the message body or a typed failure. The bare-commit (Phase 1) source is staged-only; the `-a`/`-A` would-be-staged source is Phase 2.

**Solution note**: L1 (the `diff_exclude` + `max_diff_lines` git-aware context builder) and L2 (the content-agnostic transport with validate + one-retry) are **consumed** from the shared engine — do NOT build or duplicate them. This task is the commit-specific *glue*: select the staged-diff source, wire L1→compose→L2, return the body. Composition into a convenience wrapper is permitted (spec: "Composition is permitted") but the underlying L1/L2 pieces stay shared. The oversized-diff `$EDITOR` fallback branch and `--no-ai` are Phase 3 — here, just surface the engine's outcome (body or typed failure); do not route failures.

**Outcome**: For a repo with a staged diff, the L3 generate returns a validated Conventional Commits message body produced by the shared engine. `diff_exclude` globs remove excluded files (bundles, lockfiles, minified output) **before** generation, so they never reach the prompt. The `max_diff_lines` guard is applied **at L1**, after `diff_exclude`, before any L2 call. L2's one retry on bad output is consumed (not reimplemented in commit code). The body is a single string suitable for the commit sink.

**Do**:
- In commit's L3 package (e.g. `internal/commit`), implement `Generate(ctx, cfg) (string, error)` (or a small composing wrapper) that orchestrates the consumed pieces in order:
  1. Obtain the **staged diff** via the shared L1 context builder, parameterised to commit's source `git diff --cached` (the consumed L1 is the release plan's diff-assembly side, parameterised by *source*; commit passes the staged-diff source). Apply `diff_exclude` globs (consumed L1 logic) and the `max_diff_lines` guard (consumed L1 logic) — identical to release's, only the source differs.
  2. `ComposePrompt(diff, cfg)` (1-2) → the full Conventional Commits AI input.
  3. Call the consumed L2 transport (`internal/ai` `Transport.Generate(ctx, prompt)`) → validated body or a typed engine failure (after L2's one retry / timeout rules). Do NOT re-implement the transport, validation, or the retry.
  4. Return the body **whole** (no parsing/splitting) or surface the typed failure (oversized-skip vs generation-failure distinguishable, so Phase 3 can route each to the editor).
- Parameterise L1 by source rather than copying release's `tag..HEAD` assembly: the shared builder already supports source selection; commit supplies `git diff --cached`. Document that `diff_exclude` and `max_diff_lines` are the genuinely shared git piece (identical logic for both verbs) and are consumed, not re-authored.
- Phase 1 source = **staged-only** (`git diff --cached`). Do NOT implement the `-a`/`-A` would-be-staged read-only diff (Phase 2).
- All git/AI invocation goes through the consumed `CommandRunner` seam; tests script the staged diff and the `ai_command` via the fake runner.

**Acceptance Criteria**:
- [ ] The L1 source is `git diff --cached` (staged-only) for the bare-commit path.
- [ ] `diff_exclude` globs remove excluded files **before** generation (excluded files never reach the prompt).
- [ ] The `max_diff_lines` guard is applied **at L1**, after `diff_exclude`, before any L2 call.
- [ ] L2's transport, output validation, and one retry are **consumed**, not reimplemented in commit code.
- [ ] The returned body is used whole (no parsing/splitting) and suitable for the commit sink.
- [ ] An engine failure surfaces as a distinguishable typed failure (so Phase 3 can route oversized vs generation-failure).
- [ ] No `-a`/`-A` would-be-staged source is implemented (deferred to Phase 2).
- [ ] All git/AI calls go through the consumed `CommandRunner`/fake.

**Tests**:
- `"it obtains the staged diff via git diff --cached as the L1 source"`
- `"diff_exclude removes excluded files before generation (they never reach the prompt)"`
- `"the max_diff_lines guard is applied at L1 after diff_exclude, before any L2 call"`
- `"it returns a validated message body whole for a real staged diff"`
- `"the L2 one-retry is consumed (commit code does not re-run the transport itself)"`
- `"an engine failure surfaces a distinguishable typed failure"`
- `"it does not compute an -a/-A would-be-staged diff (staged-only)"`

**Edge Cases**:
- `diff_exclude` removes excluded files before generation.
- `max_diff_lines` guard applied at L1 (after `diff_exclude`, before L2).
- L2 one-retry consumed, not reimplemented.

**Context**:
> AI Engine — Three-Layer Split: "Layer 1 — Context builder (git-aware). Produces the content to describe. Parameterised by *source*: release uses `tag..HEAD`; commit uses the staged diff (`git diff --cached`). Applies `diff_exclude` globs and the `max_diff_lines` guard — identical logic for both verbs, so this is the genuinely shared git piece. Layer 2 — AI message engine (git-unaware, content-agnostic) … Runs the transport, validates the output (non-empty / not an error / not a refusal), retries once, fails loud per policy, and returns the message body. Layer 3 — Per-verb glue. Picks the L1 source, supplies the prompt + default format … and decides the sinks." "Composition is permitted … a convenience wrapper … that composes L1→L2→sink." Commit's binding: "Layer 1 source: the staged diff (`git diff --cached`) … Layer 3 glue: supplies the Conventional Commits default prompt/format, the `[commit]` context/override knobs, and the commit sinks." Scope reuses: "`diff_exclude` globs and the `max_diff_lines` guard apply to commit's diff exactly as they apply to release's — we don't feed excluded files (bundles, lockfiles, minified output) into message generation." The oversized-diff `$EDITOR` fallback (detection at L1, before any L2 call) is Phase 3.

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "AI Engine — Three-Layer Split", "Commit's binding to the engine", "Scope → What commit reuses".

## commit-command-1-4 | approved

### Task commit-command-1-4: Wire bare `mint commit` generate-and-commit thread

**Problem**: The L3 glue (1-3), the prompt composer (1-2), and the `[commit]` config (1-1) exist as pieces, but nothing threads them into a runnable bare `mint commit` that generates a Conventional Commits message from the staged diff and creates the commit. This is the walking-skeleton vertical seam — the thinnest end-to-end thread proving the L3 binding and the commit sink work together against the real shared engine.

**Solution**: The `mint commit` CLI entry point and a commit orchestrator that, for the bare (staged-only, no flags) path, runs: generate the message via L3 (1-3) → [review gate is 1-5; preflight is 1-6] → create the commit via the consumed `git_safe` with the generated message body as the commit message. The commit message text carries the Conventional Commits message verbatim, with no `commit_prefix`/🌿 branding.

**Solution note**: This task wires the bare generate-and-commit thread and the commit sink. The review gate integration is commit-command-1-5 and the empty-index preflight is commit-command-1-6 — this task may sequence them in but its own deliverable is the generate→commit thread and the `git_safe` commit sink. Do NOT implement `-a`/`-A` staging (Phase 2), `--no-ai`/`$EDITOR` (Phase 3), gate `e`/`r` (Phase 4), or `-p` push (Phase 5). The `git commit` mutation runs through the consumed `git_safe` lock-resilient wrapper, not the raw runner.

**Outcome**: In a repo with a staged diff, bare `mint commit` generates a Conventional Commits message (AI infers `type`, **scope off by default**) and creates a commit whose message is exactly that generated body — via `git_safe` (consumed lock-resilient git). The commit message text carries **no 🌿/`commit_prefix`** and no mint branding. The thread runs staged-only with no flags.

**Do**:
- Add the `mint commit` subcommand to the CLI (the same `cmd/mint` surface the release verb uses — consumed). Phase 1 wires bare `mint commit` only; reserve but do not implement the flag behaviours owned by later phases (`-a`/`-A`, `-p`, `--no-ai`; `-y` is wired in 1-5).
- Implement a commit orchestrator (e.g. `internal/commit`'s `Run`) for the bare path, in order:
  1. Resolve repo root + load config (consumed: `internal/config` + the 1-1 `[commit]` read). [The not-a-git-repo and empty-index preflight is 1-6.]
  2. Generate the message body via L3 `Generate(ctx, cfg)` (1-3) over the staged diff.
  3. [Review gate — 1-5.] On accept (or `-y`):
  4. Create the commit via the consumed **`git_safe`** wrapper: `git commit` with the generated body as the message (e.g. pass the body via `-F -`/stdin or a temp file to preserve the subject + wrapped body verbatim). Staged-only: do NOT run any `git add` (no staging in the bare path).
- The commit message is the generated Conventional Commits body **verbatim** — assert no `commit_prefix`/🌿 is prepended (the release-only branding must never touch a commit message).
- Use the consumed `git_safe` for the `git commit` mutation (lock-resilient git is a consumed dependency); do not call the raw runner for the mutating commit.
- Wire the consumed Presenter to report the plan/message (rendering owned by cli-presentation); tests use the recording presenter + fake runner — no real git/claude.

**Acceptance Criteria**:
- [ ] Bare `mint commit` generates a Conventional Commits message from the staged diff (AI infers `type`).
- [ ] Scope is off by default in the produced message.
- [ ] The commit is created via the consumed `git_safe` wrapper, not the raw runner.
- [ ] The commit message text is the generated body verbatim — no `commit_prefix`/🌿/mint branding.
- [ ] The bare path is staged-only — no `git add` is run.
- [ ] No `-a`/`-A`, `-p`, `--no-ai`, or gate `e`/`r` behaviour is implemented (deferred).
- [ ] The thread is exercised end-to-end with the fake runner + recording presenter (no real git/claude).

**Tests**:
- `"bare mint commit generates a Conventional Commits message from the staged diff"`
- `"the AI-inferred type appears and no scope is emitted by default"`
- `"the commit is created via git_safe"`
- `"the commit message carries no 🌿 / commit_prefix branding"`
- `"the bare path runs no git add (staged-only)"`
- `"the generated body is used as the commit message verbatim"`

**Edge Cases**:
- AI infers `type`.
- Scope off by default.
- Commit created via `git_safe`.
- Message text carries no 🌿 branding.

**Context**:
> Overview: "Its core act is small and local: stage (optionally) → generate a message → optionally review → commit → optionally push." Commit Flow / Lifecycle: "On accept — apply `-a`/`-A` staging now (if given), then `git commit` (via `git_safe`)." Commit Message Format: "AI infers the `type` … Scope off by default." "No mint branding in the message text. Commit does NOT use release's `commit_prefix` (🌿)." Scope: "`git_safe` lock-resilient git" is a reused shared primitive. Commit's binding: "Layer 3 glue: … the commit sinks (`git commit`, optional push)." Phase 1 is bare staged-only: `-a`/`-A` (Phase 2), `--no-ai`/`$EDITOR` (Phase 3), `e`/`r` (Phase 4), `-p` (Phase 5) are out of scope here.

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Overview", "Commit Flow / Lifecycle", "Commit Message Format & Prompt", "CLI Surface & Flags".

## commit-command-1-5 | approved

### Task commit-command-1-5: Integrate the Continue? review gate

**Problem**: Commit's core invariant is **mutate nothing until the user accepts the gate**, and seeing the minted message before it sticks is the point. The `Continue?` review gate (ON by default) must render via the consumed Presenter, accept on `y`/Enter and commit, abort on `n` mutating nothing, skip under `-y` (auto-accept), and fail loud on the non-TTY-without-`-y` forbidden combination. Without the gate the generate-and-commit thread (1-4) commits messages unseen — the exact pain the gate exists to kill.

**Solution**: Wire the consumed Presenter's `Prompt(gate)` seam into the commit orchestrator (1-4): render the generated message + the `Continue?` gate, accept on `y`/Enter → proceed to the `git_safe` commit, abort on `n` → true no-op (nothing mutated, since the bare path stages nothing and the commit hasn't run). The `-y` flag skips the gate (auto-accept). The shared non-TTY-without-`-y` forbidden-combo rule (consumed from the Presenter) fails loud. Phase 1's gate offers the accept/abort path; the `e`/`r` actions are Phase 4.

**Solution note**: The gate *rendering*, the line-read input model, the `-y` auto-accept echo, and the forbidden-combo fail-loud are all **consumed** from the cli-presentation Presenter (`Prompt(gate)`) — do NOT build gate rendering or input parsing here. This task is the commit-side *integration*: declare the gate, place it before the commit mutation, and branch on the returned choice. Phase 1 wires `y`/`n` (Enter ⇒ `y`) and the `-y`/non-TTY posture only; `e` (edit) and `r` (regenerate) gate actions are Phase 4 — the declared choice set for Phase 1 need not offer them.

**Outcome**: An interactive bare `mint commit` shows the generated message + `Continue?`; Enter or `y` accepts and creates the commit (via `git_safe`, 1-4); `n` aborts as a true no-op — nothing is staged, nothing is committed, the repo is exactly as before. `-y` skips the gate entirely (auto-accept) with the consumed auto-accept echo. A non-TTY stdin without `-y` fails loud via the consumed forbidden-combo rule — it never hangs and never commits unseen.

**Do**:
- In the commit orchestrator (1-4), after generating the message body (1-3) and before the `git_safe` commit, call the consumed Presenter `Prompt(gate)` with a commit review gate (Enter ⇒ accept/`y`).
- Branch on the returned choice:
  - **`y` / Enter (accept)** → proceed to the commit (1-4's `git_safe` commit).
  - **`n` (abort)** → return a clean no-op: do nothing. Because the bare path defers/avoids all mutation until accept (no `git add`, no commit yet), abort leaves the repo exactly at its pre-`mint` state — no auto-unwind needed.
- **`-y` / `--yes`** → skip the gate (auto-accept), consuming the Presenter's `-y` skip + auto-accept echo. The gate is not drawn-and-auto-pressed; it is skipped.
- **Forbidden combo** → non-TTY stdin without `-y` fails loud, surfaced through the consumed Presenter forbidden-combo rule (also to stderr). Do NOT re-implement this check — consume it.
- Phase 1 gate scope = accept/abort + `-y`/non-TTY posture. The declared gate need not include `e`/`r` (Phase 4 adds them). The gate is **interactive only** and **ON by default** (no config toggle).
- Tests use the recording presenter (scripted choice) + fake runner: assert accept commits, abort mutates nothing, `-y` skips and commits, non-TTY-without-`-y` fails loud with no commit.

**Acceptance Criteria**:
- [ ] An interactive run renders the message + `Continue?` via the consumed Presenter `Prompt`.
- [ ] Enter accepts (Enter ⇒ `y`) and the commit is created.
- [ ] `y` accepts and the commit is created.
- [ ] `n` aborts as a true no-op — nothing staged, nothing committed, repo unchanged.
- [ ] `-y` skips the gate (auto-accept) and commits without rendering the prompt.
- [ ] Non-TTY stdin without `-y` fails loud (consumed forbidden-combo rule), no commit, no hang.
- [ ] Gate rendering/input/`-y`-echo/forbidden-combo are consumed (not re-implemented in commit code).
- [ ] The gate is placed before the commit mutation (nothing mutated pre-accept).

**Tests**:
- `"Enter accepts and creates the commit"`
- `"y accepts and creates the commit"`
- `"n aborts and mutates nothing (no commit, repo unchanged)"`
- `"-y auto-accepts and skips the gate"`
- `"non-TTY stdin without -y fails loud with no commit"`
- `"the gate renders before any commit mutation"`

**Edge Cases**:
- Enter accepts; `y` accepts.
- `n` aborts mutating nothing.
- `-y` auto-accept skips the gate.
- Non-TTY without `-y` → fail loud.

**Context**:
> Interactive Review Gate: "Commit reuses the cli-presentation `Continue?` gate rendering (`y`/`n`/`e`/`r`, Enter ⇒ accept), ON by default." Choice mapping: "`y` / accept → stage (if `-a`/`-A`) then commit; then push if `-p`. `n` / abort → do nothing. No auto-unwind needed — nothing has been mutated yet (staging deferred to accept), so abort is a true no-op back to the pre-`mint` state." Posture: "Interactive runs show the message + `Continue?`; `-y` skips it (auto-accept); the shared forbidden-combo rule applies (non-TTY stdin + no `-y` → fail loud)." Lifecycle invariant: "mint mutates nothing until the user accepts the gate … This is what makes abort a true no-op." Scope reuses: "The `Presenter` seam — pretty/plain rendering, `-y` orthogonality, `--plain`, the `Continue?` review-gate rendering (defined in `cli-presentation`)." The `e`/`r` gate actions are Phase 4; the gate is interactive only.

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Interactive Review Gate", "Commit Flow / Lifecycle", "Invariant — mutate nothing until accept".

## commit-command-1-6 | approved

### Task commit-command-1-6: Minimal preflight — empty-index fail-loud

**Problem**: Commit's preflight is deliberately minimal (a commit is a frequent, low-stakes, *local* act, so most of release's strict gates are actively wrong for it), but two checks remain: a git repo must be present, and there must be *something to commit*. An empty index must fail loud — mirroring git's messaging — and the AI must never be invoked on an empty diff. Without this, bare `mint commit` on a clean/empty staging area would call the AI on nothing or produce a meaningless commit.

**Solution**: A minimal commit preflight that runs before message generation: (1) git repo present (anchored at the repo root, same resolution as release — a consumed primitive); (2) something to commit — for the bare path, the existing **staged** index must be non-empty, computed read-only. An empty index fails loud with git's clean-tree message, **before any AI call**. Not-a-git-repo fails loud.

**Solution note**: Phase 1 preflight covers only the **bare (staged-only)** path: empty staged index → fail loud, plus not-a-git-repo. The richer empty-staging distinctions tied to `-a`/`-A` (clean-tree vs "no changes staged" vs tracked-only-`-a`-on-untracked pointing at `-A`) are Phase 2 — do NOT implement the flag-aware empty-staging messaging here. The dropped gates (clean-working-tree, on-release-branch, remote-in-sync) are deliberately NOT added — commit exists to operate on a dirty tree. Repo-root resolution is a consumed primitive (the release plan's `ResolveRoot`); reuse it.

**Outcome**: Bare `mint commit` outside a git repository fails loud (not-a-git-repo) with no AI call and no commit. Bare `mint commit` with an empty staged index fails loud with **"nothing to commit, working tree clean"** (git's message), with **no AI call** on the empty diff and no commit. With a non-empty staged index, preflight passes and the generate-and-commit thread (1-4/1-5) proceeds.

**Do**:
- In the commit orchestrator (1-4), run preflight **before** message generation:
  1. **Git repo present** — resolve the repo root via the consumed root resolver (release's `ResolveRoot`, `git rev-parse --show-toplevel`); not a git repo → fail loud cleanly (no panic). Same anchoring as release.
  2. **Something to commit** — for the bare staged-only path, check whether the index has staged changes, computed read-only (e.g. `git diff --cached --quiet` exit status, or porcelain). Empty staged index → fail loud.
- Empty-index message: mirror git — **"nothing to commit, working tree clean"** for the bare path. (Phase 1 produces this single message; the flag-aware "no changes staged — use `-a`/`-A`/`git add`" and tracked-only-`-a`-points-at-`-A` variants are Phase 2.)
- **No AI on an empty diff**: the empty-index check must short-circuit before L3 generate (1-3) is called — assert no `ai_command`/`claude` invocation is recorded when the index is empty.
- Do NOT add the dropped gates: clean-working-tree, on-release-branch, remote-in-sync (behind/diverged), or any pre-push gate. The spec explicitly drops these for commit.
- All git checks are read-only and go through the consumed `CommandRunner`; tests script an empty vs non-empty `git diff --cached` and a not-a-git-repo `rev-parse` failure.

**Acceptance Criteria**:
- [ ] Not a git repository → fail loud cleanly (no panic, no AI call, no commit).
- [ ] An empty staged index → fail loud with **"nothing to commit, working tree clean"**.
- [ ] No AI/`claude` is invoked when the staged diff is empty (preflight short-circuits before generate).
- [ ] A non-empty staged index → preflight passes and generation/commit proceed.
- [ ] Preflight runs before message generation.
- [ ] The dropped gates (clean-tree, on-branch, remote-sync, pre-push) are NOT implemented.
- [ ] The flag-aware empty-staging variants are NOT implemented (deferred to Phase 2).
- [ ] All checks are read-only via the consumed `CommandRunner`/fake.

**Tests**:
- `"not a git repository fails loud with no AI call and no commit"`
- `"an empty staged index fails loud with 'nothing to commit, working tree clean'"`
- `"no AI is invoked when the staged diff is empty"`
- `"a non-empty staged index passes preflight and proceeds to generation"`
- `"preflight runs before message generation"`

**Edge Cases**:
- Empty index fails loud with "nothing to commit, working tree clean".
- No AI call on an empty diff.
- Not-a-git-repo fails loud.

**Context**:
> Preflight & Safety: "Commit's preflight is minimal. Commit runs only: 1. Git repo present — anchored at the repo root (same resolution as release). 2. Something to commit — after staging; empty → fail loud (see Staging)." Gates dropped: "Clean-working-tree — dropped. Commit exists *to* operate on a dirty tree … On-release-branch — dropped … Remote-in-sync (behind/diverged) — dropped … No pre-push gate even with `-p`." Staging — Empty-staging handling: "Empty staging (nothing to commit after staging) → fail loud; never invoke the AI on an empty diff … Genuinely clean tree … → 'nothing to commit, working tree clean'." Commit Flow: "Preflight (minimal) — git repo present; *something to commit* … Computed read-only. Empty → fail loud." Phase 1 is the bare staged-only path; the flag-aware empty-staging distinctions are Phase 2.

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Preflight & Safety", "Staging Model → Empty-staging handling", "Commit Flow / Lifecycle".
