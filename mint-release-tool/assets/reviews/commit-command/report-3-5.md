TASK: 3-5 — Fail loud when the fallback has no message source (the editor-path no-message-source guard)

ACCEPTANCE CRITERIA:
- A fallback under -y fails loud with 'no AI message and no interactive editor available' — no editor launch, no staging, no commit
- A fallback under non-TTY stdin fails loud (per the startup-resolved StdinInteractive signal) — no editor launch, no staging, no commit
- The editor's interactivity is gated on the SAME startup-resolved stdin determination the gate uses (presenter.DetectStartupSignals(...).StdinInteractive threaded from startup) — no separate stdout/controlling-terminal (/dev/tty) probe, no isatty re-implementation, no accessor hunted on the Presenter interface
- A fallback on a TTY with no launchable editor fails loud (3-1 not-launchable signal) — there is no message to fall back to
- The guard applies identically across all three triggers (--no-ai, generation-failure, oversized)
- The run never hangs (guard fires before any editor launch / blocking read) and never commits an empty message
- No -m/--message escape hatch is added
- The -y/non-TTY condition reuses the once-resolved startup signal; the not-launchable signal is consumed from 3-1

STATUS: Complete

SPEC CONTEXT:
Spec ($EDITOR Fallback — Path Semantics) requires the three "no AI message" cases (--no-ai, AI-generation failure, oversized diff) to all drop to $EDITOR, which is inherently interactive (requires a TTY). When a fallback fires under -y or non-TTY stdin, mint must fail loud ("no AI message and no interactive editor available") — never hang, never commit empty — extending the gate's forbidden-combo philosophy (unattended + needs-human -> fail loud) to the editor path. There is explicitly NO -m/--message escape (unattended-with-own-message uses plain git commit). When no editor in git's chain resolves to a launchable program on a TTY, the fallback path (no message yet) also fails loud — distinct from the `e` gate action where a message already exists and graceful degrade applies (Phase 4). The gating must reuse the once-resolved startup stdin determination, not a separate probe.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/run.go:402-414 — runEditorFallback's no-message-source guard fires BEFORE OpenEditor: `if deps.editorUnavailable() { return surface(p, "editor", errNoMessageSource) }`
  - internal/commit/run.go:240-242 — editorUnavailable() = `d.Yes || !d.StdinInteractive` (the SINGLE shared predicate)
  - internal/commit/run.go:416-427 — post-launch path maps ErrNoEditor / runner.ErrCommandNotFound (the 3-1 not-launchable signal) to the SAME errNoMessageSource fail-loud
  - internal/commit/run.go:106 — errNoMessageSource = exact spec string "no AI message and no interactive editor available"
  - internal/commit/run.go:303-305 (--no-ai), 318-323 (oversized skip), 330-332 (AI-failure), 348-350 (regen-failure) — all four triggers converge on runEditorFallback, so the one guard covers them all
  - internal/commit/run.go:318-321 — oversized "opening editor" note is gated on the SAME editorUnavailable() predicate, so an unattended oversized run fails loud with no contradictory preceding note
  - cmd/mint/main.go:318 — `signals := presenter.DetectStartupSignals(opts.Plain, os.Stdout, os.Stdin)` resolved ONCE; main.go:343 threads `signals.StdinInteractive` to the engine guard; main.go:322 + presenter/wiring.go:67-77 thread the SAME determination into the presenter/gate. Single startup resolution, two consumers, no drift.
- Notes: The acceptance criterion most at risk of being faked — "same startup-resolved determination the gate uses, no separate probe" — is satisfied exactly. Both the presenter (NewForStartup -> DetectStartupSignals, wiring.go:67) and the engine guard (main.go:318/343) derive StdinInteractive from the identical DetectStartupSignals(plain, stdout, stdin) call over the real os.Stdin descriptor via the shared IsTerminal primitive (presenter/gating.go:34,64-69). No /dev/tty probe, no re-implemented isatty, no accessor on the Presenter interface (Deps carries it as a plain boolean per run.go:212-219). The guard sits strictly before OpenEditor (run.go:412 precedes run.go:416), so no editor launch / blocking read can occur — the never-hang guarantee is structural. No -m/--message flag exists anywhere in commit_flags.go. Aborted-editor-on-TTY is correctly NOT rewritten into the fail-loud (OpenEditor returns ok=false/nil-err for a quit, run.go:428-432 -> errEditorNoOp), keeping the guard narrow.

TESTS:
- Status: Adequate
- Coverage (internal/commit/run_failloud_test.go):
  - TestRun_FallbackUnderYes_FailsLoud — table across NoAI / AIFailure / Oversized: exact spec message, exactly one StageFailed, zero editor launches, zero add, zero commit (assertFailLoudNoMutation). A launchable editor is seeded so the test proves the guard fires regardless of editor availability.
  - TestRun_FallbackUnderNonTTYStdin_FailsLoud — same table under StdinInteractive=false, Yes=false: proves the non-TTY vector independent of -y.
  - TestRun_FallbackNonTTYStdin_NeverReachesEditorResolution — asserts NO `git var GIT_EDITOR` resolution ran (editorGitInvocations scan), pinning the guard to the threaded startup signal rather than editor launchability — directly verifies the "same startup-resolved determination, no separate probe" criterion.
  - TestRun_FallbackOnTTY_NoLaunchableEditor_FailsLoud — table over ErrNoEditor (failed git var) and CommandNotFound-on-launch: TTY + no launchable editor fails loud with the SAME message (3-1 signal consumed), no add, no commit.
  - TestRun_FallbackOnTTY_AbortedEditor_StaysTrueNoOp — narrowness guard: an aborted editor on a TTY is NOT rewritten into the fail-loud message; editor IS reached (1 launch), no commit.
  - TestRun_NoAI_TTY_LaunchableEditor_SaveAsAcceptStillWorks — regression guard: a legitimate interactive run still commits the saved body; the new guard does not over-fire.
  - oversizedRoot drives the REAL Generator size guard (max_diff_lines=1) so the oversized case routes through the genuine notes.ErrDiffTooLarge path rather than a stubbed branch.
- Notes: Balanced — not under-tested (all three triggers x both -y and non-TTY vectors, plus TTY-no-launchable, plus never-hang/never-commit-empty asserted via zero-launch/zero-commit/zero-add, plus -m absence covered by its non-existence and the no-escape design). Not over-tested — the shared assertFailLoudNoMutation helper and table structure remove redundancy; each table row exercises a distinct trigger, and the two narrowness/regression tests guard against the guard over-firing rather than re-asserting the happy path. Assertions are byte-exact on the spec message (failLoudMessage const) and behavioural (launch/add/commit counts via the FakeRunner-backed editorRunner whose Mutator wraps the same runner, so any mutation would be recorded), not implementation-detail coupled.

CODE QUALITY:
- Project conventions: Followed. Idiomatic Go sentinel errors (var errNoMessageSource = errors.New(...)), errors.Is routing, %w wrapping preserved through surface; the single-predicate editorUnavailable() keeps the two consumers (guard + oversized note) in lock-step per the golang DRY/naming lens.
- SOLID principles: Good. The guard is a single-responsibility predicate; the convergence of all triggers on runEditorFallback means the rule lives in exactly one place (open/closed for the trigger set — a new trigger reaching runEditorFallback inherits the guard for free). StdinInteractive injected as a boolean (dependency inversion — the engine depends on a value, not on the Presenter's TTY internals).
- Complexity: Low. editorUnavailable is a two-term boolean; runEditorFallback is a linear guard -> open -> classify-save sequence.
- Modern idioms: Yes. errors.Is sentinel discrimination, table-driven subtests with t.Parallel().
- Readability: Good. Doc comments are unusually thorough and accurately describe the mirror relationship between the fail-loud (no message yet) and the `e` graceful-degrade (message exists) consumers of the same 3-1 signal.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
