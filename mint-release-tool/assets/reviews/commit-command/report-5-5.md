TASK: 5-5 — Suppress push on empty/aborted runs; confirm no pre-push/remote-sync gate (tick-d1c608)

ACCEPTANCE CRITERIA:
- A gate `n` abort with -p armed performs no push (nothing committed — true no-op, push step never reached)
- An empty/aborted editor save with -p armed performs no push (Phase 3 editor no-op short-circuits before the push step)
- No pre-push or remote-sync gate runs at any point (no behind/diverged or push-ability precheck)
- No remote-sync precheck blocks the push attempt — the push is attempted directly and failure is reported (5-4), not pre-gated
- mint commit -Apy runs unattended end-to-end (auto-accept -> stage -> commit -> push) with no interactive prompt
- The Preflight & Safety drops (clean-working-tree, on-release-branch, remote-in-sync, no pre-push gate) remain dropped — no such gate is added

STATUS: Complete

SPEC CONTEXT:
This is an invariant-confirmation + guard-placement task, not new push machinery (push step is 5-2/5-3; failure warn is 5-4). Spec sources: Auto-push Behaviour (push is best-effort, attempted directly, failure reported not pre-gated), Preflight & Safety -> Gates commit deliberately DROPS (clean-working-tree, on-release-branch, remote-in-sync behind/diverged, no pre-push gate even with -p — "mint doesn't gate the commit on push-ability"), $EDITOR Fallback Path Semantics ("Empty/aborted editor = true no-op. No staging applied, no commit, no push (even with -p)"), and Interactive Review Gate (`n` abort = true no-op, nothing mutated). Two no-commit-so-no-push cases must short-circuit the entire accept-and-push tail; mint commit -Apy must run unattended end-to-end.

IMPLEMENTATION:
- Status: Implemented (confirmed invariant, correctly guarded)
- Location:
  - internal/commit/run.go:844-861 (pushAfterCommit) — single shared push step; `if !deps.Push { return nil }` gate; plain `git push` via Mutator; warn-don't-unwind on failure. Doc comment 823-829 explicitly states NO PRE-PUSH / REMOTE-SYNC GATE (no git fetch, no @{upstream} rev-list/--count, no precheck) — and the code contains exactly one Mutate("git","push") with no preceding probe.
  - internal/commit/run.go:353-360 (gate `n` decline) — returns errGateAborted BEFORE commitAccept, so push is unreachable on abort.
  - internal/commit/run.go:428-438 (runEditorFallback empty/aborted save) — returns errEditorNoOp BEFORE commitAccept, so push is unreachable on an empty/aborted editor save.
  - internal/commit/run.go:786-803 (commitAccept) — strict stage -> commit -> push ordering; pushAfterCommit reached only after createCommit returns nil. Sequencing makes "push only after a successful commit" free for both accept paths.
  - cmd/mint/commit_flags.go:67-78 + cmd/mint/main.go:347 — Push is flag-only (`-p`/`--push`), threaded straight to Deps.Push; no config push key is read or defaulted (confirmed: only deps.Push at run.go:845 consumes it).
- Notes: Verified by source grep that no fetch/rev-list/ls-remote/@{upstream}/--count/behind/diverged/set-upstream/pre-push token exists anywhere in non-comment commit source or engine/cmd touchpoints. The invariant holds structurally (no probe code exists) and by sequencing (every no-commit path returns before commitAccept). No drift from spec.

TESTS:
- Status: Adequate
- Coverage (all acceptance criteria covered, both accept paths):
  - run_push_test.go: TestRun_PushArmedNoCommit_GateNoAbort_NoPush (gate `n` + -p = no commit, no push), TestRun_PushArmedNoCommit_GenerateFailure_NoPush (generate-fail + -p = no push), plus push-happy-path, plain-push-argv, ordering, unarmed-no-push, -y-accept-push, and git_safe lock-retry tests.
  - run_push_suppress_test.go: TestRun_NoAI_PushArmed_EmptyEditorSave_NoPushNoCommit (empty + whitespace-only saves, table-driven), TestRun_NoAI_PushArmed_AbortedEditor_NoPushNoCommit, TestRun_NoAI_PushArmed_AddAll_EmptyEditorSave_NoPushNoCommit (the -Ap bundle, deferred staging short-circuited), TestRun_PushArmed_NoRemoteSyncProbeBeforePush (exact git verb set [diff,diff,commit,push] — proves no probe), TestRun_PushArmed_PushAttemptedDirectlyAfterCommit (pushIdx == commitIdx+1, nothing intervenes), TestRun_DashApy_RunsUnattendedEndToEnd (-Apy add<commit<push, no AskLine, no remote-sync probe).
  - run_editor_push_test.go: editor save-as-accept commits-then-pushes, -Ap --no-ai end-to-end, single shared push step, plain push argv, AI-failure-fallback push, unarmed-no-push.
  - assertNoRemoteSyncProbe helper (run_push_suppress_test.go:43) asserts absence of fetch/rev-list/ls-remote/--count/@{upstream} probes across recorded git calls — the right way to test "a gate that was dropped stays absent."
- Notes: The "no probe" assertions test the exact recorded git invocation set, which is the correct and only meaningful way to verify a deliberately-absent gate (you cannot assert on code that does not exist except via observable behaviour). Suppression tests check all three of no-add/no-commit/no-push, which is appropriate (each is an independent mutation the no-op must skip), not redundant. Not over-tested: each test pins a distinct invariant (suppression on each no-commit path x accept path, no-probe, direct-after-commit, unattended e2e). Tests would fail if a probe were inserted (verb-set assertion) or if push leaked past a no-commit return (push-count assertion).

CODE QUALITY:
- Project conventions: Followed. git_safe Mutator seam for all mutations including push (Go design-patterns / safety skills); table-driven subtests with t.Parallel(); helper extraction (gitArgVerbs, assertNoRemoteSyncProbe) per golang-testing.
- SOLID principles: Good. Single shared pushAfterCommit / commitAccept means the suppression invariant lives in one place and cannot drift between the gate-accept and editor-save-as-accept paths.
- Complexity: Low. Suppression is achieved by early-return ordering, not branching inside the push step; pushAfterCommit is a flat guard + single Mutate + warn.
- Modern idioms: Yes. errors.Is sentinel routing, []byte stdin re-pipe for lock-retry.
- Readability: Good. Doc comments are unusually explicit about the dropped-gate invariant (run.go:823-829) and the "sequencing makes the guarantee free" rationale (run.go:818-821).
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
