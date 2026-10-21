TASK: Build the setup guide's transport-paragraph defaults from the config constants (token-aware-diff-sizing-9-2)

ACCEPTANCE CRITERIA:
- `mint setup` prints the transport paragraph's timeout clause as `default 180 for release, 60 for commit` and its cap as `default 1500000`; `1,500,000` no longer appears in the guide (§18.1)
- The paragraph's three numbers are `config.DefaultReleaseTimeout` and `config.DefaultCommitTimeout` in whole seconds and `config.DefaultMaxPromptBytes` in ungrouped digits; the paragraph holds no hand-typed copy of a shipped default
- `TestGuide_AITransportParagraphNamesBothShippedTimeouts` and `TestGuide_AITransportParagraphCoversThePerCallCapAndChunking` build the timeout clause and the cap text they expect from those constants
- Apart from `1,500,000` becoming `1500000`, `mint setup` prints the guide byte-for-byte as it does today

STATUS: complete

SPEC CONTEXT: §18.1 requires the setup guide's AI-transport paragraph to state the release timeout's 180s default beside commit's 60s (§4.4) and cover the per-call size cap (§4). §18.1 also has the config reference render from the config-metadata SoT, whose timeout and max_prompt_bytes Default cells derive from the same config constants (internal/config/metadata.go:118-119, :138, :153). The task closes the drift where the prose hand-typed copies of those defaults.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/setupguide/setupguide.go:165-166 (timeout clause spliced from `wholeSeconds(config.DefaultReleaseTimeout)` / `wholeSeconds(config.DefaultCommitTimeout)`), :168 (cap spliced from `strconv.Itoa(config.DefaultMaxPromptBytes)`), :187-192 (`wholeSeconds` helper, same formula as config's unexported `seconds` at internal/config/metadata.go:159-160), :118-121 (overview doc comment on the splice); constants at internal/config/config.go:70, :95-96; production emitter cmd/mint/setup.go:46 prints `setupguide.Guide()` verbatim.
- Notes: The implementing commit's diff keeps every raw-string line break at its old position (`default ` + … + ` for\nrelease, ` + … and `default ` + … + `\nbytes`), so the emitted guide is byte-identical apart from `1,500,000` -> `1500000`. That settles criterion 4 by reading. `rg '180|\b60\b|1,500,000|1500000'` over setupguide.go now returns nothing, so the paragraph holds no hand-typed shipped default. The other defaults it names come from constants too: the command from `config.DefaultAICommand`, the timeouts and the cap as above. No grouping formatter and no internal/notes import were added. As the task's Do section requires, README.md:127's `1,500,000` and `TestGuide_ConfigReferenceNamesShippedTimeouts` are left alone. Later commits (11-2, 11-3, 9-4 corrections) reworded other parts of the paragraph and the pipeline section without touching the spliced numbers.

TESTS:
- Status: Adequate
- Coverage: `TestGuide_AITransportParagraphCoversThePerCallCapAndChunking` (internal/setupguide/setupguide_test.go:389-405) expects `"default " + strconv.Itoa(config.DefaultMaxPromptBytes) + " bytes"` against the whitespace-flattened paragraph. `TestGuide_AITransportParagraphNamesBothShippedTimeouts` (:407-421) builds the full timeout clause from both constants through a test-side `wholeSeconds` (:435-437) and keeps the negative `default 60;` guard. The new `TestGuide_StatesThePromptCapUngrouped` (:425-431) checks that `1,500,000` is absent from the whole guide. Release's 180 is still pinned literally where it matters, through `TestGuide_ConfigReferenceNamesShippedTimeouts` (:439-459, `"180 on release, 60 on commit"`), so a changed constant still turns a test red.
- Notes: The test-side `wholeSeconds` is an independent copy of the formula, so a change to the production rendering's unit would fail the timeout test. These are focused assertions, so the tests are neither over- nor under-tested.

CODE QUALITY:
- Project conventions: Followed (external test package, t.Parallel, WHY-comments that match the built code)
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None. The overview comment ("Every shipped default the transport paragraph states is spliced from its config constant") matches the code. The `wholeSeconds` comment correctly names config/metadata.go's `seconds` as the form it mirrors.

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
