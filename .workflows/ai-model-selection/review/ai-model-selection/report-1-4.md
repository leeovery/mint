TASK: ai-model-selection-1-4 — Add the layered AICommandFor(verb) accessor with multi-layer trim-and-skip

ACCEPTANCE CRITERIA:
- cfg.AICommandFor(verb) returns [verb].ai_command when that override is present and non-blank.
- A blank/whitespace [verb].ai_command falls through to the shared top-level ai_command.
- A blank/whitespace shared ai_command falls through to the shipped default floor.
- A top-level ai_command = (empty, no per-verb override) resolves to DefaultAICommand.
- The accessor NEVER returns an empty string for either verb under any input.
- The returned command preserves the operator's raw string (trim used only for the empty-check).
- Blank/whitespace detection exists in exactly one place; resolveAICommand's old single-layer behaviour is subsumed (helper deleted or repointed).
- go build / gofmt / go vet / go test -race / golangci-lint all pass.

STATUS: Complete

SPEC CONTEXT:
Spec "Resolution value semantics (ai_command)" mandates: blank/whitespace/invalid/missing drops through at every layer; the shipped default is the floor so ai_command is never empty (even top-level ai_command = '' falls through); blank/whitespace detection lives in the config accessor and applies at EVERY layer (multi-layer trim-and-skip), replacing the transport's old single blank-re-default which only ever saw one already-resolved value; the existing resolveAICommand helper (today nil-vs-present only) is folded into / replaced by the accessor so blank-skipping happens in exactly one place. Resolution order is [verb].ai_command -> top-level shared -> shipped default. Per-key independence: the accessor must read only ai_command, never timeout. The verb enum is closed/two-value so the per-verb candidate is selected exhaustively.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/config/config.go:538-561 (AICommandFor); :626-637 (aiCommandOrDefault replacing resolveAICommand); :521-537 (doc comment); :119-127 (Config.AICommand doc updated, stale "transport re-defaults" wording removed).
- Notes:
  - Candidate chain built correctly: override (if non-nil) prepended ahead of [c.AICommand, DefaultAICommand]; single trim-and-skip loop covers all layers; first candidate whose strings.TrimSpace is non-empty is returned RAW (untrimmed) — preserving operator spacing. Trailing `return DefaultAICommand` makes the method total without relying on loop structure.
  - Per-verb selection via the closed Verb enum (VerbCommit reads Commit.AICommand, every other value reads Release.AICommand). Exhaustive by construction; no unknown-verb branch.
  - Per-key independence holds by construction: only ai_command candidates are read; timeout is never consulted.
  - resolveAICommand is fully gone (grep confirms zero remaining references anywhere in the tree). Its absent->default job is now aiCommandOrDefault (absent nil -> DefaultAICommand; explicit value, blank or not, carried verbatim), so an explicit top-level `ai_command = ''` reaches the accessor AS empty and the accessor's trim-and-skip is what falls it to the floor — exactly as the spec requires. Blank/whitespace detection lives in exactly one place (the accessor).
  - The `override != nil` prepend allocates a fresh slice via append([]string{*override}, candidates...); cheap, two-to-three element slice, no concern.
  - No drift from plan/spec.

TESTS:
- Status: Adequate
- Coverage (internal/config/config_test.go):
  - PresentNonBlankReleaseOverride_Wins (:1479) and PresentNonBlankCommitOverride_Wins (:1498) — override-wins for both verbs.
  - BlankPerVerbOverride_FallsToShared (:1516) — table covers empty AND whitespace ("   ", "\t ") overrides for both verbs, proving the trim-and-skip falls to shared.
  - WhitespaceSharedCommand_FallsToFloor (:1567) — whitespace-only shared falls both verbs to the floor.
  - TopLevelEmptyCommand_ResolvesToShippedDefault (:1589) — explicit ai_command = "" reaches the accessor as empty and resolves to the floor (pins the Load-must-not-re-default contract).
  - NeverReturnsEmpty (:1611) — blank-at-every-layer inputs, both verbs, asserts non-empty.
  - NoConfigFile_ResolvesBothVerbsToPinnedDefault (:1647) — zero-config floor for both verbs.
  - PreservesRawCommandString (:1666) — internal multi-space run survives verbatim (no collapse).
  - ReadsOnlyAICommandCandidates (:1687) — leading/trailing-padded shared command returned raw, proving trim is empty-check-only.
- Each test maps to a distinct acceptance criterion; whitespace variants (spaces/tabs) exercise the trim path meaningfully rather than redundantly. Tests are behaviour-level (load real TOML, assert resolved value) per project idioms, not implementation-coupled. Would fail if the feature broke (e.g. if trim were applied to the returned value, PreservesRawCommandString/ReadsOnlyAICommandCandidates fail; if blank-skip regressed, the fall-to-shared/floor tests fail).
- Not over-tested: the per-key independence tests at :2584-2765 reference AICommandFor but belong to Task 1-8 (cross-key independence) — they are not redundant 1-4 coverage.

CODE QUALITY:
- Project conventions: Followed. Single source of truth for blank-detection (CLAUDE.md "blank-skipping lives in exactly one place"); comments kept true-to-as-built (stale "transport re-defaults an explicit empty" wording removed from Config.AICommand; aiCommandOrDefault comment states absent->default only). Strict-decode/resolveX idiom mirrored (aiCommandOrDefault parallels resolveMaxDiffLines). config never imports ai — decoupling preserved.
- SOLID principles: Good. AICommandFor has a single responsibility (resolve the command); aiCommandOrDefault a separate narrow one (absent->default). Closed enum makes the domain exhaustive (no Liskov/interface concerns here).
- Complexity: Low. One linear loop over <=3 candidates; one branch for verb selection.
- Modern idioms: Yes. strings.TrimSpace for blank-check; typed enum parameter; total method with explicit fallback return.
- Readability: Good. WHY-comments explain the floor-makes-it-total reasoning, the raw-return rationale, and the per-key independence guarantee.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
