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

  Discussion Map — Interactive mint init Setup (8 subtopics · 8 pending)

  ┌─ ○ Scope of prompted keys [pending]
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

## Summary

### Key Insights

*(to be filled as the discussion progresses)*

### Open Threads

*(to be filled)*

### Current State

- Nothing decided yet — map seeded from the discovery carrier and codebase grounding.

## Triage

(none)
