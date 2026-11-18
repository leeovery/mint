TASK: ai-model-selection-2-6 — Migrate the old `claude -p` default-argv test pins to `claude -p --model sonnet`

ACCEPTANCE CRITERIA:
- Every default-case exact-argv assertion of `claude -p` (no `--model`) is migrated to `claude -p --model sonnet`.
- FakeRunner Seed(claude, …) calls are unchanged (seeds match by binary name only).
- Explicit-command tests (e.g. `mybot gen --json`, the commit retry test's `ai.Config{AICommand: "claude -p"}`) are NOT spuriously changed.
- The transport's ex-default-command test (`TestTransport_Generate_DefaultCommandIsClaudeDashP`) is deleted or repointed to assert the passed command verbatim.
- All `ai.Config{...}` constructions in tests compile against Task 2-2's `*time.Duration` Timeout field.
- `go test -race ./...` green; build/gofmt/vet/lint pass.

STATUS: Complete

SPEC CONTEXT:
Per the spec's "Pinned default model" and "Migration & mechanical carry-overs" sections, the shipped default moves from bare `claude -p` to `claude -p --model sonnet`, and the transport's `defaultAICommand` is deleted (Task 2-1) so config's floor is the canonical default. The only real migration cost is internal: mint's own test pins that assert the OLD `claude -p` default. The migration is a "known, bounded set" of edits because project test idioms assert exact argv. The default-proof canonically moves to the config layer; the transport's job becomes "run the passed command verbatim."

IMPLEMENTATION:
- Status: Implemented (complete, bounded)
- Location:
  - internal/engine/release_configconsolidation_test.go:57,103,133 (negative assertions migrated), :158 (positive default assertion migrated) — the file housing TestRelease_AICommand_DefaultDrivesTransport / negative ConfigValue/override/shared assertions.
  - internal/engine/release_priortag_test.go:133,520 (zero-config prior-tag normal-AI default-stdin assertions migrated).
  - internal/engine/release_dryrun_test.go:189 (dry-run default invokedWith migrated).
  - internal/engine/regenerate_fresh_test.go:455,489,517 (floor/[release]-route default assertions migrated — the easy-miss regenerate wiring site, now asserting the sonnet floor).
  - internal/commit/run_aitransport_test.go:125,158,180 (commit default negative + positive assertions migrated).
  - internal/ai/transport_test.go:92 — ex-default test repointed to TestTransport_Generate_RunsPassedAICommandVerbatim (option b): constructs an explicit command and asserts the transport runs the PASSED command verbatim; no transport-supplied default asserted. `defaultAICommand` and any `claude -p` literal are gone from internal/ai/transport.go (grep clean).
- Notes:
  - Bounded-completeness verified by grep: the only surviving bare two-token `"claude", "-p"` argv assertion is internal/runner/fake_runner_test.go:109, which is a runner-package unit test of FakeRunner.RunWith's stdin recording — an arbitrary command/argv, not a shipped-default pin. Correctly out of scope.
  - Remaining `claude -p` literals are all intentional explicit fixtures or comments, NOT stale default pins:
    - internal/commit/generate_test.go:480 — explicit `ai.Config{AICommand: "claude -p"}` driving the L2 retry test; correctly left, and the `*time.Duration` Timeout was threaded (`&timeout` from config.DefaultTimeout).
    - internal/ai/transport_test.go:30/38 (helper newTransport explicit command), :415 (explicit-0 no-deadline cancel test), :477 (nil-timeout panic test) — all explicit-config, all using `ptrTo(...)`/`nil` for the new pointer field.
    - internal/config/config_test.go:805 (valid-keys-load-cleanly strict test, explicit operator value), :1800/:1806 (strict-decode bogus-key tests, explicit value).
  - All test-side `ai.Config{Timeout: …}` constructions use `*time.Duration` (ptrTo / &timeout / nil) — compiles against Task 2-2's field type.

TESTS:
- Status: Adequate
- Coverage: This IS the test-pin migration task; the migrated assertions are the deliverable. The suite now proves the three spec-required behaviours: (1) zero-config release invokes `claude -p --model sonnet` (release_configconsolidation_test.go:158, priortag :133/:520); (2) a configured/override ai_command does NOT invoke the new default (negative assertions :57/:103/:133); (3) the transport runs the passed command verbatim (transport_test.go:92). The regenerate floor route (regenerate_fresh_test.go) and commit default (run_aitransport_test.go) are covered too.
- Notes:
  - Not under-tested: every default-case site enumerated in the task description is migrated, and the canonical-default proof at the config layer (config_test.go:841 AbsentAICommand→sonnet, :858 DefaultAICommand exported value) backstops the moved responsibility.
  - Not over-tested: no redundant default-argv duplication introduced; seeds left binary-keyed (not over-specified). Negative assertions remain meaningful (assert the NEW default argv was absent, not a now-nonexistent one).

CODE QUALITY:
- Project conventions: Followed. Exact-argv assertions preserved per CLAUDE.md test idioms ("assert exact argv … drift is a contract break"); FakeRunner seeds left binary-keyed per the documented seed semantics; explicit-command tests untouched per "assert a passed value, not the default."
- SOLID principles: N/A (test-pin migration; no production code changed in this task).
- Complexity: Low — mechanical argv-token additions.
- Modern idioms: Yes — `*time.Duration` threaded via ptrTo/&value consistently.
- Readability: Good — WHY-comments updated alongside assertions (e.g. release_configconsolidation_test.go:53/:139, transport_test.go:92 doc-comment) so comment and code agree, per CLAUDE.md.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/config/config_test.go:875 — comment reads `must override the "claude -p" default`; the shipped default is now `claude -p --model sonnet`. Update the comment to reference the current default value so it does not encode the pre-migration default (test body uses an unrelated explicit value, so behaviour is unaffected — comment-only).
