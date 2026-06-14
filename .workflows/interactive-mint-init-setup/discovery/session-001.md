# Discovery Session 001

Date: 2026-06-14
Work unit: interactive-mint-init-setup

## Description (as of session)

Make `mint init` interactive — prompt the operator for config keys during
scaffolding and write a tailored `.mint.toml`, with a non-interactive
`-y`/non-TTY fallback to the static template.

## Seed

- seeds/2026-06-13-interactive-mint-init-setup.md (inbox:idea)

## Imports

(none)

## Map State at Start

(n/a — single-topic work)

## Exploration

Originated from an inbox idea captured out of the ai-model-selection
discussion. Today `mint init` is a pure generator: it emits a static,
commented `.mint.toml` template with no questions asked. The work is to
make init *interactive* — prompt the operator during scaffolding so the
generated config is tailored to their answers rather than a one-size
template they then hand-edit.

The seed prompt was AI model selection (ask which model/command and
timeout for release notes vs commits, write the per-verb keys straight
into the scaffold), but the idea generalises: init could also ask about
`tag_prefix`, the publish provider, `release_branch`, changelog enablement,
and so on. So this is an init-command UX capability, not a model-selection
detail.

Shaped as a single coherent feature. New behaviour, one scope (make the
init command ask questions). It carries a real design surface — which keys
are worth prompting for, what the defaults are when a prompt is skipped,
how the interactive and `-y` paths stay in sync, and keeping the prompts
from becoming a tedious wizard — but it all serves the one deliverable.
Not a bugfix (nothing broken), too design-heavy to be a quick mechanical
change, and it ships a concrete capability rather than defining a
project-wide pattern, so not cross-cutting. User confirmed it's one thing,
not several.

Known constraints / pointers carried from the seed: `internal/initgen` is
the pure template generator that would gain optional interactivity;
`internal/presenter` is the only output/prompt surface (`AskLine` is the
free-text read, gates go through `Prompt`) — interactivity must route
through there, never raw stdin; and the "fail loud, never hang" invariant
means `-y`/non-TTY init must not block waiting for an answer — there has
to be a clean non-interactive path falling back to sensible defaults
(presumably today's static template). A noted-but-deferred scope question:
init may run against a repo that already has a partial `.mint.toml` — does
it prompt only for missing keys, refuse to clobber, or offer to update?
Flagged as a larger surface, not to solve at discovery.

## Edits

(none)

## Topics Identified

(none)

## Conclusion

(none)
