# Discussion: Interactive `mint init` Setup

## Context

Today `mint init` is a pure generator: `engine.Init` resolves the repo root, reads
two static strings from `internal/initgen` (`MintTOML()` and `ReleaseShim()`), and
drops `.mint.toml` + the executable `release` shim at the root — idempotently,
non-clobbering, with `--force` to regenerate. No questions asked. The presenter is
constructed non-interactively (`yes=true`) because init has no gate.

The work is to make `mint init` *interactive* — prompt the operator during
scaffolding so the generated `.mint.toml` is tailored to their answers rather than a
one-size commented template they hand-edit. The seed trigger was AI model selection
(ask which model/command + timeout for release notes vs commits, write the per-verb
keys straight into the scaffold), but the idea generalises to `tag_prefix`, the
publish provider, `release_branch`, changelog enablement, and more.

The hard constraints carried from discovery and the project contract:

- **Interactivity routes through the presenter, never raw stdin.** `AskLine` is the
  free-text read; choice gates go through `Prompt(Gate)`. (`internal/presenter` is
  the only output/prompt surface.)
- **Fail loud, never hang.** A `-y` or non-TTY init must NOT block waiting for an
  answer — there must be a clean non-interactive path falling back to sensible
  defaults (presumably today's static template). Note: `AskLine` already returns
  `ErrNotInteractive` (fail-loud) on non-interactive stdin and `-y` does NOT
  auto-answer free text — so init must *branch around* prompting when non-interactive,
  not lean on AskLine's failure.
- **initgen is currently a PURE generator** (no IO, no config import; default values
  are static literals pinned by drift tests to `config.DefaultAICommand` /
  `config.DefaultTimeout`). Adding interactivity must not quietly break that contract.

### References

- Seed: `seeds/2026-06-13-interactive-mint-init-setup.md` (inbox:idea, from the
  ai-model-selection discussion)
- Discovery: `discovery/session-001.md`
- Code: `internal/initgen/initgen.go`, `internal/engine/init.go`, `cmd/mint/init.go`,
  `internal/presenter/presenter.go` (`Prompt`/`AskLine` seams)

## Discussion Map

A living index of subtopics tracked during the discussion. Grows as the conversation
branches, converges as decisions land.

### States

- **pending** (`○`) — identified but not yet explored
- **exploring** (`◐`) — actively being discussed
- **converging** (`→`) — narrowing toward a decision
- **decided** (`✓`) — decision reached with rationale documented

### Map

  Discussion Map — Interactive mint init Setup (10 subtopics — 5 decided · 5 converging)

  ┌─ ✓ Interactivity model (ambition) [decided]
  ├─ → Scope — what to configure [converging]
  │  ├─ → AI command & model (per verb) [converging]
  │  └─ ✓ diff_exclude (release-notes noise) [decided]
  └─ → AI setup guide (the pivot) [converging]
     ├─ ✓ Delivery — binary-emit (`mint <setup-cmd>`) [decided]
     ├─ ✓ Anti-drift & verification (emit + drift test) [decided]
     ├─ → Guide content & procedure [converging]
     ├─ → AI etiquette (existing config) [converging]
     └─ ✓ Static defaults floor (B → out) [decided]

*Pivot confirmed. Guide procedure + AI etiquette locked; B decided OUT (no static
diff_exclude defaults — the guide surfaces them interactively). review-002 gaps are the
agenda for the still-open threads (delivery/reachability, anti-drift enforcement, guide
depth, def-of-done, any-AI floor).*

---

*Subtopics are documented below as they reach `decided` or accumulate enough
exploration to capture.*

---

## Interactivity model (ambition)

### Context

How ambitious should the interactive flow be? This is the upstream fork — it sets the
shape of scope, output, and UX. The seed explicitly warned against "a tedious wizard."

### Options Considered

- **A — Targeted overlay.** Keep today's full commented template as the floor. Prompt
  only for the handful of keys that genuinely vary per project; write those active,
  leave the rest as the static commented template. Smallest surface; preserves the
  "uncomment to tune" philosophy; non-interactive fallback is trivially equal to
  today's output.
- **B — Full wizard.** Walk every meaningful key. Most tailored, but exactly the
  tedious wizard the seed warned against — and most keys have a fine default nobody
  wants to be asked about (`max_diff_lines`, `tag_prefix='v'`).
- **C — Tiered.** Short default path + opt-in deeper/`--advanced` pass. Flexible but
  doubles the surface and the sync burden.

### Decision

**A — Targeted overlay.** Interactivity should *augment* the 2-3 decisions a static
file genuinely can't make, not re-ask things with good defaults. Confidence: high.

---

## Scope of prompted keys

*State: converging.*

**North star (user, clarified):** make initiating a project *smoother* — first for the
author, and therefore for everyone. The prompts exist to remove the open-the-file-and-
hand-edit step for the 1-2 things that actually vary, not to interrogate.

### Context

Which keys does init actually ask about? Position A says: only those that genuinely
vary per project AND have no good auto-default. Critical framing: the
`ai-model-selection` work already gave **every** key a sane default, so interactive
init is NOT filling undecidable blanks — its value is narrower: **ergonomics** (pick
at the prompt vs hand-edit the file) and **discoverability** (a question is seen; a
commented key can be missed). That sets a high bar for "worth a prompt."

### Key insight — AI "model" is not first-class

mint has no first-class concept of an AI model. The config models AI as an opaque
command *string* (`ai_command`) on purpose: it keeps mint AI-agnostic (any CLI, not
just Claude) and version-proof (`ai-model-selection` chose the `--model sonnet`
*alias* over a baked-in full model ID specifically to avoid staleness). Consequences
for prompting:

- **Baking in a Claude model menu** (sonnet/opus/haiku → splice `--model X`) fights
  the grain three ways: contradicts AI-agnosticism, reintroduces the staleness the
  alias avoided (list rots as new models ship), and invents a first-class concept that
  exists nowhere else. Rejected (provisional) — heavy price for init ergonomics.
- **Free-text command string** is honest to the data model but the prompt defaults to
  `claude -p --model sonnet`, so the only realistic action is Enter-to-accept — adds
  ~nothing over the active default already in the template.

→ The trigger key (AI model) may be the *weakest* prompt, not the strongest: the
thing that varies (model) isn't first-class; the thing that's first-class (command)
has a default everyone accepts.

### Candidate set (running filter: varies per project + no good default)

| Key | Default | Verdict |
|---|---|---|
| **"same AI for both verbs, or different?"** | shared top-level | **strong candidate** — a *structural* choice the template can't pre-make; default "same", "different" → ask each command |
| `diff_exclude` ("diff folders") | empty | candidate — genuinely per-project, no default; caveat: at init time you often don't yet know what to exclude (premature → blank) |
| `context` (notes/commit guidance) | empty | weak — free text before first use tends to be empty/throwaway |
| AI command (raw) | `claude -p --model sonnet` | weak as a standalone prompt (Enter-to-accept); only meaningful inside the same/different branch |
| provider / `release_branch` | auto-detected/derived | no — auto-detect wins |
| `tag_prefix` / `changelog` / `publish` / `commit_prefix` | `v` / true / true / 🌿 | no — leave as active default |

### Journey

Started from "AI model is the obvious one prompt." User pushed: mint has no model
concept, it's baked in the command — so a model prompt isn't real unless we invent
model-knowledge. That exposed the trigger key as weak (Enter-to-accept) and surfaced a
better one (same/different-per-verb branch). I over-rejected a curated model menu, then
revised: the shipped default *already* assumes Claude, and the alias names are stable,
so the costs are low. The user then reframed it decisively: don't ask *which AI* (it's
always Claude) — ask *which model*; for anything else, drop to a custom command. That
escape hatch is what preserves AI-agnosticism, so curated shortcuts cost nothing we
cared about. User also confirmed `diff_exclude` is worth it ("every project has files
that should be excluded") and floated the standout idea: reuse the just-configured AI
to *scan the project and propose* the exclude list.

### Direction (converging)

- **AI framing:** Claude assumed → menu of Claude models (sonnet/opus/haiku) **+ a
  `custom` escape** to free-text any other AI command. (Resolves minimal-vs-curated in
  favour of curated-shortcuts-with-escape.)
- **Prompted set is small and deliberate:**
  - **AI model, per verb** — via the no-repeat flow (see child subtopic).
  - **`diff_exclude`** — via AI-assisted scan (see child subtopic).
  - **Out:** `max_diff_lines` (default fine, tune later — user), `context` (free text
    before first use → throwaway), provider / `release_branch` (auto-detect wins),
    `tag_prefix` / `changelog` / `publish` / `commit_prefix` (good active defaults).

---

## AI prompt flow (model × per-verb)

*Child of Scope. State: converging.*

### Context

Prompt the AI model per verb without making the user answer the same question three
times. The config already supports this shape: a shared top-level `ai_command` plus
optional per-verb `[release]`/`[commit]` overrides, resolved `verb → shared → default`.

### Decision (converging)

No-repeat flow:

```
Which Claude model should mint use?  [1 sonnet ·default·  2 opus  3 haiku  4 custom]
Use {choice} for both release notes AND commit messages?  [Y/n]
  └─ n → Which model for commit messages?  [menu again]
```

- First answer is provisionally for **both**; only `n` triggers a second ask. The
  defaults path is two Enters.
- `1–3` → splice `--model X` into `claude -p`. `4 custom` → free-text command (the
  AI-agnostic escape).
- **Config representation:** "same" → write the shared top-level `ai_command` (today's
  shape). "different" → write `[release].ai_command` + `[commit].ai_command` as per-verb
  overrides. (How the un-prompted top-level default coexists is an Output-shape detail.)

### Edge cases / parked

- **Timeout coupling (parked):** the per-attempt timeout is fatal, not retried; a
  slower pick (opus) could bite the 60s default. Picking a slow model *might* warrant
  init also writing a higher per-verb `timeout`. Not deciding yet.
- **`custom` + same/different:** works uniformly — custom command can be shared or
  per-verb just like a model pick.

---

## AI-assisted diff_exclude analysis

*Child of Scope. State: exploring. The feature's standout / differentiating idea.*

### Context

`diff_exclude` targets **tracked** generated files (committed `dist/`, generated code,
lockfiles) — gitignored paths are already absent from the notes diff. These are exactly
what a human forgets and what an AI scan can surface. The idea: after the AI command is
configured, **offer** to scan the project and propose an exclude list the user
approves/skips (and can edit later in the file).

### How it would work

1. After AI config: "Scan your project for files to exclude from release-notes diffs?
   [Y/n]".
2. `Y` → feed the tracked-file list (e.g. `git ls-files`) to the **configured** AI
   command, asking it to return glob pathspecs for generated/vendored/lock files.
3. Present the proposal → **approve** (write to `diff_exclude`) / **skip** (leave
   empty). Inline editing deferred — user edits the file afterward.

### The architectural consequence (the real cost)

Init starts making AI calls. Today `engine.Init` is dead simple (resolve root, write two
static files; `InitDeps` is deliberately narrower than `ReleaseDeps` — no AI, no
mutator). The scan pulls the whole AI transport stack into init. Containment rules that
keep init's *core* AI-free:

- Strictly **opt-in** behind a Y/n.
- **Degrades to "skip"** on any AI failure/timeout — never blocks, never aborts init.
- **Never offered under `-y` / non-TTY** (fail-loud-never-hang: an unattended init must
  not make a network call or wait).
- Writing the two files never depends on the AI working.

### Options for the mechanism (open)

- **AI-analysis** — catches project-specific generated code; matches user intent; reuses
  the configured command. Cost: AI in init, latency, failure handling. *(orchestrator
  lean)*
- **Static heuristic** — detect lockfiles + common patterns (`dist`, `*.pb.go`, …);
  deterministic, free, no AI dependency; misses the long tail.
- **Heuristic baseline enriched by AI** — most thorough, most complex.

### Open questions

- Mechanism choice (above).
- Phasing: prompt-driven config (AI model + same/different) is shippable without the
  scan; the scan is a clean second layer. Decide whether it's in-scope for v1 or a
  follow-on.
- Presenter capability: a model **menu** and an **approve/skip** prompt may not fit the
  existing fixed-Choice `Prompt(Gate)` shape — may need AskLine-based selection or a new
  presenter method (see Presenter prompt capabilities subtopic).

### Resolution — deferred (user)

User cooled on building the scan ("thinking out loud… not a huge deal"). **Deferred, not
killed.** Clean sketch to preserve if revisited: prompt the AI to return *strictly* a JSON
array of glob strings and nothing else (no prose/footers — "this is parsed
programmatically"); code parses deterministically; unparseable → declare the scan failed
and tell the user to set `diff_exclude` manually. No fragile parsing.

Reframe of what `diff_exclude` is *for*: **release-notes noise, not generated code.**
`.gitignore`'d paths (node_modules, vendor) are already absent from the diff. The real
targets are process/meta/doc files — `.workflows/`, `.claude/`, `docs/`, lockfiles (we'd
cite `package.json`, not `package-lock.json`). Those are near-universal for mint's actual
audience (Claude-ecosystem repos), so they're a candidate for a *smarter shipped default*,
not necessarily a prompt. (Largely subsumed by the AI-setup-guide pivot below — the agent
infers these directly.)

---

## Worth-it check (key inventory)

Triggered by the user asking to see every config key to judge whether the feature earns
its keep. Full canonical schema (`internal/config`): ~24 keys across top-level,
`[release]`, `[release.hooks]`, `[commit]`. Verdict through the smoother-init lens (prompt
only what *varies* AND has no good default/auto-detect):

- **Strong:** `diff_exclude` (one key).
- **Modest:** `ai_command` model menu (Enter-to-accept default).
- **Marginal/wizard-y:** `publish`, `changelog` toggles; `version_file`/`version_pattern`
  (coupled, fiddly); hooks (valuable but commands → hard to prompt).
- **Leave as default/auto/commented:** the other ~16 keys — by design.

Finding: mint was *built* so defaults + the self-documenting commented template answer
these for you. That design directly **competes with** interactive init, which is why the
feature kept feeling thin. A/B/C fork put to user — (A) build interactive init, (B) ship
smarter ecosystem-aware static defaults instead, (C) hybrid. Orchestrator lean was B,
then **superseded by the pivot below.**

---

## Offload interactivity to an AI setup guide (CONFIRMED)

*State: exploring (the work's spine now). Pivot confirmed by user; topic name unchanged
(still suits). Supersedes A/B/C and the wizard subtopics (trimmed from the map).*

### Decisions locked

- **Pivot confirmed** — the deliverable is an AI setup guide, not a mint-side wizard.
- **Form factor (decided — refined to binary-emit):** the setup instructions are
  **emitted by the binary itself** — a thin new `mint` subcommand (name TBD, e.g.
  `mint setup`) that prints an embedded static instruction string (same shape as
  `initgen`: a pure string generator). The **README prompt is tiny**: ~"mint is an AI tool
  for commits & releases; run `mint <setup-cmd>` and follow what it prints." Layering, all
  version-matched to the installed binary:
  1. README one-liner → which command to run.
  2. `mint <setup-cmd>` → emits the **procedure + etiquette** (NOT a duplicated option
     reference).
  3. Agent runs `mint init` → reads the generated template's comments as the
     **authoritative, version-matched option meanings** (DRY — option docs live only
     there, already drift-tested).
  - **Why it's the unlock:** emitted-by-binary means the instructions **cannot drift** from
    the schema that binary implements and **cannot version-skew** — an old mint emits
    old-but-correct instructions, so no N-versions doc maintenance. Resolves review **F1**
    (reachability — present wherever mint is installed, no clone/fetch), **F7** (version
    skew), and **F10** (anti-drift: now *enforceable* with a drift test pinning the emitted
    text's schema references, exactly like the existing `initgen`↔`config` drift tests).
    Gives **F3** (definition of done) a real, gate-able answer: a Go test on the command's
    output.
  - **Nature shift:** the deliverable swings back to a *thin Go feature* (subcommand +
    embedded string + drift test) rather than pure content — good news: it slots into
    mint's existing generator+cmd+test patterns.
  - **Install handling (lean):** prompt **assumes mint is installed**; README links the
    install; if `mint <setup-cmd>` isn't found the agent asks the user to install — mint
    does NOT auto-install itself (installing a binary >> editing a config in blast radius).
  - **Non-AI floor improved:** `mint <setup-cmd>` output is human-readable, so a user with
    no web access / no agentic AI can still read and follow it (softens F2).
  - Optional later layer: a Claude-Code skill wrapper.
- **Guide procedure (locked):** the inspect-and-map flow — learn mint (read README +
  commented template; internalise *only set what varies*) → inspect the project (release
  process → **hooks**; noise dirs → `diff_exclude`; version file; AI model per verb;
  provider/branch only if auto-detect would be wrong) → **propose → explain → approve** →
  sanity-check (unknown keys fail loudly via strict schema). Hook/release-process
  detection is the centrepiece.
- **Static defaults floor (B): OUT (user).** Don't ship ecosystem-aware `diff_exclude`
  defaults — a wrong default is worse than none (`docs/**` is wrong for a docs tool).
  `diff_exclude` stays empty/commented in the static template (unchanged); the guide
  surfaces the obvious patterns (`.workflows/`, `.claude/`, agent dirs) **interactively**
  — "that's what this new process is literally for." Accepted tradeoff: the non-AI path
  gets no `diff_exclude` help (review F2 — the gap widens with B out).
- **AI etiquette (locked, user):** the guide instructs the agent to —
  - **Ask interactively** with whatever user-question tool it has — phrased generally for
    AI-agnosticism ("if you're Claude, use your Ask-User tool; otherwise, if you have any
    tool for asking the user questions, use it").
  - **Confirm the user is comfortable**, and make **clear exactly what is being
    changed/updated** before writing.
  - **Never remove anything without explicit permission.**
  - Surfacing the `diff_exclude` patterns happens inside this interactive confirmation.
  - (Resolves review F4 — trust/blast-radius — at the etiquette level.)

### Review reconciliation (set 001)

The pivot **moots** review-001's implementation-half findings (F1–F8, F11) — they
critiqued a mint-side wizard we're no longer building. F9 (Claude-first framing) is
addressed: the guide and prompt are explicitly AI-agnostic. F10 (testing without spawning
AI) transforms into content-accuracy verification, carried by the anti-drift thread.
review-001 marked `incorporated`; the next commit triggers a fresh review of the *pivoted*
design.

### The idea (user)

Don't build an interactive surface in mint at all. Ship a **setup guide / skill / prompt**
that teaches an AI agent (Claude — or any AI) everything: what mint does, what every
config option means, what to look for in a project (files to exclude, AI command, existing
release process, what to wire into hooks). README carries a one-line "give this prompt to
your AI to set mint up for you." The AI does the research and runs the interactive session
in natural language. mint itself stays a pure static-file generator.

### Why it's compelling

- **The whole "how it's built" half evaporates.** No presenter menu capability, no
  AI-in-init seam, no non-interactive fallback, no initgen purity break, no output-shape
  or existing-file handling — mint grows *no* interactive surface, so the review's
  implementation-half gaps are *avoided*, not solved.
- **Richer than any wizard we'd build.** The AI reads the actual project: detects the real
  release process and maps it into mint's **hooks** (preflight/pre_tag/post_release),
  infers noise dirs, understands CI/version files. The hooks-detection is the standout —
  exactly what mint's hooks exist for.
- **The diff_exclude scan, for free** — the AI does the analysis conversationally,
  human-in-the-loop, no JSON-parse fragility, no AI-in-init.
- **AI-agnostic** (user: "doesn't have to be Claude"); maintenance is editing markdown,
  not maintaining prompt loops + presenter methods + tests.

### The one real risk: accuracy / drift

The guide describes config that must stay **true-to-as-built** — the same drift battle mint
already fights (initgen↔config drift tests, README discipline). Mitigation: make the guide
a **procedure, not a restatement** — point the AI at the already-maintained README + the
commented `.mint.toml` template for what each option *means*; the guide supplies only the
part that lives nowhere yet (how to inspect a project and map findings to config) + mint's
minimalist philosophy (only set what varies). Strict schema (`DisallowUnknownFields`) is a
free backstop: a hallucinated key fails loudly at first `mint` run.

### Complementarity (best-of-both holds)

The static commented template **remains** the standalone non-AI path (hand-editable,
documented). Non-AI users edit the template; AI users paste the prompt. This **dissolves
the fail-loud/non-interactive concern entirely** — mint never prompts, so there's no
`-y`/non-TTY hang to design around.

### Open questions (live) — remaining review-002 agenda

*Resolved: F1/F7/F10 (binary-emit), F4 (etiquette), F9 (hook mapping), F8 (minimalism — see
sections below). Still live:*

- **Existing `.mint.toml` (F5)** — detect/diff/preserve-vs-refuse specifics beyond the
  never-remove-without-permission etiquette. (Being discussed now.)
- **"Any AI" fidelity floor (F6)** — realistically Claude-tuned with "any AI" best-effort,
  stated honestly? (overlaps non-agentic-AI tail of F2.)
- **Definition of done (F3)** — beyond the emitted-output Go test, a run-against-sample-
  projects acceptance check?

---

## Hook detection & mapping (F9 — decided)

The standout value (detect the project's existing release process → map into mint's hooks)
needs a clear rule for the imperfect-fit case. mint has three hook phases —
`preflight` (before any release work; failure aborts), `pre_tag` (after notes, before the
tag; **accepts an array** of ordered commands), `post_release` (after publish) — bracketing
the fixed pipeline: preflight → notes → pre_tag → tag+push (PONR) → publish → post_release.

### Decision (user)

- **Propose a best-fit mapping and flag it to the user.** Never silently skip a step — "if
  it's in the customer's release script, it's important."
- When a step **doesn't fit**, surface it honestly. The outcome may legitimately be "mint
  isn't suitable here" or "you'll need to adjust your process" — acceptable, not a failure
  to paper over.
- The agent's job is to **explain mint's model clearly** (the pipeline + where each hook
  slots in) so a technical user can collaborate on a workaround, an adaptation, or a clean
  fit. The instructions **facilitate that conversation**, ending in a clean mint
  implementation — they don't force a mapping.

### Falls out of this (content requirements)

- The emitted instructions **must carry mint's pipeline/stage model** (ordered stages +
  which hook fires where) — otherwise the agent can't explain or map accurately. This is
  drift-sensitive: the model must match the engine, reinforcing the drift test.
- `pre_tag`-as-array widens what fits: a linear multi-step build/test sequence maps to a
  `pre_tag` array, so the genuinely-unmappable set narrows to *needs a step where mint has
  no hook* / non-linear / mid-pipeline approval-gate cases.

### Cross-cutting principle — agent as collaborator (not auto-configurer)

Generalises beyond hooks (informs F8, F5): the guide makes the agent a knowledgeable
collaborator — **explain mint's model, propose, flag, never silently drop or clobber, help
the user fit their process or recognise a genuine misfit.** Not a magic one-shot configurer.

---

## Minimalism — only set what varies (F8 — decided)

The mirror risk to hook-mapping: a thoroughness-rewarded agent populates every inferable
key and hands back a bloated `.mint.toml`, defeating the good-defaults + commented-template
design that motivated the pivot.

### Decision (user + orchestrator)

- **Activate a key ONLY to set a non-default value.** If the default is fine, leave the key
  commented — that is NOT 'skipping' or 'disabling': mint's defaults are compiled into the
  binary and apply whether or not the key appears in the file. The whole `.mint.toml` is
  optional.
- **For every key the agent does activate, state the project-specific reason** at the
  confirmation step — so over-configuration is *visible* (each active key carries a
  justification), not silent.
- **The guide must explain the template's nature explicitly:** the comments are **inert
  documentation** (for human + AI) listing what config exists — they do nothing
  functionally. The real defaults live in the binary and are not readable from the file. So
  'commented = at its default', not 'off'. Call this out clearly so neither human nor agent
  defensively uncomments everything to "make it explicit."

### Refinement (orchestrator catch)

The values shown on commented keys are **illustrative examples, not the defaults** (per
`initgen`: example values are chosen to be schema-valid, not the real default — e.g.
`# timeout = 120` is an example override; the actual default is 60). So the guide must tell
the agent to judge "is the default fine?" from the **prose comment / default behaviour**,
never by treating a commented example value as the default. (Reinforces DRY: the guide never
restates default values.)

## Summary

### Key Insights

1. **North star: smoother init** — remove the hand-edit for the few things that vary.
2. mint's good-defaults + self-documenting commented template **compete with** any
   interactive init — which is why the feature kept feeling thin (only `diff_exclude` had
   strong prompt value).
3. AI is an opaque command string, not a first-class model (resolved by framing: Claude
   assumed → model menu + `custom` escape).
4. **Pivot:** offload interactivity to an AI setup guide/prompt rather than building it
   into mint — richer (project-aware, detects existing release process → hooks), avoids
   the entire implementation-half, AI-agnostic. Static template stays the non-AI floor.
5. **Binary-emit is the keystone:** `mint <setup-cmd>` prints the instructions, so they're
   reachable, version-matched, and drift-testable — turning the deliverable into a thin Go
   feature, not loose content.
6. **Agent as collaborator, not auto-configurer:** explain mint's model, propose, flag,
   never silently drop/clobber — handles imperfect fits and even "mint isn't right here"
   gracefully.

### Open Threads

- Confirm the pivot (reshapes work unit → AI-facing content).
- Form factor (guide + README prompt + optional skill); where the guide lives; how the AI
  fetches it; `mint`-emits-the-prompt option.
- Anti-drift: guide as procedure that references README/template, not a restatement.
- Keep B (ecosystem-aware static `diff_exclude` defaults) as the no-AI floor?

### Current State

- **Decided:** Interactivity model = A (targeted overlay) — *under review by the pivot*;
  AI-assisted diff_exclude scan = deferred.
- **Exploring (live direction):** offload interactivity to an AI setup guide — likely
  supersedes building a mint-side wizard.
- **Superseded-if-pivot-confirmed:** presenter capability, non-interactive fallback,
  initgen purity, output shape, seam placement (mint grows no interactive surface).

## Triage

(none)
