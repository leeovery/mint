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

  Discussion Map — Interactive mint init Setup (9 subtopics — 1 decided · 1 exploring · 7 pending)

  ┌─ ✓ Interactivity model (ambition) [decided]
  ├─ ◐ Scope of prompted keys [exploring]
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

*State: exploring.*

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

### Journey (so far)

Started from "AI model is the obvious one prompt." User pushed: mint has no model
concept, it's baked in the command — so a model prompt isn't real unless we invent
model-knowledge. That exposed the trigger key as a weak prompt and surfaced a better
one (same/different-AI structural branch). User also floated `diff_exclude`. Still
open: does the targeted set reduce to just the same/different-AI branch, or include
`diff_exclude`? Is there a worthwhile core at all, or is the honest answer "ship the
commented template, ask later"?

## Summary

### Key Insights

1. Every config key already has a sane default (post `ai-model-selection`), so
   interactive init's value is ergonomics + discoverability, not filling blanks — a
   high bar for any prompt.
2. AI is modelled as an opaque command string, not a first-class model — so a "pick a
   model" prompt isn't natively possible without inventing model-knowledge (rejected,
   provisional).

### Open Threads

- Does the targeted prompt set reduce to one structural question (same/different AI),
  or also include `diff_exclude`?
- Is the feature's core worthwhile given good defaults, or should it be right-sized
  hard (e.g. opt-in `--interactive` asking only the same/different-AI question)?

### Current State

- **Decided:** Interactivity model = A (targeted overlay).
- **Exploring:** Scope of prompted keys.

## Triage

(none)
