TASK: One composition rule for a failed AI call's captured output (token-aware-diff-sizing-1-6)

ACCEPTANCE CRITERIA:
- A forward release whose notes call fails on both attempts with `Prompt is too long\n` on stdout shows `Prompt is too long` beneath its ✗ line, and a regenerate failing the same way shows the identical text, as before the change
- A commit whose AI call fails on both attempts with `error: unknown option '--safe-mode'\n` on stdout carries `error: unknown option '--safe-mode'` beneath the attended `could not generate a message, opening editor` warning, beneath the unattended `no AI message and no interactive editor available` line, and beneath the warning when an `r` regeneration fails the same way, as before the change
- With both streams non-blank, the text is stdout then stderr, joined by one newline, interior verbatim, trailing whitespace trimmed, on both verbs (`Prompt is too long\n` + `warning: retrying\n` reads `Prompt is too long\n\nwarning: retrying`)
- A whitespace-only stream counts as empty on both verbs; two whitespace-only streams render no output block
- A cause carrying no captured streams (timeout, missing binary, oversized diff, plain error) renders no output on either verb
- `ai.GenerationError` still holds `Stdout`/`Stderr` raw, and neither helper reads the streams: `rg -n 'genErr\.(Stdout|Stderr)' --type go -g '!*_test.go'` returns nothing
- The existing tests pass with no test file edited, and all five gates pass

STATUS: complete

SPEC CONTEXT: The spec requires the AI's captured output to render verbatim beneath mint's line on every AI-failure surface — release's ✗ line (§12 / §9.6), the fallback warning, regenerate --all skip warnings, and commit's attended warning and unattended fail-loud line (§ on commit, lines ~844-845). One shared composition rule keeps the same tool failure reading alike on both verbs.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/ai/transport.go:89-105 — `func (e *GenerationError) Output() string`, carrying the rule's doc (moved from release.go); `Stdout`/`Stderr` stay raw fields (transport.go:58-70)
  - internal/ai/transport.go:54-57 — `GenerationError` doc now says the fields are DISTINCT, RAW and that Output composes them (no "downstream concern" claim)
  - internal/ai/transport.go:239-244 and :247-256 — both `Generate` construction-site comments updated; construction expressions unchanged
  - internal/engine/release.go:2009-2015 — `notesFailureOutput` keeps name/signature, reduced to `errors.As` + `genErr.Output()`
  - internal/commit/run.go:799-807 — `aiFailureOutput` reduced to `errors.As` + `genErr.Output()`; doc drops the cross-reference to `notesFailureOutput`
- Notes:
  - The task commit (0df42d4) touches exactly internal/ai/transport.go, internal/engine/release.go, internal/commit/run.go plus one new test file; no existing test file edited.
  - `rg -n 'genErr\.(Stdout|Stderr)' --type go -g '!*_test.go'` returns nothing today. The only non-test reads of the raw streams are `composeClassifyPrompt` (internal/notes/classify.go:95,97), a later task's deliberate per-stream fencing, not a composition copy.
  - The `unicode` import is gone from both release.go and run.go; the only `unicode` import among the three files is now internal/ai/transport.go:19, used by Output. `strings` remains used in both helpers' files.
  - The moved composition is semantically identical to both former copies (release used two `if`s, commit a loop; the method takes the loop form). Behaviour is unchanged on both verbs.
  - Later work routes the method through more consumers (internal/notes/resolve.go:193, classify.go:55, exclusioncheck.go:164, and engine's `failureOutput` at release.go:2001), all via `Output()`, so the rule is single-sourced as intended.
  - `notesFailureOutput` has no doc comment now; a later analysis-cycle commit (09ea399) removed it deliberately. That is outside this task and is not a defect.

TESTS:
- Status: Adequate
- Coverage:
  - New internal/ai/generationerror_output_test.go pins the method directly: stdout-only, stderr-only, both streams (`Prompt is too long\n\nwarning: retrying`), whitespace-only stdout behind stderr, two whitespace-only streams → "", no streams → "", interior whitespace verbatim with trailing trim. It also asserts Output leaves the raw fields unmodified.
  - Release side (unchanged tests): internal/engine/notesfailureoutput_internal_test.go (forward and regenerate chains, composition variants, non-carrier → ""), internal/engine/notesfailurewiring_internal_test.go (forward ✗ line and regenerate surface both carry `Prompt is too long`; non-carrier causes → empty Output).
  - Commit side (unchanged tests): internal/commit/run_aifail_output_test.go covers attended warn (`--safe-mode` stdout, stderr behind whitespace stdout, both streams), unattended under `-y` and non-TTY, `r` regeneration failure, and non-AI fallback triggers carrying no output.
  - Raw-field preservation: internal/ai/transport_test.go:620-641 pins a whitespace-only body carried byte-for-byte on `Stdout`.
- Notes: The new ai test overlaps the engine composition tests, but those could not be edited under this task's criterion. Pinning the rule in its owning package is the right home, so the overlap is not over-testing.

CODE QUALITY:
- Project conventions: Followed (external test package, table-driven, t.Parallel throughout; WHY-comments updated in the same commit as the behaviour move, per CLAUDE.md "Comments")
- SOLID principles: Good — composition is owned by the carrier type; consumers keep only extraction
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "The existing tests pass with no test file edited — `internal/engine/notesfailureoutput_internal_test.go`, `internal/engine/notesfailurewiring_internal_test.go`, `internal/commit/run_aifail_output_test.go`, `internal/ai/transport_test.go` among them — and all five gates pass" — the "no test file edited" half is settled by reading (commit 0df42d4 edits no existing test file). The pass half needs a run of `go build ./...`, `gofmt -l .`, `go vet ./...`, `go test -race ./...` and `golangci-lint run` over the tree.
