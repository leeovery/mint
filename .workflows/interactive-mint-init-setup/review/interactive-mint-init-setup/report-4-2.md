TASK: interactive-mint-init-setup-4-2 — Add the any-AI entry-point prompt routing operators to mint setup

ACCEPTANCE CRITERIA:
- [ ] README carries an entry-point prompt that names `mint setup` and instructs the operator to run it and follow what it prints.
- [ ] The prompt frames mint as an AI tool for commits & releases and frames setup for "any AI of choice" (naming at least Claude and one other, e.g. Codex).
- [ ] The prompt carries the light Opus-level steer (a recommendation, not a requirement).
- [ ] The README ROUTES to `mint setup` and does NOT restate the guide's contents (no inline copy of procedure, etiquette, minimalism, or config-reference sections).
- [ ] No fidelity-floor machinery, model gate, or new code/config is introduced.
- [ ] The entry point does not imply auto-install (assumes mint is installed, consistent with Install section).
- [ ] README still builds as valid Markdown (no broken fences or anchors).

STATUS: Complete

SPEC CONTEXT:
Spec "README — entry point" (specification.md:195-197): README carries the tiny entry-point prompt pointing operators at the binary-emitted guide — roughly "mint is an AI tool for commits & releases; run `mint setup` and follow what it prints." README does NOT restate the guide; it routes to it (binary is the version-matched source).
Spec "README — 'any AI' framing (light, no machinery)" (specification.md:199-201): Frame as "to set up, pass the following prompt to your AI of choice — Claude, Codex, …" with a light steer "we find Opus-level models do the best work here." NO fidelity-floor machinery — a convenience; weak-AI choice is the operator's. Strict-schema loud-fail + natural "verify the config loads" remain as backstops, not defensive engineering.
Spec "Install handling" (specification.md:54): README entry point assumes mint installed and links install; if `mint setup` not found the agent asks the user to install — mint does not auto-install.

IMPLEMENTATION:
- Status: Implemented
- Location: /Users/leeovery/Code/mint/README.md:58-64 (new `### Setup` subsection under `## Quick Start`).
- Notes:
  - README.md:60 — "mint is an AI tool for commits & releases. To configure it for your project, pass the prompt below to your AI of choice — Claude, Codex, or whatever you run (we find Opus-level models do the best work here):" — satisfies the AI-tool framing, the any-AI framing naming Claude + Codex, and the light Opus-level steer in one line. The steer is phrased as a finding ("we find … do the best work"), i.e. a recommendation, not a requirement. Good.
  - README.md:62 — the prompt itself, as a blockquote: "> Run `mint setup` and follow what it prints." — names `mint setup` and instructs run-and-follow, matching the spec's roughly-quoted phrasing verbatim.
  - README.md:64 — "`mint setup` emits a version-matched setup guide that inspects your project and proposes a config — it is the source of truth, so this README does not reproduce it here. (This assumes mint is already installed; see [Install](#install).)" — explicitly states the binary is the version-matched source and that the README does NOT reproduce the guide (routes, does not restate). The parenthetical pins the install assumption and links the Install section without implying auto-install.
  - Placement: chosen as a dedicated `### Setup` subsection immediately after the Quick Start fenced block — one of the two spec-blessed options ("Quick Start lead-in vs a dedicated block"). Reads cleanly in the surrounding narrative.
  - No drift: the change is confined to README prose. `git status` is clean on a committed tree; no code/config files touched by this task.

TESTS:
- Status: Adequate (no automated test is correct for this task)
- Coverage: The plan explicitly states this task has no Go test (README prose editing, no compiler); verification is manual-narrative read. The Task 4-3 tripwire deliberately does NOT check this prose (it only asserts schema key NAMES appear), so no automated coverage is expected or appropriate here. Adding a prose-matching test would be over-testing brittle free text.
- Notes: Manual-read verification points all pass:
  - "routes a new operator to mint setup and tells them to follow what it prints" — README.md:62,64. PASS.
  - "entry point frames setup for any AI with a light Opus-level steer" — README.md:60. PASS.
  - "does not duplicate the guide's procedure/etiquette/minimalism/config-reference body" — README.md:58-64 contains none of the inspect-and-map steps, etiquette, minimalism, or a config-key list; it stops at the route. The Configuration section's per-key tables (README.md:182+) are the pre-existing human config reference (owned by Task 4-1), not a restatement of the `mint setup` guide, and the spec explicitly accepts the README as the human config surface. PASS.
  - "no fidelity-floor machinery or new code is added" — confirmed: no model gate, no capability check, no new code/config. PASS.

CODE QUALITY:
- Project conventions: N/A (documentation prose; no Go code). The relevant project skill area is README/docs. The "any AI (Claude or any AI)" framing and the GitHub-docs-vs-`mint setup` SoT split are consistent with CLAUDE.md's content-agnostic AI posture and with the spec's two-surface model (humans → README/docs; agent → `mint setup`).
- SOLID principles: N/A (prose).
- Complexity: Low — four lines, single responsibility (route to the guide).
- Modern idioms: N/A.
- Readability: Good. The blockquote isolates the literal prompt the operator pastes, the lead-in carries the framing/steer, and the trailing line carries the SoT-and-install caveats. Clear separation of concerns within the prose.
- Issues: None.

Acceptance criteria assessment (all met):
- (a) Entry-point prompt names `mint setup` + run-and-follow — MET (README.md:62, "Run `mint setup` and follow what it prints").
- (b) Routes to the binary guide, never restates it; binary is version-matched SoT — MET (README.md:64 states explicitly "it is the source of truth, so this README does not reproduce it here"; no procedure/etiquette/config body inline).
- (c) Any-AI framing (Claude, Codex, …) with a light Opus-level steer, no fidelity-floor machinery — MET (README.md:60; steer is "we find … do the best work here", a recommendation; no gate or capability check anywhere).
- (d) Strict-schema loud-fail + natural verify-config-loads remain as backstops, not newly added defensive machinery — MET by omission: this task adds no defensive machinery, and the pre-existing `DisallowUnknownFields` strict decoding (internal/config) is untouched. The entry point does not invent a verify step; the parenthetical "verify the config loads" backstop is left to the binary guide / next `mint` run as the spec intends.
- (extra) No auto-install implication — MET (README.md:64 parenthetical "This assumes mint is already installed; see [Install](#install)").
- (extra) Valid Markdown — MET: Quick Start ```bash fence (README.md:50) closes at :56; the new `### Setup` heading and `>` blockquote at :62 are well-formed; the `[Install](#install)` anchor resolves to the existing `## Install` heading (README.md:34). TOC link `#quick-start` (README.md:13) still resolves; the new `### Setup` is a subsection and needs no TOC entry.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None. The implementation is tight, matches the spec phrasing closely, and adds nothing beyond the routing entry point. No do-now/quickfix/idea/bug findings that propose a concrete change.
