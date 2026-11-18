TASK: mint-release-tool-6-3 — Fail-loud validation: bad types

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented, aligned with spec + ACs. internal/config/config.go: strict decode + *toml.DecodeError interception (277-287), typeErrorMessages map + translateTypeError (330-352), HookValue shape validation validateHooks/isHookShape (433-470), validateOnNotesFailure (476-489). Type errors mapped by matching the library's struct-field path text (deliberate workaround for go-toml/v2 reporting type mismatches with nil Key()). Fallback to decErr.String() prevents silent swallowing. Hook shape validation handles nil/string/[]any (per-element string check) and rejects everything else incl. tables (map[string]any) and arrays with non-strings. on_notes_failure closed enum abort|fallback.

TESTS:
- Status: Adequate. config_test.go: max_diff_lines string, publish string, changelog string, diff_exclude scalar, hook string, hook array, hook integer, on_notes_failure invalid (lists valid values), on_notes_failure valid (abort+fallback). Extra: hook table, hook array-of-non-strings. Error-message assertions check both key name + expected-type wording.

CODE QUALITY:
- Followed conventions (table-driven tests, t.Parallel, fail-loud wrapped errors w/ "invalid .mint.toml:" prefix, doc comments). SOLID/DRY good — single decode+validate pass. Low complexity, errors.As branching, type switch in isHookShape.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/config/config.go:344-352 — translateTypeError identifies the offending field via strings.Contains(text, field+" ") against go-toml/v2's free-text DecodeError.Error(). Couples to the library's internal formatting; a v2 upgrade retitling the field path would silently route every type error to the decErr.String() fallback (losing the clean key-named message). The four bad-type tests catch it only via exact-wording assertion. Add a test that fails loudly if decErr doesn't match any typeErrorMessages entry for the four known fields, converting a future silent-degrade into a hard test failure.
- [idea] internal/config/config.go:330-335 — typeErrorMessages is hand-maintained and must stay in lockstep with the schema's constrained-type fields; a future int/bool/array key needs a new entry or the type error falls back to the opaque message. Decide whether to derive these from the schema (reflection over struct tags) or accept the manual map.
