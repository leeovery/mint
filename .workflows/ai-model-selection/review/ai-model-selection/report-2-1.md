TASK: ai-model-selection-2-1 — Delete the transport command self-default and correct its comments

ACCEPTANCE CRITERIA:
- The `defaultAICommand` const is deleted from internal/ai/transport.go; the literal "claude -p" appears nowhere in the file.
- NewTransport assigns the Transport's `command` field directly from `cfg.AICommand` with no trim/re-default branch.
- A blank/whitespace AICommand is no longer re-defaulted by the transport (passed through unchanged).
- The Config.AICommand, NewTransport, and parseCommand comments no longer claim the transport defaults an empty command; they state config owns the default and the blank-skip.
- All gates pass (go build / gofmt / go vet / go test -race / golangci-lint).

STATUS: Complete

SPEC CONTEXT:
Per spec "Single source of truth for config defaults" and "Resolution value semantics (ai_command)": config's floor (config.DefaultAICommand) always resolves a valid, non-empty command, and the multi-layer blank trim-and-skip lives once in config.AICommandFor. The transport's old "empty → re-default" path is therefore unreachable dead code and is removed (the duplicate const also re-introduced the exact `claude -p` literal the de-duplication target removes). Spec "Transport doc-comment migration" mandates the WHY-comments encoding the deleted "(default `claude -p` when empty)" / "An empty AICommand resolves to `claude -p`" contracts be corrected in the SAME change (CLAUDE.md comments-true-to-as-built). This task is the command-side half only; the timeout-side self-default deletion is Task 2-2.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/ai/transport.go (task commit 292ccb0). Current HEAD form: Config.AICommand comment lines 45-48; NewTransport lines 83-86 + assignment line 121 (`command: cfg.AICommand`); parseCommand comment lines 242-248.
- Notes: Verified mechanically against acceptance criteria:
  - `defaultAICommand` const: absent (grep returns none).
  - Literal "claude -p": absent from transport.go (grep returns none).
  - NewTransport assigns `command: cfg.AICommand` with no trim/re-default branch (the `strings.TrimSpace(command)=="" → defaultAICommand` block is gone).
  - `strings` import retained and still used (strings.Fields, strings.TrimSpace, strings.NewReader) — go build implicitly green via gates.
  - All three comments rewritten: each now states config's floor guarantees non-empty and the blank-skip lives in config.AICommandFor; parseCommand explicitly notes "NewTransport no longer guards blank" and the empty-fields branch is a defensive no-op.
  - No drift. The timeout-side machinery now in the HEAD file (nil-panic, *time.Duration boundary, conditional deadline) was added by Task 2-2 (commit 548c11d), correctly outside this task's scope. The HEAD comments remain true-to-as-built: the command-side claims match the shipped command-side code, and the timeout-side comments match 2-2's shipped code.

TESTS:
- Status: Adequate
- Coverage: internal/ai/transport_test.go.
  - TestTransport_Generate_RunsPassedAICommandVerbatim (the repointed former DefaultCommandIsClaudeDashP) — proves a passed `mybot gen` drives argv name `mybot` + args ["gen"] and asserts NO `claude` invocation occurred (the re-default is gone). Directly covers "assigns from cfg.AICommand verbatim".
  - TestTransport_Generate_PassesBlankAICommandThroughUnchanged — constructs with AICommand "  ", seeds the empty-name binary, asserts the transport does NOT substitute `claude`. Proves the dead blank-re-default path is removed (the spec edge case).
  - The default-command coverage premise correctly migrated: the old "no AICommand → claude -p" test premise is gone; canonical zero-config default coverage now lives in config's AICommandFor test (per task note). Only this file's directly-broken test was touched; the full argv-pin sweep is Task 2-6.
- Notes: Tests assert exact argv (name + equalArgs) per project idiom, use FakeRunner, t.Parallel, external package — convention-conformant. Not over-tested: the blank-passthrough and verbatim tests each prove a distinct behaviour (re-default removed vs verbatim argv) with no redundant assertions. Would fail if the feature regressed (e.g. a reinstated re-default would invoke `claude` and trip the explicit no-claude guard loop).

CODE QUALITY:
- Project conventions: Followed. No business-logic output bypasses presenter; subprocess stays on the runner seam; comments kept true-to-as-built per CLAUDE.md (the central obligation of this task) — verified the command-side comments no longer carry any deleted-contract claim.
- SOLID principles: Good. Removing the duplicate default tightens single-source-of-truth (config owns defaults); the transport's responsibility narrows to "run the resolved command".
- Complexity: Low. NewTransport lost a branch; parseCommand unchanged.
- Modern idioms: Yes.
- Readability: Good. The corrected WHY-comments accurately explain why the empty-fields branch in parseCommand stays (defensive no-op, unreachable from production).
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
