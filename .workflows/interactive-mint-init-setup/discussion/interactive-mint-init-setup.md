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

  Discussion Map — Interactive mint init Setup (11 subtopics — 1 decided · 2 converging · 1 exploring · 7 pending)

  ┌─ ✓ Interactivity model (ambition) [decided]
  ├─ → Scope of prompted keys [converging]
  │  ├─ → AI prompt flow (model × per-verb) [converging]
  │  └─ ◐ AI-assisted diff_exclude analysis [exploring]
  ├─ ○ Non-interactive fallback (-y / non-TTY) [pending]
  ├─ ○ Defaults & single source of truth [pending]
  ├─ ○ Output shape of the generated file [pending]
  ├─ ○ Wizard UX — avoiding tedium [pending]
  ├─ ○ Seam & architecture placement [pending]
  ├─ ○ Presenter prompt capabilities [pending]
  └─ ○ Existing / partial .mint.toml [pending]

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

## Summary

### Key Insights

1. **North star: smoother init.** Prompts remove the hand-edit step for the 1-2 things
   that vary; they don't interrogate.
2. Every config key already has a sane default (post `ai-model-selection`), so the bar
   for "worth a prompt" is high — value is ergonomics + discoverability, not filling
   blanks.
3. AI is an opaque command string, not a first-class model. Resolved by framing: Claude
   assumed → model menu + `custom` escape (escape preserves agnosticism).
4. The AI-assisted `diff_exclude` scan is what makes the feature genuinely valuable vs
   ergonomic sugar — but it's also the one thing that pulls AI into init's minimal core.

### Open Threads

- diff_exclude scan: mechanism (AI vs heuristic vs both) and phasing (v1 vs follow-on).
- Per-verb `timeout` auto-write when a slow model is picked (parked).
- Presenter capability for menu + approve/skip prompts.

### Current State

- **Decided:** Interactivity model = A (targeted overlay).
- **Converging:** Scope (AI model per verb + diff_exclude; everything else out); AI
  prompt flow (no-repeat model × per-verb + custom escape).
- **Exploring:** AI-assisted diff_exclude analysis (mechanism + phasing + presenter).

## Triage

(none)
