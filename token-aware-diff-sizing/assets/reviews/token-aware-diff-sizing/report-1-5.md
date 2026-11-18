TASK: Commit's AI failure carries the AI's verbatim output (token-aware-diff-sizing-1-5, tick-106ed5)

ACCEPTANCE CRITERIA:
- Given an attended `mint commit` whose AI tool exits non-zero on both attempts with a message, such as a claude rejecting `--safe-mode` as an unknown option, the run shows `could not generate a message, opening editor` with the tool's captured output verbatim beneath it, then opens the editor as built (§17.1, §2.5)
- Given the same failure on an unattended run (`-y`, or a non-TTY stdin), the run fails with `no AI message and no interactive editor available`, with the tool's captured output verbatim beneath it (§17.1)
- Given a tool that writes its message to stderr behind an empty or whitespace-only stdout, that message shows beneath the line on both runs, as it does beneath release's ✗ line (§17.1)
- Given `r` at the review gate whose regeneration fails the same way, the warning shows the regeneration call's captured output verbatim beneath it before the editor opens (§17.1)
- Given `--no-ai`, a diff over the size limit, or an all-excluded diff, the editor-fallback lines are unchanged and carry no AI output (§17.1): an over-limit diff still shows `diff too large to summarise — opening editor`, an all-excluded diff `all changes are excluded from the AI context — opening editor`; an unattended run fails with `no AI message and no interactive editor available` and nothing beneath it

STATUS: complete

SPEC CONTEXT: §17.1 requires commit's AI-failure lines to carry the AI's captured output verbatim: beneath the attended warn before the editor opens, and beneath the unattended fail-loud line (the run with nobody present needs the cause most). The other editor-fallback triggers (`--no-ai`, oversized prompt, all-excluded diff) keep their lines unchanged with no AI output. §2.5 names the motivating case: a claude too old for the lean flags fails every call, and only claude's own message says why. §17.2: commit gets no classify call. The transport already carries both streams in `ai.GenerationError` (only on bad-content failures; timeout and missing binary carry none), and release composes them via `GenerationError.Output()`.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/run.go:388-394 — first-generation AI-failure branch extracts `aiFailureOutput(err)` once; attended runs carry it on the `ai` warn's Output, and it goes to `runEditorFallback` for the unattended fail-loud
  - internal/commit/run.go:474-476 — `runEditorFallback`'s no-message-source guard now renders via `surfaceOutput(p, "editor", errNoMessageSource, aiOutput)`
  - internal/commit/run.go:675-678 — failed `r` regeneration warn carries `aiFailureOutput(gerr)`
  - internal/commit/run.go:799-807 — `aiFailureOutput` uses `errors.As` to reach `*ai.GenerationError` and returns `Output()`, the same composition release's `notesFailureOutput` uses (internal/engine/release.go:2009-2015), so commit and release render identical text for the same failure
  - internal/commit/run.go:335, 361, 376, 413 — `--no-ai`, oversized, all-excluded and regen-fallback callers pass "" so no AI output can reach those lines
  - internal/commit/surface.go:19-29 — `surfaceOutput` doc generalised from git-stderr to captured output, which is accurate
- Notes: The `%w` wrapping in internal/commit/generate.go:156 keeps the carrier reachable by `errors.As`. The presenter already renders both `Warning.Output` and `StageFailure.Output` verbatim beneath the line (internal/presenter/pretty.go:583-585, 695-698; plain equivalents), so no presenter change was needed. On an attended run where the editor then cannot launch (run.go:484-486), the fail-loud carries no output. That is correct because the warn above it already showed the output. Timeout and missing-binary failures yield "" by construction. No classify call is added (§17.2). The new comments in the changed hunks are accurate against the code.

TESTS:
- Status: Adequate
- Coverage: internal/commit/run_aifail_output_test.go runs the real `ai.Transport` over the FakeRunner, so the failure is scripted as `claude` itself failing on both attempts:
  - Attended (lines 58-101): an stdout message (the `--safe-mode` unknown-option case), stderr behind whitespace-only stdout, stderr behind empty stdout, and both streams. It asserts the exact `Warning{ai, could not generate…, Output}`, one editor launch and the saved body committed.
  - Unattended (lines 103-142): `-y` and non-TTY stdin, each with an stdout message and with stderr behind whitespace stdout. It asserts the exact `StageFailure{editor, failLoudMessage, Output}`, no warnings and no mutation.
  - Regeneration (lines 144-175): the initial generate succeeds, `r` with an empty line, then both regen attempts fail. It asserts the warn carries the retry attempt's output, then one editor launch and the commit.
  - Non-AI triggers (lines 177-254): `--no-ai`, oversized and all-excluded, attended and unattended, each with a failing claude seeded that must never be reached. It asserts the exact warnings with empty Output and the exact fail-loud with empty Output.
  - No classify call (lines 256-278): exactly the commit prompt's two claude attempts.
  Every assertion compares whole structs, so dropping or leaking Output in any path fails a test.
- Notes: Not over-tested. The table rows each exercise a distinct stream shape or attendance mode named by the criteria.

CODE QUALITY:
- Project conventions: Followed. All output goes through the presenter, subprocess output travels verbatim in `Warning.Output`/`StageFailure.Output`, sentinels are matched with `errors.Is`/`errors.As`, and tests are external, parallel and table-driven, asserting exact presenter payloads.
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
