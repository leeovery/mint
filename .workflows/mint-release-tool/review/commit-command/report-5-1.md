TASK: 5-1 — Parse the -p/--push flag (flag-only, no config default)

ACCEPTANCE CRITERIA:
- -p/--push present => push is armed (push == true) on the run options
- -p/--push absent => push is disarmed (push == false) — the default, no push
- --push long form parses identically to -p
- -p composes in the -Ap and -Apy bundles via the 2-1 short-flag pre-expansion (-Ap -> -A -p; -Apy -> -A -p -y)
- No push config key is defined, read, or defaulted anywhere (the [commit] read stays push-free)
- Push is never armed by default — the flag is the sole source of the armed value
- No push execution / failure-warn / empty-suppression behaviour is implemented (deferred to 5-2..5-5)

STATUS: Complete

SPEC CONTEXT:
Auto-push Behaviour: push is opt-in via -p/--push (default no push), FLAG-ONLY, no config
default ("we never push without the -p flag"). Cross-verb -p divergence (release -p = --patch,
commit -p = --push) is intentional and acceptable. Config Schema -> "Deliberately NOT added
for commit": no push config key. CLI Surface lists -p/--push and the -Ap/-Apy bundles as the
headline ergonomic targets. This task owns ONLY flag parsing + the armed value the rest of
Phase 5 consumes — execution/warn/suppression are 5-2..5-5.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - cmd/mint/commit_flags.go:37 (commitFlags.Push field), :54/:67-68 (-p and --push BoolVar
    onto a single push var, paired short/long like -a/--all), :78 (threaded into commitFlags)
  - cmd/mint/commit_flags.go:99-148 (expandShortFlagBundles / shortFlagBundle — generic over
    every DEFINED single-letter flag via fs.Lookup, so -p and -y join bundles with no per-flag
    special-casing once -p is registered)
  - cmd/mint/main.go:347 (Deps.Push: opts.Push) — flag value threaded to the orchestrator
  - internal/commit/run.go:227 (Options.Push field), :845 (pushAfterCommit gates solely on
    deps.Push; no config read)
  - internal/config/config.go:177-180 (Commit struct = Context + Prompt only), :245-248
    (commitShape = context + prompt only) — no push key exists
- Notes: Clean single-source wiring flag -> commitFlags.Push -> Deps.Push -> Options.Push.
  -p is registered as a defined single-letter flag, which is exactly what lets the 2-1
  pre-expansion fold it into -Ap/-Apy with no bundling-specific code. Verified the [commit]
  config table holds only Context/Prompt; grep for config-driven push reads in internal/commit
  returns only an explanatory comment, no code. pushAfterCommit (run.go:844) is the 5-2/5-3
  execution step, correctly gated on deps.Push — not scope creep into 5-1, which owns only the
  armed value; the boolean it consumes is this task's deliverable.

TESTS:
- Status: Adequate
- Coverage:
  - cmd/mint/commit_flags_test.go TestParseCommitFlags (:43-46): -p arms, --push arms, absent
    -p stays disarmed, -A -p -y combination arms push
  - TestParseCommitFlags_Push (:81-107): -p and --push arm identically; absent leaves disarmed;
    "other flags without push stay disarmed" (-A -y) demonstrates the flag — not config — drives
    the armed value (the parser never consults config), covering the "no push config key" micro
    acceptance
  - TestParseCommitFlags_BundledShortFlags (:175-176): -Ap arms add-all+push; -Apy arms
    add-all+push+auto-accept — both via the same pre-expansion, exercising the 2-1 path for -p/-y
- Notes: Every acceptance criterion and every listed micro-acceptance test maps to a concrete
  assertion. The "no push config key" criterion is verified behaviourally (parser arms only on
  the flag, never on config) AND structurally (config.Commit / commitShape carry no push field).
  Not over-tested: the two push-focused tests have a small deliberate overlap (-p / --push
  identity appears in both the table test and TestParseCommitFlags_Push), which is justified —
  TestParseCommitFlags is the broad surface table and TestParseCommitFlags_Push is the focused
  flag-is-sole-source proof; neither is redundant filler. Tests assert behaviour (resolved
  option values), not implementation details.

CODE QUALITY:
- Project conventions: Followed. Paired short/long BoolVar onto one var mirrors the existing
  -a/--all and -y/--yes idiom; resolveStagingMode mirrors resolveBump. Table-driven t.Parallel
  tests match golang-testing. Doc comments are full sentences naming the WHY.
- SOLID principles: Good. parseCommitFlags owns parsing only; resolution of the armed value is
  a plain field; execution lives behind Deps.Push in a separate package — clean separation of
  the flag-parse concern from push execution (the explicit 5-1 boundary).
- Complexity: Low. The flag addition is two BoolVar lines; bundling expansion is generic and
  already in place from 2-1, so -p added zero new branches.
- Modern idioms: Yes.
- Readability: Good. Comments at commit_flags.go:63-68 and run.go:220-227 explicitly state the
  flag-only / no-config-default contract, so the load-bearing invariant is self-documenting.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
