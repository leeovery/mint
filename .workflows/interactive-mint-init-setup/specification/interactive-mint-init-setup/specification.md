# Specification: Interactive Mint Init Setup

## Specification

### Overview

`mint init` stays a pure static-file generator — it grows **no** interactive surface. Interactivity is offloaded to an **AI setup guide emitted by the binary**: a new thin `mint setup` subcommand prints an embedded static instruction string that teaches an AI agent (Claude or any AI) how to configure mint for a project — what mint does, what each config option means, what to inspect in the project (existing release process → hooks, noise dirs → `diff_exclude`, version file, AI model per verb), and how to propose and confirm changes interactively. The agent does the research and runs the natural-language setup session; **mint itself never prompts**.

This supersedes the original framing ("make `mint init` interactive"). The "augment only what varies" principle survives as the guide's minimalism rule, but there is no mint-side overlay or wizard — the agent does the tailoring.

### Goal (end state)

A `.mint.toml` that is **up to date with the installed mint version** and **contains the project's relevant config — whether sourced from an existing file or built fresh**. "Setup" therefore doubles as "upgrade my config to this mint version."

### Why binary-emit is the keystone

Emitting the instructions from the binary means they:
- **Cannot drift** from the schema that binary implements — drift-testable, exactly like the existing `initgen`↔`config` drift tests.
- **Cannot version-skew** — an old mint emits old-but-correct instructions, so there is no N-versions doc maintenance.
- Are **reachable** wherever mint is installed (no clone/fetch step).
- Are **human-readable**, so a user with no agentic AI or no web access can still read and follow them.

This turns the deliverable into a **thin Go feature** (subcommand + embedded string + drift test) that slots into mint's existing generator + cmd + test patterns.

### Non-goals / out of scope

- **No mint-side interactive wizard.** mint never prompts during init; the fail-loud/non-interactive (`-y`/non-TTY hang) concern is dissolved because mint never reads stdin for setup.
- **No ecosystem-aware static `diff_exclude` defaults** (option B — explicitly rejected). A wrong default is worse than none (`docs/**` is wrong for a docs tool); the guide surfaces patterns interactively instead. Accepted tradeoff: the non-AI path gets no `diff_exclude` help.
- **No in-init AI calls / no AI in `engine.Init`.** The in-init AI `diff_exclude` scan is deferred (sketch preserved in the discussion); init stays AI-free.
- **No auto-install.** Setup assumes mint is installed; if `mint setup` isn't found the agent asks the user to install — mint does not install itself (a binary install is a far larger blast radius than editing a config).
- **Deferred to possible future work:** per-verb `timeout` auto-write for slow models; a Claude-Code skill wrapper.

### The `mint setup` subcommand

A new top-level verb (working name `mint setup`; the exact command name is a planning detail) that emits the AI setup guide. It is a **pure string emitter** in the spirit of `initgen` — it prints an embedded static instruction string and performs no IO beyond writing to stdout.

**What it emits** (one embedded string, all version-matched to the installed binary):
1. The setup **procedure** (the inspect-and-map flow — see the guide-content sections).
2. The AI **etiquette** rules.
3. The **config reference** — rendered from the config-metadata source of truth (the `key · level · default · description` table) so the agent reads option meanings from a drift-tested table rather than from template comments.

**Runs unconditionally — no git/cwd guard.** Unlike `mint init` (which resolves the repo root via `git rev-parse --show-toplevel` and fails loudly outside a work tree), `mint setup` is a pure text emitter: it must print even when the operator is not yet inside the target repo (setup instructions are commonly read before `cd`-ing in). Safety lives in the *instructions* instead — the emitted guide tells the agent to confirm it is in the intended repo root before inspecting or writing, and `mint init` (run during setup) remains the loud-fail backstop outside a work tree.

**Help-surface wiring (the curated-help contract).** `mint setup` threads through mint's existing hand-written help surface exactly like every other verb:
- A `rootUsage` command-list line for `setup`.
- A curated `setupUsage` text for `mint setup --help` (printed to stdout, exit 0, via the `flag.ErrHelp` path).
- Dispatch wiring in `classifyCommand` / `run` (a new `commandSetup` route).
- The coverage test the help contract requires (the existing test that pins every verb's usage coverage).

`mint help` stays the **frozen curated text** — it gains only the `setup` command line and is **not** retrofitted into a dynamic renderer. `mint help` deliberately does **not** carry the config reference: humans get config reference from the GitHub docs / README; the agent gets it from `mint setup`.

**Install handling.** The README entry point assumes mint is installed and links the install. If `mint setup` is not found, the agent asks the user to install mint — mint does **not** auto-install itself.

---

## Working Notes
