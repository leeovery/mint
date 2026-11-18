TASK: Parse --plain on the Regenerate Route (mint-release-tool-10-6, type: bug)

ACCEPTANCE CRITERIA:
- `mint release regenerate <ver> --plain` is accepted (not a usage error) and selects plain presentation.
- `--plain` composes with the other regenerate flags (--target, --reuse/--fresh, -y, --all).
- Flag-parse test asserting `--plain` is recognised on the regenerate route and propagates into the presenter startup.

STATUS: Complete

SPEC CONTEXT:
Specification "Global / presentation flags" (line 654) declares `--plain` a global presentation flag that applies to EVERY verb: forces token-efficient plain output instead of styled output. Detection model (`--plain` else isatty(stdout)) and rendering are deferred to the CLI Presentation spec. The bug being remediated: the regenerate route hardcoded plainFlag=false and never defined --plain, contradicting that contract and `init.go`'s own doc (init.go:23-25 documents Plain as "the global --plain ... presenter regardless of TTY").

IMPLEMENTATION:
- Status: Implemented
- Location:
  - cmd/mint/regenerate_flags.go:67-70 — `Plain bool` field added to regenerateRequest with a doc comment stating it is the global --plain flag, identical name/default/meaning to the forward route.
  - cmd/mint/regenerate_flags.go:83,91 — `plain` bool declared and registered: `fs.BoolVar(&plain, "plain", false, "force plain (un-styled) output")` — name, default, and usage string match init.go:42 and the forward release parser.
  - cmd/mint/regenerate_flags.go:126 — `Plain: plain` threaded into the returned regenerateRequest.
  - cmd/mint/regenerate_validate.go:52 — validateRegenerateRequest returns `req` unchanged on the success path, so validated.Plain carries the parsed value through unmodified.
  - cmd/mint/main.go:128 — `presenter.NewForStartup(validated.Plain, validated.Yes, ...)` — the hardcoded `false` first argument is gone; the parsed flag is now threaded. Signature confirmed at internal/presenter/wiring.go:66 (`NewForStartup(plainFlag, yes bool, ...)`), so position is correct.
- Notes: The plan text references `cmd/mint/main.go:115` for the NewForStartup call; the call now lives at line 128 (the file evolved). The substance — replacing the hardcoded false first arg with the parsed value — is correctly done. No drift from the acceptance criteria. The --plain token is parsed in the flag-only remainder (splitRegeneratePositional passes flag tokens through), so it composes with the positional <version> in any order and with --all.

TESTS:
- Status: Adequate
- Coverage: cmd/mint/regenerate_flags_test.go — TestParseRegenerateFlags is a table test extended with a `wantPlain` column (line 22) asserted for every row (lines 178-180). Three dedicated rows:
  - "plain is recognised on the regenerate route" (line 127): `{"1.4.0", "--plain"}` → wantPlain true — directly covers AC #1 (accepted, not a usage error; the test Fatals on any parse error at line 158, so a usage error would fail).
  - "plain composes with the single-version regenerate flags" (line 134): `{"1.4.0", "--reuse", "--target", "release", "-y", "--plain"}` → asserts Plain, Source=reuse, Target=release, Yes — covers AC #2 composition with the single-version axes.
  - "plain composes with the all batch flags" (line 144): `{"--all", "--plain"}` → wantPlain true, wantAll true — covers AC #2 composition with --all.
  Every pre-existing row also asserts wantPlain (defaulting to false), confirming --plain defaults off when omitted.
- Notes: Tests verify parse + propagation into the regenerateRequest, which is the propagation point the presenter startup consumes (validateRegenerateRequest passes req through, main.go threads validated.Plain). The "propagates into the presenter startup" wording in the plan's Tests line is satisfied at the request level; there is no separate test asserting NewForStartup receives the value, but that wiring is a single direct field read with no branching, so a request-level assertion is the right granularity — not under-tested. Not over-tested: no redundant rows, no implementation-detail assertions, no superfluous mocking. Balanced.

CODE QUALITY:
- Project conventions: Followed. Matches golang-cli flag-handling conventions — the flag is registered on the same FlagSet as the other regenerate flags, mirroring init.go and the forward release parser exactly (same name "plain", same default false, same usage string). golang-testing: table-driven, t.Parallel() at both levels, plain testing package (no testify needed here, consistent with the file's existing style).
- SOLID principles: Good. parseRegenerateFlags retains a single responsibility (structural parse only); the flag addition does not leak semantic concerns. Plain is carried as a plain value, no new coupling.
- Complexity: Low. One BoolVar registration and one struct field assignment; no new branches.
- Modern idioms: Yes. Idiomatic flag.FlagSet usage consistent with the rest of the package.
- Readability: Good. The Plain field carries a precise doc comment (lines 67-70) explaining it is the global flag and composes with every regenerate flag.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] cmd/mint/regenerate_flags_test.go:5-9 — TestParseRegenerateFlags's doc comment still says "the --all / -y booleans" and "parse skeleton only", predating the --plain addition; append --plain to the enumerated surface so the comment matches the asserted columns.
