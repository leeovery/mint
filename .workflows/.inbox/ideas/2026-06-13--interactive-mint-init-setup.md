# Interactive `mint init` setup

Today `mint init` emits a static, commented `.mint.toml` template — a pure generator with no questions asked. The idea: make `mint init` *interactive*, prompting the operator during scaffolding so the generated config is tailored to their answers rather than a one-size template they then hand-edit.

The prompt that surfaced this was AI model selection: init could ask "which model/AI for release notes?" and "which for commits?" and write the resulting per-verb `ai_command` (and timeout) keys straight into the scaffold. But the idea naturally generalises beyond models — init could just as well ask about `tag_prefix`, the publish provider, `release_branch`, whether to enable the changelog, and so on. The interactive flow is an init-command UX capability in its own right, not a model-selection detail.

Why it's worth capturing separately rather than folding into the AI-model-selection work: making init interactive changes the command's nature. `internal/initgen` is currently described as "the pure `mint init` template generator" — turning it interactive pulls in real machinery: presenter prompts (the `AskLine` free-text read path), TTY detection, and crucially the project's "fail loud, never hang" invariant — an unattended `-y` or non-TTY invocation must not block waiting for an answer, so there has to be a clean non-interactive path that falls back to sensible defaults (presumably exactly today's static template). That's a meaningful design surface: which keys are worth prompting for, what the defaults are when skipped, how the interactive and `-y` paths stay in sync, and how to keep the prompts from becoming a tedious wizard.

There's also a smaller, related question of scope: an interactive init is most valuable on a fresh project, but `mint init` may also run against a repo that already has a partial `.mint.toml` — does it prompt only for missing keys, refuse to clobber, or offer to update? Not something to solve now, just noting the surface is larger than "ask a few questions."

Constraints / pointers if this is picked up later:
- `internal/initgen` — the pure template generator that would gain (optional) interactivity.
- `internal/presenter` — the only output/prompt surface; `AskLine` is the free-text read, gates go through `Prompt`. Interactivity must route through here, not raw stdin.
- The "fail loud, never hang" invariant: `-y`/non-TTY init must not prompt.
- The in-scope counterpart is already being handled inside the `ai-model-selection` feature: init's *static* template will surface the new per-verb `ai_command`/timeout keys (commented, with documented defaults) so the feature is discoverable without any interactivity. This idea is strictly about going further — making init *ask*.

Captured 2026-06-13 from the ai-model-selection discussion.
