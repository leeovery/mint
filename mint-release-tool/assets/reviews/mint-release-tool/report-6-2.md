TASK: mint-release-tool-6-2 — Fail-loud validation: unknown keys

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/config/config.go: strict decode L275-276 (dec.DisallowUnknownFields()), error routing L277-287, translateStrict L310-321, unknownKeyMessage L358-373. Correct for go-toml/v2 v2.3.1. Strict decoding surfaces unknown keys at all three levels into *toml.StrictMissingError; translateStrict reports the first offender via its full dotted key path, with a targeted [hooks]→[release.hooks] variant. Generic message names leaf + owning table. Scope held: bad-types/enum (6-3) and provider carve-out (6-4) elsewhere.

TESTS:
- Status: Adequate. UnknownTopLevelKey, UnknownReleaseKey (asserts key + [release]), UnknownReleaseHooksKey (key + [release.hooks]), TopLevelHooksTable_RejectedWithNestGuidance, TypodKey_SurfacedClearly, FullyValidFile_LoadsWithoutError. Substring assertions behaviour-focused.

CODE QUALITY:
- Followed conventions (errors.As type-switching on decoder tree, thorough doc comments, lowercase non-punctuated error strings). SOLID good — small single-purpose helpers. Low complexity, %q quoting.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/config/config.go:362 — unknownKeyMessage keys the nest-guidance variant on key[0]=="hooks" for ANY top-level offender starting with `hooks`, incl. a scalar `hooks = 1`. A scalar top-level hooks would emit "[hooks] is not valid… nest under [release.hooks]", arguably helpful but technically describes it as a table. Decide whether to tighten to only the table case.
- [quickfix] internal/config/config_test.go:700 — TestLoad_TopLevelHooksTable_RejectedWithNestGuidance asserts only that the error contains [release.hooks]. To lock in the targeted variant specifically vs the generic unknown-key path, add an assertion that the message does NOT contain `unknown` / does contain `not valid at the top level`.
