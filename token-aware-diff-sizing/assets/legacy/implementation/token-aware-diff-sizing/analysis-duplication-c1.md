AGENT: duplication
FINDINGS:
- FINDING: The setup guide's transport paragraph hand-types the shipped timeout and max_prompt_bytes defaults that config owns, beside a config reference rendered from the constants
  SEVERITY: low
  FAILURE: Suppose someone changes a shipped default, as this epic already did when it moved release's timeout from 60 to 180. `mint setup` would then emit a guide that contradicts itself. Its config-reference table is rendered from `config.MetadataRows()`, so it shows the new value. Its "The AI transport" paragraph still says "default 180 for release, 60 for commit" and "default 1,500,000". The agent reading the guide is the one configuring the repo. If it trusts the prose, it may leave a slow `[release].ai_command` under a timeout it thinks is 180s, and every notes call then hits the fatal per-attempt deadline. No test notices. `TestGuide_AITransportParagraphNamesBothShippedTimeouts` and `TestGuide_AITransportParagraphCoversThePerCallCapAndChunking` pin the same stale literals, so they stay green. Only the config-reference test (`TestGuide_ConfigReferenceNamesShippedTimeouts`) fails, and fixing it does not point at the prose.
  FILES: internal/setupguide/setupguide.go:160-163, internal/setupguide/setupguide.go:156, internal/config/config.go:71, internal/config/config.go:96-97, internal/config/metadata.go:118-119, internal/config/metadata.go:138, internal/config/metadata.go:153
  DESCRIPTION: The shipped defaults (`DefaultReleaseTimeout` 180s, `DefaultCommitTimeout` 60s, `DefaultMaxPromptBytes` 1,500,000) live in `internal/config`. `metadata.go` derives its Default cells from them through `seconds()` and `strconv.Itoa`. The setup guide renders those cells in its config reference. The same guide's transport paragraph copies the three values as literals. One line earlier, that paragraph sources the shipped command from `config.DefaultAICommand` precisely so it cannot drift. So one paragraph holds both a derived value and three hand-copied ones, and the hand-copied ones are the drift risk.
  RECOMMENDATION: Build the paragraph's three numbers from `config.DefaultReleaseTimeout`, `config.DefaultCommitTimeout` and `config.DefaultMaxPromptBytes`, as the paragraph already does for `config.DefaultAICommand` and as `metadata.go` does for the reference cells. The grouped "1,500,000" needs a thousands formatter. `notes.groupThousands` is the existing one but it is unexported, so either share it or print the ungrouped digits the config reference already shows. Point the two prose tests at the same constants so they stop pinning copies.
COMMENT_CORRECTIONS:
- internal/engine/release.go:1999-2007 — the function no longer composes anything (`ai.GenerationError.Output` composes the streams), and the remainder is reasoning about why it is not a copy of `hookFailureOutput`, which the code does not need
  OLD: // notesFailureOutput extracts claude's captured output from a *ai.GenerationError
// carrier and composes it for StageFailure.Output. It mirrors hookFailureOutput's
// PATTERN — a typed-carrier extraction — but NOT its field choice: the hook helper reads
// Result.Stderr, whereas claude writes its payload (e.g. "Prompt is too long") to STDOUT,
// so a literal copy of the hook helper would render nothing. errors.As is used precisely
// because it traverses the %w chain, so the carrier still matches when it sits inside
// abortError's forward chain ("notes generation failed (%s): %w") OR regenerate's shorter
// "generating notes: %w" chain. Any non-carrier cause (timeout, command-missing, or a
// plain error) yields "" so the ✗ line stands alone.
  NEW:
SUMMARY: Eight consolidation passes have left little cross-task duplication. Diff-header parsing, exclude composition, cause classification, the `r` rule, the merge inputs and the `--notes-file` command formatting each sit in one place. The one rule still copied is the shipped defaults in the setup guide's prose: no test catches that copy going stale, so `mint setup` would contradict its own config reference. One stale comment still describes `notesFailureOutput` as composing output that `GenerationError.Output` now composes.
